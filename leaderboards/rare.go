package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"path/filepath"
)

func RunCountFishTypesGlobal(params LeaderboardParams) {
	config := params.Config
	pool := params.Pool

	globalFishTypesCount := make(map[string]data.FishInfo)

	// Process all chats
	for chatName, chat := range config.Chat {
		if !chat.CheckEnabled {
			if chatName != "global" {
				fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
			}
			continue
		}

		// Query the database to get the count of each fish type caught in the chat
		rows, err := pool.Query(context.Background(), `
            SELECT type AS fish_type, COUNT(*) AS type_count
            FROM fish
            WHERE chat = $1
            GROUP BY fish_type
            `, chatName)
		if err != nil {
			fmt.Println("Error querying database:", err)
			return
		}
		defer rows.Close()

		// Iterate through the query results and store fish type count for each chat
		for rows.Next() {
			var fishInfo data.FishInfo
			if err := rows.Scan(&fishInfo.Type, &fishInfo.Count); err != nil {
				fmt.Println("Error scanning row:", err)
				continue
			}

			// Check if the fish type already exists in the map
			emoji := config.Chat[chatName].Emoji
			existingFishInfo, exists := globalFishTypesCount[fishInfo.Type]
			if exists {
				existingFishInfo.Count += fishInfo.Count

				if existingFishInfo.ChatCounts == nil {
					existingFishInfo.ChatCounts = make(map[string]int)
				}
				existingFishInfo.ChatCounts[emoji] += fishInfo.Count

				if fishInfo.Count > existingFishInfo.MaxCount {
					existingFishInfo.MaxCount = fishInfo.Count
					existingFishInfo.Chat = emoji
				}
				globalFishTypesCount[fishInfo.Type] = existingFishInfo
			} else {
				globalFishTypesCount[fishInfo.Type] = data.FishInfo{
					Count:      fishInfo.Count,
					Chat:       emoji,
					MaxCount:   fishInfo.Count,
					ChatCounts: map[string]int{emoji: fishInfo.Count},
				}
			}
		}
	}

	updateFishTypesLeaderboard(globalFishTypesCount)
}

func updateFishTypesLeaderboard(globalFishTypesCount map[string]data.FishInfo) {
	fmt.Println("Updating rarest fish leaderboard...")
	title := "### How many times a fish has been caught\n"
	filePath := filepath.Join("leaderboards", "global", "rare.md")
	isGlobal, isType := true, true
	isFishw := false
	err := writeCount(filePath, globalFishTypesCount, title, isGlobal, isType, isFishw)
	if err != nil {
		fmt.Println("Error writing rarest fish leaderboard:", err)
	} else {
		fmt.Println("Rarest fish leaderboard updated successfully.")
	}
}
