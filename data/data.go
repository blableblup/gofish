package data

import (
	"context"
	"fmt"
	"gofish/utils"
	"os"
	"path/filepath"
	"strings"
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

	wg.Wait()

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
}

func insertFishDataIntoDB(allFish []FishInfo, chatName string, pool *pgxpool.Pool) error {
	// Construct the SQL statement for inserting fish data
	tableName := "fish" + chatName
	if err := ensureTableExists(pool, tableName); err != nil {
		return err
	}

	// Begin a transaction
	tx, err := pool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())

	// Construct the SQL statement for inserting fish data
	query := fmt.Sprintf("INSERT INTO %s (fish_id, type, typename, weight, catch_type, player, date, bot, chat) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)", tableName)

	var newFishCount int

	for _, fish := range allFish {
		// Check if the fish with the same fish_id already exists in the database
		var count int
		err := tx.QueryRow(context.Background(), "SELECT COUNT(*) FROM "+tableName+" WHERE fish_id = $1", fish.FishId).Scan(&count)
		if err != nil {
			return err
		}

		// Skip fish which already exist
		if count > 0 {
			continue
		}

		// Look up the fish name from the typename database based on the fish type
		var typeName string
		fishtablename := "typename"
		if err := ensureTableExists(pool, fishtablename); err != nil {
			return err
		}
		row := tx.QueryRow(context.Background(), "SELECT typename FROM "+fishtablename+"  WHERE type = $1", fish.Type)
		if err := row.Scan(&typeName); err != nil {
			if err == pgx.ErrNoRows {
				// Fish type doesn't exist in the database, add it
				if err := addFishType(pool, fish.Type); err != nil {
					return err
				}
				// Query the fish name again
				row = tx.QueryRow(context.Background(), "SELECT typename FROM "+fishtablename+"  WHERE type = $1", fish.Type)
				if err := row.Scan(&typeName); err != nil {
					return err
				}
			} else {
				return err
			}
		}

		// Execute the SQL statement to insert the fish data
		_, err = tx.Exec(context.Background(), query, fish.FishId, fish.Type, typeName, fish.Weight, fish.CatchType, fish.Player, fish.Date, fish.Bot, fish.Chat)
		if err != nil {
			return err
		}

		newFishCount++
	}

	if newFishCount > 0 {
		fmt.Printf("Successfully inserted %d new fish into the database for chat '%s'.\n", newFishCount, chatName)
	} else {
		fmt.Printf("No new fish found to insert into the database for chat '%s'.\n", chatName)
	}

	// Commit the transaction
	if err := tx.Commit(context.Background()); err != nil {
		return err
	}

	return nil
}

func ensureTableExists(pool *pgxpool.Pool, tableName string) error {
	// Check if the table exists by querying the information_schema.tables
	var exists bool
	err := pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE lower(table_name) = lower($1))", tableName).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		fmt.Printf("Table '%s' does not exist, creating...\n", tableName)
	}

	// Create the appropriate table if it doesn't exist
	if !exists {
		switch {
		case strings.HasPrefix(tableName, "fish"):
			// Create the fish table
			_, err := pool.Exec(context.Background(), fmt.Sprintf(`
				CREATE TABLE %s (
					fish_id VARCHAR(255) PRIMARY KEY,
					type VARCHAR(255),
					typename VARCHAR(255),
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

			fmt.Printf("Table '%s' created successfully\n", tableName)
		case tableName == "typename":
			// Create the typename table
			_, err := pool.Exec(context.Background(), fmt.Sprintf(`
				CREATE TABLE %s (
					type VARCHAR(255) PRIMARY KEY,
					typename VARCHAR(255)
				)
			`, tableName))
			if err != nil {
				return err
			}

			fmt.Printf("Table '%s' created successfully\n", tableName)
		default:
			return fmt.Errorf("unsupported table name: %s", tableName)
		}
	}

	return nil
}

func addFishType(pool *pgxpool.Pool, fishType string) error {
	// Check if the fish type already exists in the typename table
	var exists bool
	err := pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM typename WHERE type = $1)", fishType).Scan(&exists)
	if err != nil {
		return err
	}

	// If the fish type doesn't exist, prompt the user for the fish name
	if !exists {
		fmt.Printf("New fish type '%s' detected. Please enter the fish name: ", fishType)
		var fishName string
		fmt.Scanln(&fishName)

		// Insert the fish type and name into the typename table
		_, err := pool.Exec(context.Background(), "INSERT INTO typename (type, typename) VALUES ($1, $2)", fishType, fishName)
		if err != nil {
			return err
		}

		// Notify that a new fish type has been added
		fmt.Printf("New fish type '%s' added to the database with name '%s'.\n", fishType, fishName)
	}

	return nil
}
