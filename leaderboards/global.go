package leaderboards

import (
	"fmt"
	"gofish/data"
	"gofish/utils"
	"os"
	"path/filepath"
)

func RunGlobal(leaderboard string) {
	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		os.Exit(1)
	}

	// Construct the absolute path to the config file
	configFilePath := filepath.Join(wd, "config.json")

	// Load the config from the constructed file path
	config := utils.LoadConfig(configFilePath)

	switch leaderboard {
	case "type":
		RunTypeGlobal(config)
	case "weight":
		RunWeightGlobal(config)
	case "all":
		RunWeightGlobal(config)
		RunTypeGlobal(config)
	default:
		fmt.Println("Please specify a valid leaderboard type.")
	}
}

func RunTypeGlobal(config utils.Config) {
	// Create a map to store combined records
	globalRecordType := make(map[string]data.Record)

	// Process all chats
	for chatName, chat := range config.Chat {
		if !chat.CheckEnabled {
			fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
			continue // Skip processing if check_enabled is false
		}

		oldRecordType, err := ReadTypeRankings(chat.Type)
		if err != nil {
			fmt.Printf("Error reading old type leaderboard for chat '%s': %v\n", chatName, err)
			continue
		}

		// Combine old type records into global record, keeping only the biggest record per fish type
		for fishType, oldRecord := range oldRecordType {
			convertedRecord := ConvertToRecord(oldRecord)

			existingRecord, exists := globalRecordType[fishType]
			if !exists || convertedRecord.Weight > existingRecord.Weight {
				convertedRecord.Chat = config.Chat[chatName].Emoji
				globalRecordType[fishType] = convertedRecord
			}
		}
	}

	// Write the global type leaderboard
	updateTypeLeaderboard(config, globalRecordType)
}

func RunWeightGlobal(config utils.Config) {
	// Create a map to store combined records
	globalRecordWeight := make(map[string]data.Record)

	// Get the weight limit from the "global" configuration
	WeightLimit := config.Chat["global"].Weightlimit

	// Process all chats
	for chatName, chat := range config.Chat {
		if !chat.CheckEnabled {
			fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
			continue // Skip processing if check_enabled is false
		}

		oldRecordWeight, err := ReadWeightRankings(chat.Weight)
		if err != nil {
			fmt.Printf("Error reading old weight leaderboard for chat '%s': %v\n", chatName, err)
			continue
		}

		// Combine old weight records into global record, keeping only the biggest record per player
		for player, oldRecord := range oldRecordWeight {
			convertedRecord := ConvertToRecord(oldRecord)

			if convertedRecord.Weight > WeightLimit {
				existingRecord, exists := globalRecordWeight[player]
				if !exists || convertedRecord.Weight > existingRecord.Weight {
					convertedRecord.Chat = config.Chat[chatName].Emoji
					globalRecordWeight[player] = convertedRecord
				}
			}
		}
	}

	// Write the global weight leaderboard
	updateWeightLeaderboard(config, globalRecordWeight)
}

// Update the type leaderboard
func updateTypeLeaderboard(config utils.Config, recordType map[string]data.Record) {
	fmt.Println("Updating global type leaderboard...")
	title := "### Biggest fish per type caught globally\n"
	isGlobal := true
	err := writeTypeLeaderboard(config.Chat["global"].Type, recordType, title, isGlobal)
	if err != nil {
		fmt.Println("Error writing type leaderboard:", err)
	} else {
		fmt.Println("Global type leaderboard updated successfully.")
	}
}

// Update the weight leaderboard
func updateWeightLeaderboard(config utils.Config, recordWeight map[string]data.Record) {
	fmt.Println("Updating global weight leaderboard...")
	title := "### Biggest fish caught per player globally\n"
	isGlobal := true
	err := writeWeightLeaderboard(config.Chat["global"].Weight, recordWeight, title, isGlobal)
	if err != nil {
		fmt.Println("Error writing weight leaderboard:", err)
	} else {
		fmt.Println("Global weight leaderboard updated successfully.")
	}
}

func ConvertToRecord(info LeaderboardInfo) data.Record {
	return data.Record{
		Weight: info.Weight,
		Type:   info.Type,
		Bot:    info.Bot,
		Player: info.Player,
	}
}
