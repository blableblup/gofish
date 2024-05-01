package data

import (
	"context"
	"fmt"
	"gofish/playerdata"
	"gofish/utils"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

func GetData(chatNames, data string, numMonths int, monthYear string, mode string) {
	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		os.Exit(1)
	}

	configFilePath := filepath.Join(wd, "config.json")
	config := utils.LoadConfig(configFilePath)

	pool, err := Connect()
	if err != nil {
		fmt.Println("Error connecting to the database:", err)
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
		fmt.Println("Please specify a valid database type.")
	}
}

func GetFishData(config utils.Config, pool *pgxpool.Pool, chatNames string, numMonths int, monthYear string, mode string) {

	switch chatNames {
	case "all":
		var wg sync.WaitGroup
		fishChan := make(chan []FishInfo)

		fmt.Printf("Checking new fish data.\n")
		for chatName, chat := range config.Chat {
			if !chat.CheckEnabled {
				if chatName != "global" {
					fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
				}
				continue
			}

			wg.Add(1)
			go func(chatName string, chat utils.ChatInfo) {
				defer wg.Done()
				urls := utils.CreateURL(chatName, numMonths, monthYear)
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
			fmt.Println("Error inserting fish data into database:", err)
			return
		}
	default:
		fmt.Println("Please specify 'all' for chat names.") // For now only check "all" chats
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
				fmt.Println("Error fetching fish data:", err)
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

		var typeName string
		typenametable := "typename"
		if err := utils.EnsureTableExists(pool, typenametable); err != nil {
			return err
		}
		row := tx.QueryRow(context.Background(), "SELECT typename FROM "+typenametable+"  WHERE type = $1", fish.Type)
		if err := row.Scan(&typeName); err != nil {
			if err == pgx.ErrNoRows {
				// Fish type doesn't exist in the database, add it
				if err := addFishType(pool, fish.Type); err != nil {
					return err
				}
				// Query the fish name again
				row = tx.QueryRow(context.Background(), "SELECT typename FROM "+typenametable+"  WHERE type = $1", fish.Type)
				if err := row.Scan(&typeName); err != nil {
					return err
				}
			} else {
				return err
			}
		}

		query := fmt.Sprintf("INSERT INTO %s (chatid, type, typename, weight, catchtype, player, playerid, date, bot, chat) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)", tableName)
		_, err = tx.Exec(context.Background(), query, chatID, fish.Type, typeName, fish.Weight, fish.CatchType, fish.Player, playerID, fish.Date, fish.Bot, fish.Chat)
		if err != nil {
			return err
		}

		newFishCounts[fish.Chat]++
	}

	for chat, count := range newFishCounts {
		if count > 0 {
			fmt.Printf("Successfully inserted %d new fish into the database for chat '%s'.\n", count, chat)
		} else {
			fmt.Printf("No new fish found to insert into the database for chat '%s'.\n", chat)
		}
	}

	if err := tx.Commit(context.Background()); err != nil {
		return err
	}

	return nil
}

func addFishType(pool *pgxpool.Pool, fishType string) error {

	var exists bool
	err := pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM typename WHERE type = $1)", fishType).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		fmt.Printf("New fish type '%s' detected. Please enter the fish name: ", fishType)
		var fishName string
		fmt.Scanln(&fishName)

		_, err := pool.Exec(context.Background(), "INSERT INTO typename (type, typename) VALUES ($1, $2)", fishType, fishName)
		if err != nil {
			return err
		}

		fmt.Printf("New fish type '%s' added to the database with name '%s'.\n", fishType, fishName)
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
