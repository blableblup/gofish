package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/utils"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
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

	pool, err := data.Connect()
	if err != nil {
		fmt.Println("Error connecting to the database:", err)
		return
	}
	defer pool.Close()

	leaderboardList := strings.Split(leaderboards, ",")

	for _, leaderboard := range leaderboardList {
		switch leaderboard {
		case "count":
			RunCountGlobal(config, pool)
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

	updateWeightLeaderboard(globalRecordWeight)
}

func RunCountGlobal(config utils.Config, pool *pgxpool.Pool) {
	globalCount := make(map[string]data.FishInfo)
	totalcountLimit := config.Chat["global"].Totalcountlimit

	// Process all chats
	for chatName, chat := range config.Chat {
		if !chat.CheckEnabled {
			fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
			continue
		}

		// Query the database to get the count of fish caught by each player
		rows, err := pool.Query(context.Background(), `
            SELECT playerid, COUNT(*) AS fish_count
            FROM fish
            WHERE chat = $1
            GROUP BY playerid
            `, chatName)
		if err != nil {
			fmt.Println("Error querying database:", err)
			return
		}
		defer rows.Close()

		// Iterate through the query results and store fish count for each player
		for rows.Next() {
			var fishInfo data.FishInfo
			if err := rows.Scan(&fishInfo.PlayerID, &fishInfo.Count); err != nil {
				fmt.Println("Error scanning row:", err)
				continue
			}

			// Retrieve player name from the playerdata table
			err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Player)
			if err != nil {
				fmt.Println("Error retrieving player name:", err)
				continue
			}

			// Check if the player is already in the map
			existingFishInfo, exists := globalCount[fishInfo.Player]
			if exists {
				existingFishInfo.Count += fishInfo.Count

				if existingFishInfo.ChatCounts == nil {
					existingFishInfo.ChatCounts = make(map[string]int)
				}
				emoji := config.Chat[chatName].Emoji
				existingFishInfo.ChatCounts[emoji] += fishInfo.Count

				if fishInfo.Count > existingFishInfo.MaxCount {
					existingFishInfo.MaxCount = fishInfo.Count
					existingFishInfo.Chat = emoji
				}
				globalCount[fishInfo.Player] = existingFishInfo
			} else {
				emoji := config.Chat[chatName].Emoji
				globalCount[fishInfo.Player] = data.FishInfo{
					Player:     fishInfo.Player,
					Count:      fishInfo.Count,
					Chat:       emoji,
					MaxCount:   fishInfo.Count,
					ChatCounts: map[string]int{emoji: fishInfo.Count},
				}
			}

		}
	}

	// Filter out players who caught less than the totalcountLimit
	for playerName, fishInfo := range globalCount {
		if fishInfo.Count < totalcountLimit {
			delete(globalCount, playerName)
		}
	}

	updateCountLeaderboard(globalCount)
}

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

func updateCountLeaderboard(globalCount map[string]data.FishInfo) {
	fmt.Println("Updating global count leaderboard...")
	title := "### Most fish caught globally\n"
	isGlobal := true
	filePath := filepath.Join("leaderboards", "global", "count.md")
	err := writeCount(filePath, globalCount, title, isGlobal)
	if err != nil {
		fmt.Println("Error writing global count leaderboard:", err)
	} else {
		fmt.Println("Global count leaderboard updated successfully.")
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
