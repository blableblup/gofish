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

	params := LeaderboardParams{
		Pool:     pool,
		Mode:     mode,
		ChatName: chatNames,
		Config:   config,
	}

	for _, leaderboard := range leaderboardList {
		params.LeaderboardType = leaderboard
		switch leaderboard {
		case "fishw":
			processLeaderboard(config, params, processFishweek)
		case "trophy":
			processLeaderboard(config, params, processTrophy)
		case "weight":
			processLeaderboard(config, params, processWeight)
		case "count":
			processLeaderboard(config, params, processCount)
		case "type":
			processLeaderboard(config, params, processType)
		case "typecount":
			RunCountFishTypesGlobal(params)

		case "all":
			fmt.Println("Updating all leaderboards...")
			RunCountFishTypesGlobal(params)
			params.LeaderboardType = "type"
			processLeaderboard(config, params, processType)
			params.LeaderboardType = "count"
			processLeaderboard(config, params, processCount)
			params.LeaderboardType = "weight"
			processLeaderboard(config, params, processWeight)
			params.LeaderboardType = "trophy"
			processLeaderboard(config, params, processTrophy)
			params.LeaderboardType = "fishweek"
			processLeaderboard(config, params, processFishweek)
		default:
			fmt.Println("＞︿＜ Invalid leaderboard specified:", leaderboard)
		}
	}
}

func processLeaderboard(config utils.Config, params LeaderboardParams, processFunc func(LeaderboardParams)) {
	switch params.ChatName {
	case "all":
		// Process all chats
		for chatName, chat := range config.Chat {
			if !chat.CheckEnabled {
				if chatName != "global" {
					fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
				}
				continue
			}

			fmt.Printf("Checking leaderboard for chat '%s'.\n", chatName)
			params.ChatName = chatName
			params.Chat = chat
			processFunc(params)
		}
	case "global":
		processGlobalLeaderboard(params)
	case "":
		fmt.Println("Please specify chat names.")
	default:
		// Process specified chat names
		specifiedchatNames := strings.Split(params.ChatName, ",")
		for _, chatName := range specifiedchatNames {
			chat, ok := config.Chat[chatName]
			if !ok {
				fmt.Printf("Chat '%s' not found in config.\n", chatName)
				continue
			}
			if !chat.CheckEnabled {
				if chatName != "global" {
					fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
				}
				continue
			}

			fmt.Printf("Checking leaderboard for chat '%s'.\n", chatName)
			params.ChatName = chatName
			params.Chat = chat
			processFunc(params)
		}
	}
}

func processGlobalLeaderboard(params LeaderboardParams) {

	switch params.LeaderboardType {
	case "weight":
		RunWeightGlobal(params)
	case "count":
		RunCountGlobal(params)
	case "type":
		RunTypeGlobal(params)
	default:
		fmt.Printf("（︶^︶） There is no global leaderboard for that board '%s'\n", params.LeaderboardType)
	}
}

type LeaderboardParams struct {
	Chat            utils.ChatInfo
	Pool            *pgxpool.Pool
	Config          utils.Config
	ChatName        string
	Mode            string
	LeaderboardType string
}
