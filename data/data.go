package data

import (
	"context"
	"fmt"
	"gofish/logs"
	"gofish/utils"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetFishData(pool *pgxpool.Pool, chatNames string, catches string, numMonths int, monthYear string, logInstance string, mode string) {

	config := utils.LoadConfig()

	var wg sync.WaitGroup
	fishChan := make(chan []FishInfo)

	logs.Logs().Info().Msgf("Checking new data")

	fishCatches := returnTheCatchPatterns(catches)

	switch chatNames {
	case "all":

		for chatName, chat := range config.Chat {
			if !chat.CheckFData {
				if chatName != "global" && chatName != "default" {
					logs.Logs().Warn().
						Str("Chat", chatName).
						Msg("Skipping chat because checkfdata is false")
				}
				continue
			}

			wg.Add(1)
			go func(chatName string, chat utils.ChatInfo) {
				defer wg.Done()
				urls := CreateURL(chatName, chat, numMonths, monthYear, logInstance)
				fishData := ProcessFishDataForChat(urls, chatName, fishCatches, chat, pool, mode)
				fishChan <- fishData

			}(chatName, chat)
		}

	default:

		specifiedchatNames := strings.Split(chatNames, ",")
		for _, chatName := range specifiedchatNames {
			chat, ok := config.Chat[chatName]
			if !ok {
				logs.Logs().Error().
					Str("Chat", chatName).
					Msg("Chat not found in config")
				return
			}
			if !chat.CheckFData {
				if chatName != "global" && chatName != "default" {
					logs.Logs().Warn().
						Str("Chat", chatName).
						Msg("Skipping chat because checkfdata is false")
				}
				continue
			}

			wg.Add(1)
			go func(chatName string, chat utils.ChatInfo) {
				defer wg.Done()
				urls := CreateURL(chatName, chat, numMonths, monthYear, logInstance)
				fishData := ProcessFishDataForChat(urls, chatName, fishCatches, chat, pool, mode)
				fishChan <- fishData

			}(chatName, chat)
		}
	}

	go func() {
		wg.Wait()
		close(fishChan)
	}()

	var allFish []FishInfo
	for fishData := range fishChan {
		allFish = append(allFish, fishData...)
	}

	// Sort the final data by date
	sort.SliceStable(allFish, func(i, j int) bool {
		return allFish[i].Date.Before(allFish[j].Date)
	})

	if err := insertFishDataIntoDB(allFish, pool, config, mode); err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error inserting fish data into database")
		return
	}

}

func ProcessFishDataForChat(urls []string, chatName string, fishCatches []FishCatch, Chat utils.ChatInfo, pool *pgxpool.Pool, mode string) []FishInfo {
	var allFish []FishInfo
	var wg sync.WaitGroup
	var mu sync.Mutex

	fishChan := make(chan FishInfo)

	var latestCatchDate, latestBagDate, latestTournamentDate time.Time

	ctx := context.Background()

	if mode == "a" {
		// Set the latest date to "0", so that all data gets parsed and checked
		latestCatchDate = time.Time{}
		latestBagDate = time.Time{}
		latestTournamentDate = time.Time{}
	} else {
		var err error
		latestCatchDate, err = getLatestCatchDateFromDatabase(ctx, pool, chatName, "fish")
		if err != nil {
			logs.Logs().Fatal().Err(err).
				Str("Chat", chatName).
				Msg("Error while retrieving latest catch date for chat")
		}
		latestBagDate, err = getLatestCatchDateFromDatabase(ctx, pool, chatName, "bag")
		if err != nil {
			logs.Logs().Fatal().Err(err).
				Str("Chat", chatName).
				Msg("Error while retrieving latest bag date for chat")
		}
		latestTournamentDate, err = getLatestCatchDateFromDatabase(ctx, pool, chatName, "tournaments")
		if err != nil {
			logs.Logs().Fatal().Err(err).
				Str("Chat", chatName).
				Msg("Error while retrieving latest tournament result date for chat")
		}
	}

	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			fishData, err := GetFishDataFromURL(url, chatName, fishCatches, pool, latestCatchDate, latestBagDate, latestTournamentDate)
			if err != nil {
				logs.Logs().Error().Err(err).
					Msg("Error fetching data")
				return
			}
			mu.Lock()
			defer mu.Unlock()
			allFish = append(allFish, fishData...)
		}(url)
	}

	go func() {
		wg.Wait()
		close(fishChan)
	}()

	for fish := range fishChan {
		mu.Lock()
		allFish = append(allFish, fish)
		mu.Unlock()
	}

	return allFish
}

func insertFishDataIntoDB(allFish []FishInfo, pool *pgxpool.Pool, config utils.Config, mode string) error {

	// Start the transaction to insert the data
	// maybe include adding/renaming players, adding fish, creating the tables in the transaction or ?
	// could batch all the inserts with pgx.batch
	// this would be faster, but i dont know how to see what query failed if there is an error
	tx, err := pool.Begin(context.Background())
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error starting transaction")
		return err
	}
	defer tx.Rollback(context.Background())

	tableName := "fish"
	tableNameBag := "bag"
	fishinfotable := "fishinfo"
	playerdatatable := "playerdata"
	tournamenttable := "tournaments"

	CheckTables := []string{
		fishinfotable,
		tableName,
		tableNameBag,
		playerdatatable,
		tournamenttable,
	}

	for _, table := range CheckTables {
		if err := utils.EnsureTableExists(pool, table); err != nil {
			logs.Logs().Error().Err(err).
				Str("Table", table).
				Msg("Error ensuring table exists")
			return err
		}
	}

	lastChatIDs := make(map[string]int)
	newBagCounts := make(map[string]int)
	newFishCounts := make(map[string]int)
	newResultCounts := make(map[string]int)

	// to store some stuff
	fishNames := make(map[string]string)

	for chatName, chat := range config.Chat {
		if chat.CheckFData {
			newBagCounts[chatName] = 0
			newFishCounts[chatName] = 0
			newResultCounts[chatName] = 0
		}
	}

	// First need to check fish . Date, because not all instances were always in utc

	// Checking older logs when there is daylight savings can add fish again which are already in the db
	// idk, i delete those
	// https://logs.ivr.fi/channel/psp1g/2023/10/29 compare to https://logs.nadeko.net/channel/psp1g/2023/10/29
	// there is also 1 fish for vaia + 2 for psp in october 2024 where this happens on oct27 for logxx
	// and also for bread and psp chats in march 2024 its 12 catches for logxx

	// Time can be inconsistent between channels in same instance:
	// https://logs.ivr.fi/channel/d_egree/user/gofishgame/2024/6 ahead of spanix by 2 hours https://logs.spanix.team/channel/d_egree/user/gofishgame/2024/6?
	// but not here: https://logs.ivr.fi/channel/ajspyman/user/gofishgame/2024/6 ; https://logs.spanix.team/channel/ajspyman/user/gofishgame/2024/6
	// The time in spanix is probably the correct one based off of mitglieds logs:
	// [2024-06-1 22:13:21] #breadworms mitgliederversammlung: $gn elisSleep in logs joinuv
	// [2024-06-01 23:33:58] #d_egree gofishgame: @mitgliederversammlung, You caught a ✨ 🦑 ✨! It weighs 15.02 lbs. (30m cooldown after a catch) logs ivr

	// Some data was added to the db before I knew that the time wasnt in utc:
	// for julialuxel, ajspyman, zomballr, ovrht, d_egree, ryanpotat
	// I updated the times for their fish (update fish set date = date - interval '1/2' hour where chat ... and date ....)
	// but their tournament results i didnt change the times ! bags was rechecked anyways so didnt matter

	// Fishids of the fish added from when i rechecked all the instances for all the chats: 163725 - 165222
	// this was in early 2025

	// When logs ivr started using utc in the logs, seems to be the same for potat and spanix i think ?
	// i found this date by comparing psp chat fish logs from nadeko and ivr; nadeko should be in utc
	datelogsivr, _ := utils.ParseDate("2024-06-06 00:00:00")

	datesusgee, _ := utils.ParseDate("2024-07-01 00:00:00")
	datesusgee2, _ := utils.ParseDate("2024-03-31 00:00:00")

	// logxx had times not in utc before august 2025 (i think)
	datelogxx, _ := utils.ParseDate("2025-08-07 00:00:00")
	datelogxx2, _ := utils.ParseDate("2025-03-30 00:00:00")
	datelogxx3, _ := utils.ParseDate("2024-10-27 00:00:00")

	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error loading time location for berlin")
		return err
	}

	// find all the different players and their playerids and when they fished
	// before inserting the fish
	confirmedPlayers, err := ConfirmWhoIsWho(allFish, pool)
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error going over all the players")
		return err
	}

	for _, fish := range allFish {

		// logs.ivr had the logs in utc+1/utc+2, potat and spanix aswell
		if strings.Contains(fish.Url, "logs.ivr.fi") || strings.Contains(fish.Url, "logs.potat.app") || strings.Contains(fish.Url, "logs.spanix.team") {

			if fish.Date.Before(datelogsivr) {

				// re parse the time of the fish to have the location set to berlin cet/cest
				notUTC, err := utils.ParseDateInLoc(fish.Date.Format("2006-01-2 15:04:05"), loc)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Time", fish.Date.String()).
						Msg("Error parsing time")
					return err
				}

				// convert the time of the fish back to utc, this works with daylight savings
				fish.Date = notUTC.In(time.UTC)
			}
		}

		// logs.susgee is in utc+1/utc+2; but was +2/+4 (?)
		// OR maybe: logs susgee was always in utc+1/+2; and ivr, spanix and potat were in utc ? (???)
		if strings.Contains(fish.Url, "logs.susgee.dev") {

			if fish.Date.After(datesusgee) {
				notUTC, err := utils.ParseDateInLoc(fish.Date.Format("2006-01-2 15:04:05"), loc)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Time", fish.Date.String()).
						Msg("Error parsing time")
					return err
				}

				fish.Date = notUTC.In(time.UTC)
			} else if fish.Date.Before(datesusgee2) {

				fish.Date = fish.Date.Add(time.Hour * -2)
			} else if fish.Date.Before(datesusgee) && fish.Date.After(datesusgee2) {

				fish.Date = fish.Date.Add(time.Hour * -4)
			}
		}

		// for logxx add 2 / 1 hours (idk?)
		if strings.Contains(fish.Url, "logxx") {
			if fish.Date.Before(datelogxx) && fish.Date.After(datelogxx2) {
				fish.Date = fish.Date.Add(time.Hour * 2)
			} else if fish.Date.Before(datelogxx2) && fish.Date.After(datelogxx3) {
				fish.Date = fish.Date.Add(time.Hour * 1)
			} else if fish.Date.Before(datelogxx3) && fish.Date.After(datesusgee2) {
				fish.Date = fish.Date.Add(time.Hour * 2)
			} else if fish.Date.Before(datesusgee2) {
				fish.Date = fish.Date.Add(time.Hour * 1)
			}
		}

		// for playerid: here now only need to retrieve the id from the map
		var playerID int

		for _, maybePlayer := range confirmedPlayers[fish.Player] {

			for _, dates := range maybePlayer.confirmedDates {
				// this could be weird; since here im checking the dates after updating the dates
				// and for the confirmed players im getting the dates before updating
				if fish.Date.Before(dates.HighestDate) && fish.Date.After(dates.LowestDate) {
					playerID = maybePlayer.PlayerID
					break
				}
			}

		}

		if playerID == 0 {
			logs.Logs().Warn().
				Str("Chat", fish.Chat).
				Str("Player", fish.Player).
				Str("Date", fish.Date.Format("2006-01-2 15:04:05")).
				Msg("NO PLAYERID FOR PLAYER")
		}

		switch fish.CatchType {
		// Add the fish into fish table
		default:

			// Only need to check if a fish/bag exists if mode is 'a', because else you only have new fish
			// Not checking the exact second here and in bags, because that can be different, example:
			// [2023-12-31 00:33:46] #psp1g gofishgame: @leoisbaba, You caught a ✨ 🐟 ✨! It weighs 31.86 lbs. (30m cooldown after a catch) logs.ivr.fi
			// [2023-12-30 23:33:45] #psp1g gofishgame: @leoisbaba, You caught a ✨ 🐟 ✨! It weighs 31.86 lbs. (30m cooldown after a catch) logs.nadeko.net
			// Can even be more than one second, 5 is the largest difference i found, examples:
			// [2024-05-25 00:10:27] #psp1g gofishgame: @divra__, You caught a ✨ 🐠 ✨! It weighs 3.25 lbs. (30m cooldown after a catch) logs.ivr.fi
			// [2024-05-24 22:10:25] #psp1g gofishgame: @divra__, You caught a ✨ 🐠 ✨! It weighs 3.25 lbs. (30m cooldown after a catch) logs.nadeko.net
			// [2024-01-22 02:23:25] #psp1g gofishgame: @caprise627, You caught a ✨ 🥫 ✨! It weighs 3.62 lbs. (30m cooldown after a catch) logs.ivr.fi
			// [2024-01-22 01:23:21] #psp1g gofishgame: @caprise627, You caught a ✨ 🥫 ✨! It weighs 3.62 lbs. (30m cooldown after a catch) logs.nadeko.net
			// [2024-01-20 13:48:50] #psp1g gofishgame: @norque69, You caught a ✨ 🦪 ✨! It weighs 15.93 lbs. (30m cooldown after a catch) logs.ivr.fi
			// [2024-01-20 12:48:45] #psp1g gofishgame: @norque69, You caught a ✨ 🦪 ✨! It weighs 15.93 lbs. (30m cooldown after a catch) logs.nadeko.net
			// If someone gets the same fish from releasing in between ten seconds and one of the catches wasnt logged, this would skip that catch though
			// Because fishtype, weight (0 lbs), player and chat would be the same and the date would fall in between that date range

			// for the 2024 winter gifts, only one of the candys or acorns gets added to the db
			// when checking in mode a
			// because the others all have identical date, weight, player, chat, fishtype
			if mode == "a" {

				var count int
				err := tx.QueryRow(context.Background(), `
				SELECT COUNT(*) FROM `+tableName+`
				WHERE date <= $1::timestamp AND date >= $2::timestamp
				AND weight = $3 AND player = $4 AND chat = $5 AND fishtype = $6
				`, fish.Date.Add(time.Second*5), fish.Date.Add(time.Second*-5), fish.Weight, fish.Player, fish.Chat, fish.FishType).Scan(&count)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Table", tableName).
						Msg("Error checking if fish exists")
					return err
				}
				if count > 0 {
					continue // Skip that fish
				}
			}

			if _, ok := lastChatIDs[fish.Chat]; !ok {
				lastChatID, err := getLastChatIDFromDB(pool, fish.Chat, tableName)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Table", tableName).
						Str("Chat", fish.Chat).
						Msg("Error getting last chat ID")
					return err
				}
				lastChatIDs[fish.Chat] = lastChatID
			}

			lastChatIDs[fish.Chat]++
			chatID := lastChatIDs[fish.Chat]

			// get the fishname for the fishtype and store it
			if _, ok := fishNames[fish.FishType]; !ok {
				fishName, err := GetFishName(pool, fishinfotable, fish.FishType)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Type", fish.FishType).
						Msg("Error getting fish name")
					return err
				}

				fishNames[fish.FishType] = fishName
			}

			fishName := fishNames[fish.FishType]

			query := fmt.Sprintf("INSERT INTO %s (chatid, fishtype, fishname, weight, catchtype, player, playerid, date, bot, chat, url) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)", tableName)
			_, err = tx.Exec(context.Background(), query, chatID, fish.FishType, fishName, fish.Weight, fish.CatchType, fish.Player, playerID, fish.Date, fish.Bot, fish.Chat, fish.Url)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Query", query).
					Msg("Error inserting fish data")
				return err
			}

			newFishCounts[fish.Chat]++

		// Add the bag into the table for bags
		case "bag":

			if mode == "a" {

				var count int
				err := tx.QueryRow(context.Background(), `
				SELECT COUNT(*) FROM `+tableNameBag+`
				WHERE date <= $1::timestamp AND date >= $2::timestamp
				AND player = $3 AND chat = $4 AND bag = $5
				`, fish.Date.Add(time.Second*5), fish.Date.Add(time.Second*-5), fish.Player, fish.Chat, fish.Bag).Scan(&count)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Table", tableNameBag).
						Msg("Error checking if bag exists")
					return err
				}
				if count > 0 {
					continue // Skip that bag
				}
			}

			query := fmt.Sprintf("INSERT INTO %s (bag, player, playerid, date, bot, chat, url) VALUES ($1, $2, $3, $4, $5, $6, $7)", tableNameBag)
			_, err = tx.Exec(context.Background(), query, fish.Bag, fish.Player, playerID, fish.Date, fish.Bot, fish.Chat, fish.Url)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Query", query).
					Msg("Error inserting bag data")
				return err
			}
			newBagCounts[fish.Chat]++

		// Insert the tournament result into the tournament table
		case "result":

			// check if that exact result is already in the db in the week before the results date
			// There is a bug where it will show the checkin result of the previous week if noone checked in, have to manually delete those
			// It appeared here:
			// jellyuh: jan 2025 - march 2025 (4+?)
			// psp1g: sept 2023 (3), oct 2023 (1), feb 2024 (4), april 2024 (1), may 2024 (2)
			// omie: end of aug 2024 (2), mid oct 2024 (2)
			// julia: dec 2023 - jan 2024 (7), because i didnt update the times of the older results to be in utc
			// ryanpotat: dec 2024 - feb 2025 (2)
			// mowogan: 2025
			// aaurie: 2025
			// For bread some results (7) get readded in may 2023 because she kept updating the results and format
			var count int
			err := tx.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM `+tournamenttable+`
			WHERE date >= $1::timestamp AND date <= $2::timestamp
			AND player = $3 AND fishcaught = $4 AND placement1 = $5 AND totalweight = $6 AND placement2 = $7 AND biggestfish = $8 AND placement3 = $9 AND chat = $10
		`, fish.Date.Add(time.Hour*-168), fish.Date, fish.Player, fish.Count, fish.FishPlacement, fish.TotalWeight, fish.WeightPlacement,
				fish.Weight, fish.BiggestFishPlacement, fish.Chat).Scan(&count)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Table", tournamenttable).
					Str("Chat", fish.Chat).
					Msg("Error counting existing results")
				return err
			}
			if count > 0 {
				continue
			}

			query := fmt.Sprintf("INSERT INTO %s ( player, playerid, fishcaught, placement1, totalweight, placement2, biggestfish, placement3, date, bot, chat, url) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)", tournamenttable)
			_, err = tx.Exec(context.Background(), query, fish.Player, playerID, fish.Count, fish.FishPlacement, fish.TotalWeight,
				fish.WeightPlacement, fish.Weight, fish.BiggestFishPlacement, fish.Date, fish.Bot, fish.Chat, fish.Url)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Table", tournamenttable).
					Str("Chat", fish.Chat).
					Str("Query", query).
					Msg("Error inserting tournament data")
				return err
			}

			newResultCounts[fish.Chat]++
		}
	}

	if err := tx.Commit(context.Background()); err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error committing transaction")
		return err
	}

	// Log the fish / bags / results
	newCounts := []map[string]int{newFishCounts, newBagCounts, newResultCounts}
	var things string
	somenumber := 0

	for _, m := range newCounts {
		somenumber++

		switch somenumber {
		case 1:
			things = "fish"
		case 2:
			things = "bags"
		case 3:
			things = "results"
		}

		var noNewCounts []string

		for chat, count := range m {
			if count > 0 {
				logs.Logs().Info().
					Int("Count", count).
					Str("Chat", chat).
					Msgf("New %s added into the database for chat", things)
			} else {
				noNewCounts = append(noNewCounts, chat)
			}
		}

		sort.SliceStable(noNewCounts, func(i, j int) bool {
			return noNewCounts[i] < noNewCounts[j]
		})

		logs.Logs().Info().
			Interface("Chats", noNewCounts).
			Msgf("No new %s found for chats", things)

	}

	return nil
}

func getLastChatIDFromDB(pool *pgxpool.Pool, chatName string, tablename string) (int, error) {
	var lastChatID int

	query := fmt.Sprintf("SELECT COALESCE(MAX(chatid), 0) FROM %s WHERE chat = $1", tablename)
	row := pool.QueryRow(context.Background(), query, chatName)
	err := row.Scan(&lastChatID)
	if err != nil {
		// 42P01 is when the table doesnt exist yet, if you check for the first time
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42P01" {
			return 0, nil
		}
		return 0, err
	}
	return lastChatID, nil
}
