package leaderboards

import (
	"fmt"
	"gofish/data"
	"gofish/utils"
	"os"
	"path/filepath"
	"strings"
)

func RunGlobal(leaderboards string) {
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

	leaderboardList := strings.Split(leaderboards, ",")

	for _, leaderboard := range leaderboardList {
		switch leaderboard {
		case "count":
			// Count(config)
		case "weight":
			RunWeightGlobal(config)
		case "type":
			RunTypeGlobal(config)
		case "all":
			fmt.Println("Updating all global leaderboards...")
			RunWeightGlobal(config)
			RunTypeGlobal(config)

		default:
			fmt.Println("Invalid leaderboard specified:", leaderboard)

		}
	}
}

func RunTypeGlobal(config utils.Config) {

	globalRecordType := make(map[string]data.FishInfo)

	// Process all chats
	for chatName, chat := range config.Chat {
		if !chat.CheckEnabled {
			fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
			continue
		}

		filePath := filepath.Join("leaderboards", chatName, "type.md")
		oldRecordType, err := ReadTypeRankings(filePath)
		if err != nil {
			fmt.Printf("Error reading old type leaderboard for chat '%s': %v\n", chatName, err)
			return
		}

		// Combine old type records into global record, keeping only the biggest record per fish type
		for fishType, oldRecord := range oldRecordType {
			convertedRecord := ConvertToFishInfo(oldRecord)

			existingRecord, exists := globalRecordType[fishType]
			if !exists || convertedRecord.Weight > existingRecord.Weight {
				convertedRecord.Chat = config.Chat[chatName].Emoji
				globalRecordType[fishType] = convertedRecord
			}
		}
	}

	// Write the global type leaderboard
	updateTypeLeaderboard(globalRecordType)
}

func RunWeightGlobal(config utils.Config) {

	globalRecordWeight := make(map[string]data.FishInfo)

	// Get the weight limit from the "global" configuration
	WeightLimit := config.Chat["global"].Weightlimit

	// Process all chats
	for chatName, chat := range config.Chat {
		if !chat.CheckEnabled {
			fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
			continue
		}

		filePath := filepath.Join("leaderboards", chatName, "weight.md")
		oldRecordWeight, err := ReadWeightRankings(filePath)
		if err != nil {
			fmt.Printf("Error reading old weight leaderboard for chat '%s': %v\n", chatName, err)
			return
		}

		// Combine old weight records into global record, keeping only the biggest record per player
		for player, oldRecord := range oldRecordWeight {
			convertedRecord := ConvertToFishInfo(oldRecord)

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
	updateWeightLeaderboard(globalRecordWeight)
}

// Update the type leaderboard
func updateTypeLeaderboard(recordType map[string]data.FishInfo) {
	fmt.Println("Updating global type leaderboard...")
	title := "### Biggest fish per type caught globally\n"
	isGlobal := true
	filePath := filepath.Join("leaderboards", "global", "type.md")
	err := writeType(filePath, recordType, title, isGlobal)
	if err != nil {
		fmt.Println("Error writing global type leaderboard:", err)
	} else {
		fmt.Println("Global type leaderboard updated successfully.")
	}
}

// Update the weight leaderboard
func updateWeightLeaderboard(recordWeight map[string]data.FishInfo) {
	fmt.Println("Updating global weight leaderboard...")
	title := "### Biggest fish caught per player globally\n"
	isGlobal := true
	filePath := filepath.Join("leaderboards", "global", "weight.md")
	err := writeWeight(filePath, recordWeight, title, isGlobal)
	if err != nil {
		fmt.Println("Error writing global weight leaderboard:", err)
	} else {
		fmt.Println("Global weight leaderboard updated successfully.")
	}
}

func ConvertToFishInfo(info LeaderboardInfo) data.FishInfo {
	return data.FishInfo{
		Weight: info.Weight,
		Type:   info.Type,
		Bot:    info.Bot,
		Player: info.Player,
	}
}
