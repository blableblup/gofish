package data

import (
	"context"
	"fmt"
	"gofish/utils"
	"os"
	"path/filepath"
	"strings"
	"sync"

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

	switch data {
	case "f":
		GetFishData(config, chatNames, numMonths, monthYear, mode)
	case "all":
		GetFishData(config, chatNames, numMonths, monthYear, mode)
	default:
		fmt.Println("Please specify a valid database type.")
	}
}

func GetFishData(config utils.Config, chatNames string, numMonths int, monthYear string, mode string) {

	switch chatNames {
	case "all":
		// Process all chats
		for chatName, chat := range config.Chat {
			if !chat.CheckEnabled {
				fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
				continue
			}

			fmt.Printf("Checking chat '%s'.\n", chatName)
			urls := utils.CreateURL(chatName, numMonths, monthYear)
			ProcessFishData(urls, chatName, chat, mode)
		}
	case "":
		fmt.Println("Please specify chat names.")
	default:
		// Process specified chat names
		specifiedchatNames := strings.Split(chatNames, ",")
		for _, chatName := range specifiedchatNames {
			chat, ok := config.Chat[chatName]
			if !ok {
				fmt.Printf("Chat '%s' not found in config.\n", chatName)
				continue
			}
			if !chat.CheckEnabled {
				fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
				continue
			}

			fmt.Printf("Checking chat '%s'.\n", chatName)
			urls := utils.CreateURL(chatName, numMonths, monthYear)
			ProcessFishData(urls, chatName, chat, mode)
		}
	}
}

func ProcessFishData(urls []string, chatName string, Chat utils.ChatInfo, mode string) {
	// Define a slice to hold the data of every fish caught
	var allFish []FishInfo
	var allFishMutex sync.Mutex
	var wg sync.WaitGroup

	// Concurrently fetch data from URLs using FishData function
	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			fishData, err := FishData(url, chatName, allFish)
			if err != nil {
				fmt.Println("Error fetching fish data:", err)
				return
			}
			// Lock the mutex before updating the shared slice
			allFishMutex.Lock()
			defer allFishMutex.Unlock()
			// Append fish data to the slice
			allFish = append(allFish, fishData...)
		}(url)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	fmt.Println("Total number of fish caught in the checked URLs:", len(allFish)) // This is temporary since this can be done via sql later
	if mode == "c" {
		fmt.Printf("Finished checking for fish caught for chat '%s'.\n", chatName)
		return
	}

	// Insert fish data into the database
	pool, err := Connect()
	if err != nil {
		fmt.Println("Error connecting to the database:", err)
		return
	}
	defer pool.Close()
	if err := insertFishDataIntoDB(allFish, chatName, pool); err != nil {
		fmt.Println("Error inserting fish data into database:", err)
		return
	}

	fmt.Printf("Successfully inserted fish data into database for chat '%s'.\n", chatName)
}

func insertFishDataIntoDB(allFish []FishInfo, chatName string, pool *pgxpool.Pool) error {
	// Begin a transaction
	tx, err := pool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())

	// Construct the SQL statement for inserting fish data
	tableName := "Fish" + chatName
	if err := ensureTableExists(pool, tableName); err != nil {
		return err
	}
	query := fmt.Sprintf("INSERT INTO %s (fish_id, type, weight, catch_type, player, date, bot, chat) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", tableName)

	// Iterate over all fish data and execute the SQL statement for each fish
	for _, fish := range allFish {
		_, err := tx.Exec(context.Background(), query, fish.FishId, fish.Type, fish.Weight, fish.CatchType, fish.Player, fish.Date, fish.Bot, fish.Chat)
		if err != nil {
			return err
		}
	}

	// Commit the transaction
	if err := tx.Commit(context.Background()); err != nil {
		return err
	}

	return nil
}

// Change the type of pool in the function signature
func ensureTableExists(pool *pgxpool.Pool, tableName string) error {
	// Check if the table exists by querying the information_schema.tables
	var exists bool
	err := pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = $1)", tableName).Scan(&exists)
	if err != nil {
		return err
	}

	// If the table doesn't exist, create it
	if !exists {
		_, err := pool.Exec(context.Background(), fmt.Sprintf(`
            CREATE TABLE %s (
                fish_id VARCHAR(255) PRIMARY KEY,
                type VARCHAR(255),
                weight FLOAT,
                catch_type VARCHAR(255),
                player VARCHAR(255),
                date TIMESTAMP,
                bot VARCHAR(255),
                chat VARCHAR(255)
            )
        `, tableName))
		if err != nil {
			return err
		}
	}

	return nil
}
