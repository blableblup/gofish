package data

import (
	"context"
	"fmt"
	"gofish/logs"
	"gofish/playerdata"
	"gofish/utils"
	"sort"
	"sync"

	"github.com/jackc/pgx/v4/pgxpool"
)

func GetData(chatNames, data string, numMonths int, monthYear string, mode string) {

	config := utils.LoadConfig()

	pool, err := Connect()
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error connecting to the database")
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
		logs.Logs().Warn().Msg("Please specify a valid database type")
	}
}

func GetFishData(config utils.Config, pool *pgxpool.Pool, chatNames string, numMonths int, monthYear string, mode string) {

	switch chatNames {
	case "all":
		var wg sync.WaitGroup
		fishChan := make(chan []FishInfo)

		logs.Logs().Info().Msgf("Checking new fish data")
		for chatName, chat := range config.Chat {
			if !chat.CheckEnabled {
				if chatName != "global" && chatName != "default" {
					logs.Logs().Warn().Msgf("Skipping chat '%s' because check_enabled is false", chatName)
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

		if err := insertFishDataIntoDB(allFish, pool, mode); err != nil {
			logs.Logs().Error().Err(err).Msg("Error inserting fish data into database")
			return
		}
	default:
		logs.Logs().Warn().Msg("Please specify 'all' for chat names") // For now only check "all" chats
	}
}

func ProcessFishData(urls []string, chatName string, Chat utils.ChatInfo, pool *pgxpool.Pool, mode string) []FishInfo {
	var allFish []FishInfo
	var wg sync.WaitGroup
	var mu sync.Mutex // Mutex for synchronizing access to allFish

	fishChan := make(chan FishInfo)

	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			fishData, err := FishData(url, chatName, allFish, pool, mode)
			if err != nil {
				logs.Logs().Error().Err(err).Msg("Error fetching fish data")
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

func insertFishDataIntoDB(allFish []FishInfo, pool *pgxpool.Pool, mode string) error {

	tx, err := pool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())

	lastChatIDs := make(map[string]int)
	newFishCounts := make(map[string]int)

	for _, fish := range allFish {

		if _, ok := newFishCounts[fish.Chat]; !ok {
			newFishCounts[fish.Chat] = 0
		}
		tableName := "fish"
		if err := utils.EnsureTableExists(pool, tableName); err != nil {
			return err
		}

		// Only needed if mode is a since FishData only adds new fish else.
		if mode == "a" {
			var count int
			err := tx.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM `+tableName+`
			WHERE EXTRACT(year FROM date) = EXTRACT(year FROM $1::timestamp)
			AND EXTRACT(month FROM date) = EXTRACT(month FROM $2::timestamp)
			AND EXTRACT(day FROM date) = EXTRACT(day FROM $3::timestamp)
			AND EXTRACT(hour FROM date) = EXTRACT(hour FROM $4::timestamp)
			AND EXTRACT(minute FROM date) = EXTRACT(minute FROM $5::timestamp)
			AND EXTRACT(second FROM date) = EXTRACT(second FROM $6::timestamp)
			AND weight = $7 AND player = $8
		`, fish.Date, fish.Date, fish.Date, fish.Date, fish.Date, fish.Date, fish.Weight, fish.Player).Scan(&count)
			if err != nil {
				return err
			}
			if count > 0 {
				continue
			}
		}

		if _, ok := lastChatIDs[fish.Chat]; !ok {
			lastChatID, err := getLastChatIDFromDB(pool, fish.Chat, tableName)
			if err != nil {
				return err
			}
			lastChatIDs[fish.Chat] = lastChatID
		}

		lastChatIDs[fish.Chat]++
		chatID := lastChatIDs[fish.Chat]

		playerID, err := playerdata.GetPlayerID(pool, fish.Player, fish.Date, fish.Chat)
		if err != nil {
			return err
		}

		fishinfotable := "fishinfo"
		if err := utils.EnsureTableExists(pool, fishinfotable); err != nil {
			return err
		}
		fishName, err := GetFishName(pool, fishinfotable, fish.Type)
		if err != nil {
			return err
		}

		query := fmt.Sprintf("INSERT INTO %s (chatid, fishtype, fishname, weight, catchtype, player, playerid, date, bot, chat) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)", tableName)
		_, err = tx.Exec(context.Background(), query, chatID, fish.Type, fishName, fish.Weight, fish.CatchType, fish.Player, playerID, fish.Date, fish.Bot, fish.Chat)
		if err != nil {
			return err
		}

		newFishCounts[fish.Chat]++
	}

	if err := tx.Commit(context.Background()); err != nil {
		return err
	}

	for chat, count := range newFishCounts {
		if count > 0 {
			logs.Logs().Info().Msgf("Successfully inserted %d new fish into the database for chat '%s'", count, chat)
		} else {
			logs.Logs().Info().Msgf("No new fish found to insert into the database for chat '%s'", chat)
		}
	}

	return nil
}

func getLastChatIDFromDB(pool *pgxpool.Pool, chatName string, tablename string) (int, error) {
	var lastChatID int

	query := fmt.Sprintf("SELECT COALESCE(MAX(chatid), 0) FROM %s WHERE chat = $1", tablename)
	row := pool.QueryRow(context.Background(), query, chatName)
	if err := row.Scan(&lastChatID); err != nil {
		return 0, err
	}
	return lastChatID, nil
}
