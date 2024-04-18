package leaderboards

import (
	"fmt"
	"gofish/data"
	"gofish/utils"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
)

func Leaderboards(leaderboards string, chatNames string, mode string) {
	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		os.Exit(1)
	}

	configFilePath := filepath.Join(wd, "config.json")
	config := utils.LoadConfig(configFilePath)

	leaderboardList := strings.Split(leaderboards, ",")

	pool, err := data.Connect()
	if err != nil {
		fmt.Println("Error connecting to the database:", err)
		return
	}
	defer pool.Close()

	for _, leaderboard := range leaderboardList {
		switch leaderboard {
		case "count":
			Count(config, chatNames, pool, mode)
		case "weight":
			Weight(config, chatNames, pool, mode)
		case "type":
			Type(config, chatNames, pool, mode)
		case "trophy":

		case "fishw":

		case "all":
			fmt.Println("Updating all leaderboards...")
			Weight(config, chatNames, pool, mode)
			Type(config, chatNames, pool, mode)

		default:
			fmt.Println("Invalid leaderboard specified:", leaderboard)

		}
	}
}

func Weight(config utils.Config, chatNames string, pool *pgxpool.Pool, mode string) {

	switch chatNames {
	case "all":
		// Process all chats
		for chatName, chat := range config.Chat {
			if !chat.CheckEnabled {
				fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
				continue
			}

			fmt.Printf("Checking weight records for chat '%s'.\n", chatName)
			processWeight(chatName, chat, pool, mode)
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

			fmt.Printf("Checking weight records for chat '%s'.\n", chatName)
			processWeight(chatName, chat, pool, mode)
		}
	}
}

func Type(config utils.Config, chatNames string, pool *pgxpool.Pool, mode string) {

	switch chatNames {
	case "all":
		// Process all chats
		for chatName, chat := range config.Chat {
			if !chat.CheckEnabled {
				fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
				continue
			}

			fmt.Printf("Checking type records for chat '%s'.\n", chatName)
			processType(chatName, chat, pool, mode)
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

			fmt.Printf("Checking type records for chat '%s'.\n", chatName)
			processType(chatName, chat, pool, mode)
		}
	}
}

func Count(config utils.Config, chatNames string, pool *pgxpool.Pool, mode string) {

	switch chatNames {
	case "all":
		// Process all chats
		for chatName, chat := range config.Chat {
			if !chat.CheckEnabled {
				fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
				continue
			}

			fmt.Printf("Checking chat '%s'.\n", chatName)
			// processWeight(chatName, chat, mode)
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
			// processWeight(chatName, chat, mode)
		}
	}
}
