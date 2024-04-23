package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"path/filepath"
	"time"
)

func RunTypeGlobal(params LeaderboardParams) {
	config := params.Config
	pool := params.Pool

	globalRecordType, newRecordType := make(map[string]data.FishInfo), make(map[string]data.FishInfo)

	// Query the database to get the biggest fish per type
	rows, err := pool.Query(context.Background(), `
		SELECT f.type AS fish_type, f.weight, f.typename, f.bot, f.chat AS chatname, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
			SELECT type, MAX(weight) AS max_weight
			FROM fish 
			GROUP BY type
		) AS sub
		ON f.type = sub.type AND f.weight = sub.max_weight
		AND f.fishid = (
			SELECT MIN(fishid)
			FROM fish
			WHERE type = sub.type AND weight = sub.max_weight
	)`)
	if err != nil {
		fmt.Println("Error querying database:", err)
		return
	}
	defer rows.Close()

	// Iterate through the query results
	for rows.Next() {
		var fishType, typeName, bot, catchtype, chatname string
		var date time.Time
		var fishid, chatid, playerid int
		var weight float64

		if err := rows.Scan(&fishType, &weight, &typeName, &bot, &chatname, &date, &catchtype, &fishid, &chatid, &playerid); err != nil {
			fmt.Println("Error scanning row:", err)
			continue
		}

		// Retrieve player name from the playerdata table
		var playerName string
		err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", playerid).Scan(&playerName)
		if err != nil {
			fmt.Printf("Error retrieving player name for id '%d':\n", playerid)
			continue
		}

		newRecordType[fishType] = data.FishInfo{
			Weight:    weight,
			Player:    playerName,
			TypeName:  typeName,
			Bot:       bot,
			Date:      date,
			CatchType: catchtype,
			Chat:      chatname,
			FishId:    fishid,
			ChatId:    chatid,
		}
	}

	if err := rows.Err(); err != nil {
		fmt.Println("Error iterating over query results:", err)
		return
	}

	for fishType, newTypeRecord := range newRecordType {
		emoji := config.Chat[newTypeRecord.Chat].Emoji
		globalTypeRecord, exists := globalRecordType[fishType]
		if !exists {
			newTypeRecord.Chat = emoji
			globalRecordType[fishType] = newTypeRecord
		} else {
			if newTypeRecord.Weight > globalTypeRecord.Weight {
				newTypeRecord.Chat = emoji
				globalRecordType[fishType] = newTypeRecord
			} else {
				globalRecordType[fishType] = globalTypeRecord
			}
		}
	}

	updateTypeLeaderboard(globalRecordType)
}

func RunWeightGlobal(params LeaderboardParams) {
	config := params.Config

	globalRecordWeight := make(map[string]data.FishInfo)

	WeightLimit := config.Chat["global"].Weightlimit

	// Process all chats
	for chatName, chat := range config.Chat {
		if !chat.CheckEnabled {
			if chatName != "global" {
				fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
			}
			continue
		}

		filePath := filepath.Join("leaderboards", chatName, "weight.md")
		oldRecordWeight, err := ReadWeightRankings(filePath)
		if err != nil {
			fmt.Printf("Error reading old weight leaderboard for chat '%s': %v\n", chatName, err)
			return
		}

		for player, oldRecord := range oldRecordWeight {
			convertedRecord := ConvertToFishInfo(oldRecord)

			if convertedRecord.Weight >= WeightLimit {
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

func RunCountGlobal(params LeaderboardParams) {
	config := params.Config
	pool := params.Pool

	globalCount := make(map[string]data.FishInfo)
	totalcountLimit := config.Chat["global"].Totalcountlimit

	// Process all chats
	for chatName, chat := range config.Chat {
		if !chat.CheckEnabled {
			if chatName != "global" {
				fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
			}
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

			err := pool.QueryRow(context.Background(), "SELECT name, firstfishdate FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Player, &fishInfo.Date)
			if err != nil {
				fmt.Printf("Error retrieving player name for id '%d':\n", fishInfo.PlayerID)
			}
			if fishInfo.Date.Before(time.Date(2023, time.September, 14, 0, 0, 0, 0, time.UTC)) {
				fishInfo.Bot = "supibot"
			}

			// Check if the player is already in the map
			emoji := config.Chat[chatName].Emoji
			existingFishInfo, exists := globalCount[fishInfo.Player]
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
				globalCount[fishInfo.Player] = existingFishInfo
			} else {
				globalCount[fishInfo.Player] = data.FishInfo{
					Player:     fishInfo.Player,
					Count:      fishInfo.Count,
					Chat:       emoji,
					MaxCount:   fishInfo.Count,
					Bot:        fishInfo.Bot,
					ChatCounts: map[string]int{emoji: fishInfo.Count},
				}
			}
		}
	}

	// Filter out players who caught less than the totalcountLimit
	for playerName, fishInfo := range globalCount {
		if fishInfo.Count <= totalcountLimit {
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
	isType, isFishw := false, false
	isGlobal := true
	filePath := filepath.Join("leaderboards", "global", "count.md")
	err := writeCount(filePath, globalCount, title, isGlobal, isType, isFishw)
	if err != nil {
		fmt.Println("Error writing global count leaderboard:", err)
	} else {
		fmt.Println("Global count leaderboard updated successfully.")
	}
}
