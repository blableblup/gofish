package data

import (
	"context"
	"fmt"
	"gofish/logs"
	"gofish/playerdata"
	"gofish/utils"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetData(pool *pgxpool.Pool, chatNames string, data string, numMonths int, monthYear string, logInstance int, mode string) {

	config := utils.LoadConfig()

	switch data {
	case "f":
		GetFishData(config, pool, chatNames, data, numMonths, monthYear, logInstance, mode)
	case "t":
		GetFishData(config, pool, chatNames, data, numMonths, monthYear, logInstance, mode)
	case "all":
		GetFishData(config, pool, chatNames, data, numMonths, monthYear, logInstance, mode)
	default:
		logs.Logs().Warn().
			Str("DB", data).
			Msg("That does not exist")
	}
}

func GetFishData(config utils.Config, pool *pgxpool.Pool, chatNames string, data string, numMonths int, monthYear string, logInstance int, mode string) {

	var wg sync.WaitGroup
	fishChan := make(chan []FishInfo)

	logs.Logs().Info().Msgf("Checking new data")

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
				urls := CreateURL(chatName, numMonths, monthYear, logInstance, config)
				fishData := ProcessFishDataForChat(urls, chatName, data, chat, pool, mode)
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
				urls := CreateURL(chatName, numMonths, monthYear, logInstance, config)
				fishData := ProcessFishDataForChat(urls, chatName, data, chat, pool, mode)
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

func ProcessFishDataForChat(urls []string, chatName string, data string, Chat utils.ChatInfo, pool *pgxpool.Pool, mode string) []FishInfo {
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
		latestTournamentDate, err = getLatestCatchDateFromDatabase(ctx, pool, chatName, "tournaments"+chatName)
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
			fishData, err := GetFishDataFromURL(url, chatName, data, pool, latestCatchDate, latestBagDate, latestTournamentDate)
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

	CheckTables := []string{fishinfotable, tableName, tableNameBag, playerdatatable}

	for _, table := range CheckTables {
		if err := utils.EnsureTableExists(pool, table); err != nil {
			logs.Logs().Error().Err(err).
				Str("Table", table).
				Msg("Error ensuring table exists")
			return err
		}
	}

	playerids := make(map[string]int)
	lastChatIDs := make(map[string]int)
	newBagCounts := make(map[string]int)
	newFishCounts := make(map[string]int)
	newResultCounts := make(map[string]int)

	didwealreadycheckiftableexists := make(map[string]bool)

	for chatName, chat := range config.Chat {
		if chat.CheckFData {
			newBagCounts[chatName] = 0
			newFishCounts[chatName] = 0
			newResultCounts[chatName] = 0
		}
	}

	date1, _ := utils.ParseDate("2024-06-06 00:00:00") // When logs ivr started using utc in the logs
	date2, _ := utils.ParseDate("2024-03-31 03:00:00") // When normal time changed to summer time

	for _, fish := range allFish {

		if _, ok := playerids[fish.Player]; !ok {
			playerID, err := playerdata.GetPlayerID(pool, fish.Player, fish.Date, fish.Chat)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Player", fish.Player).
					Msg("Error getting player ID")
				return err
			}
			playerids[fish.Player] = playerID
		}

		playerID := playerids[fish.Player]

		// Because logs.ivr didnt use utc but instead had the logs in utc+1/utc+2
		if strings.Contains(fish.Url, "logs.ivr.fi") {

			if fish.Date.Before(date1) && fish.Date.Before(date2) {
				// Subtract one hour (utc+1 to utc)
				fish.Date = fish.Date.Add(time.Hour * -1)
			}
			if fish.Date.Before(date1) && fish.Date.After(date2) {
				// Subtract two hours (utc+2 to utc)
				fish.Date = fish.Date.Add(time.Hour * -2)
			}
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
			if mode == "a" {

				var count int
				err := tx.QueryRow(context.Background(), `
				SELECT COUNT(*) FROM `+tableName+`
				WHERE date <= $1::timestamp AND date >= $2::timestamp
				AND weight = $3 AND player = $4 AND chat = $5 AND fishtype = $6
				`, fish.Date.Add(time.Second*5), fish.Date.Add(time.Second*-5), fish.Weight, fish.Player, fish.Chat, fish.Type).Scan(&count)
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

			fishName, err := GetFishName(pool, fishinfotable, fish.Type)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Type", fish.Type).
					Msg("Error getting fish name")
				return err
			}

			query := fmt.Sprintf("INSERT INTO %s (chatid, fishtype, fishname, weight, catchtype, player, playerid, date, bot, chat, url) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)", tableName)
			_, err = tx.Exec(context.Background(), query, chatID, fish.Type, fishName, fish.Weight, fish.CatchType, fish.Player, playerID, fish.Date, fish.Bot, fish.Chat, fish.Url)
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
				`, fish.Date.Add(time.Second*5), fish.Date.Add(time.Second*-5), fish.Player, fish.Chat, fish.Type).Scan(&count)
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
			_, err = tx.Exec(context.Background(), query, fish.Type, fish.Player, playerID, fish.Date, fish.Bot, fish.Chat, fish.Url)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Query", query).
					Msg("Error inserting bag data")
				return err
			}
			newBagCounts[fish.Chat]++

		// Insert the tournament result into the chats tournament table
		case "result":

			// This is here so that you dont create tables for chats with no tournament results
			tableNameTournament := "tournaments" + fish.Chat
			if _, ok := didwealreadycheckiftableexists[tableNameTournament]; !ok {
				if err := utils.EnsureTableExists(pool, tableNameTournament); err != nil {
					logs.Logs().Error().Err(err).
						Str("Table", tableNameTournament).
						Str("Chat", fish.Chat).
						Msg("Error ensuring table exists")
					return err
				}
				didwealreadycheckiftableexists[tableNameTournament] = true
			}

			// Always checks if the result is already in the db, because you can do +checkin multiple times
			// There is a bug where it will show the checkin result of the previous week if noone checked in, have to manually delete those
			// If you check for more than 7 days, you could end up skipping a result if someone has the exact same result two weeks in a row (very unlikely)
			var count int
			err := tx.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM `+tableNameTournament+`
			WHERE date >= $1::timestamp AND date <= $2::timestamp
			AND player = $3 AND fishcaught = $4 AND placement1 = $5 AND totalweight = $6 AND placement2 = $7 AND biggestfish = $8 AND placement3 = $9
		`, fish.Date.Add(time.Hour*-168), fish.Date, fish.Player, fish.Count, fish.FishPlacement, fish.TotalWeight, fish.WeightPlacement,
				fish.Weight, fish.BiggestFishPlacement).Scan(&count)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Table", tableNameTournament).
					Str("Chat", fish.Chat).
					Msg("Error counting existing results")
				return err
			}
			if count > 0 {
				continue
			}

			query := fmt.Sprintf("INSERT INTO %s ( player, playerid, fishcaught, placement1, totalweight, placement2, biggestfish, placement3, date, bot, chat, url) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)", tableNameTournament)
			_, err = tx.Exec(context.Background(), query, fish.Player, playerID, fish.Count, fish.FishPlacement, fish.TotalWeight,
				fish.WeightPlacement, fish.Weight, fish.BiggestFishPlacement, fish.Date, fish.Bot, fish.Chat, fish.Url)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Table", tableNameTournament).
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
