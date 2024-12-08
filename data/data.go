package data

import (
	"context"
	"fmt"
	"gofish/logs"
	"gofish/playerdata"
	"gofish/utils"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4/pgxpool"
)

func GetData(chatNames, data string, numMonths int, monthYear string, mode string) {

	config := utils.LoadConfig()

	pool, err := Connect()
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error connecting to the database")
		return
	}
	defer pool.Close()

	switch data {
	case "f":
		GetFishData(config, pool, chatNames, numMonths, monthYear, mode)
	case "t":
		GetTournamentData(config, pool, chatNames, numMonths, monthYear, mode)
	case "all":
		GetFishData(config, pool, chatNames, numMonths, monthYear, mode)
		GetTournamentData(config, pool, chatNames, numMonths, monthYear, mode)
	default:
		logs.Logs().Warn().
			Str("DB", data).
			Msg("Please specify a valid database type")
	}
}

func GetFishData(config utils.Config, pool *pgxpool.Pool, chatNames string, numMonths int, monthYear string, mode string) {

	switch chatNames {
	case "all":
		var wg sync.WaitGroup
		fishChan := make(chan []FishInfo)

		logs.Logs().Info().Msgf("Checking new fish data")
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
				urls := utils.CreateURL(chatName, numMonths, monthYear, config)
				fishData := ProcessFishData(urls, chatName, chat, pool, mode)
				fishChan <- fishData
			}(chatName, chat)
		}

		go func() {
			wg.Wait()
			close(fishChan)
		}()

		var allFish []FishInfo
		for fishData := range fishChan {
			allFish = append(allFish, fishData...)
		}

		// Sort the final fish data by date
		sort.SliceStable(allFish, func(i, j int) bool {
			return allFish[i].Date.Before(allFish[j].Date)
		})

		if err := insertFishDataIntoDB(allFish, pool, config, mode); err != nil {
			logs.Logs().Error().Err(err).
				Msg("Error inserting fish data into database")
			return
		}
	default:
		logs.Logs().Warn().Msg("Please specify 'all' for chat names") // For now only check "all" chats
	}
}

func ProcessFishData(urls []string, chatName string, Chat utils.ChatInfo, pool *pgxpool.Pool, mode string) []FishInfo {
	var allFish []FishInfo
	var wg sync.WaitGroup
	var mu sync.Mutex

	fishChan := make(chan FishInfo)

	var latestCatchDate, latestBagDate time.Time

	ctx := context.Background()

	if mode == "a" {
		latestCatchDate = time.Time{}
		latestBagDate = time.Time{}
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
	}

	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			fishData, err := FishData(url, chatName, allFish, pool, latestCatchDate, latestBagDate)
			if err != nil {
				logs.Logs().Error().Err(err).
					Msg("Error fetching fish data")
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

// Shared map for tournament + fishdata
var playerids = make(map[string]int)

func insertFishDataIntoDB(allFish []FishInfo, pool *pgxpool.Pool, config utils.Config, mode string) error {

	tx, err := pool.Begin(context.Background())
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error starting transaction")
		return err
	}
	defer tx.Rollback(context.Background())

	fishinfotable := "fishinfo"
	if err := utils.EnsureTableExists(pool, fishinfotable); err != nil {
		logs.Logs().Error().Err(err).
			Str("Table", fishinfotable).
			Msg("Error ensuring table exists")
		return err
	}

	tableName := "fish"
	if err := utils.EnsureTableExists(pool, tableName); err != nil {
		logs.Logs().Error().Err(err).
			Str("Table", tableName).
			Msg("Error ensuring table exists")
		return err
	}

	tableNameBag := "bag"
	if err := utils.EnsureTableExists(pool, tableNameBag); err != nil {
		logs.Logs().Error().Err(err).
			Str("Table", tableNameBag).
			Msg("Error ensuring table exists")
		return err
	}

	// This is only here and not in tournamentdata, because in order to have tournament results
	// fish need to have been caught (unless you only check for tournament results)
	playerdatatable := "playerdata"
	if err := utils.EnsureTableExists(pool, playerdatatable); err != nil {
		logs.Logs().Error().Err(err).
			Str("Table", playerdatatable).
			Msg("Error ensuring playerdata table exists")
		return err
	}

	lastChatIDs := make(map[string]int)
	newFishCounts := make(map[string]int)
	newBagCounts := make(map[string]int)

	for chatName, chat := range config.Chat {
		if chat.CheckFData {
			newFishCounts[chatName] = 0
			newBagCounts[chatName] = 0
		}
	}

	for _, fish := range allFish {

		// Only needed if mode is a since FishData only adds new fish else.
		if mode == "a" {
			logs.Logs().Warn().Msg("This doesnt do anything atm. There needs to be a way to insert 'old' fish which arent in the db. Add this back for bags and fish")
			return nil
		}

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

		if fish.CatchType != "bag" {
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
		} else {
			query := fmt.Sprintf("INSERT INTO %s (bag, player, playerid, date, bot, chat, url) VALUES ($1, $2, $3, $4, $5, $6, $7)", tableNameBag)
			_, err = tx.Exec(context.Background(), query, fish.Type, fish.Player, playerID, fish.Date, fish.Bot, fish.Chat, fish.Url)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Query", query).
					Msg("Error inserting bag data")
				return err
			}
			newBagCounts[fish.Chat]++
		}
	}

	if err := tx.Commit(context.Background()); err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error committing transaction")
		return err
	}

	for chat, count := range newFishCounts {
		if count > 0 {
			logs.Logs().Info().
				Int("Count", count).
				Str("Chat", chat).
				Msg("New fish added into the database for chat")
		} else {
			logs.Logs().Info().
				Int("Count", count).
				Str("Chat", chat).
				Msg("No new fish found for chat")
		}
	}

	for chat, count := range newBagCounts {
		if count > 0 {
			logs.Logs().Info().
				Int("Count", count).
				Str("Chat", chat).
				Msg("New bags added into the database for chat")
		} else {
			logs.Logs().Info().
				Int("Count", count).
				Str("Chat", chat).
				Msg("No new bags found for chat")
		}
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
