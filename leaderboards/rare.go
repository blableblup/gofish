package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/utils"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v4/pgxpool"
)

func RunCountFishTypesGlobal(config utils.Config, pool *pgxpool.Pool) {
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
	err := writeRare(filePath, globalFishTypesCount, title)
	if err != nil {
		fmt.Println("Error writing rarest fish leaderboard:", err)
	} else {
		fmt.Println("Rarest fish leaderboard updated successfully.")
	}
}

func writeRare(filePath string, globalFishTypesCount map[string]data.FishInfo, title string) error {
	oldLeaderboardCount, err := ReadTotalcountRankings(filePath)
	if err != nil {
		return err
	}

	// Ensure that the directory exists before attempting to create the file
	err = os.MkdirAll(filepath.Dir(filePath), 0755)
	if err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "%s", title)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(file, "| Rank | Fish | Times Caught | Chat |")
	_, _ = fmt.Fprintln(file, "|------|------|-------------|------|")

	sortedTypes := SortMapByCountDesc(globalFishTypesCount)

	rank := 1
	prevRank := 1
	prevCount := -1
	occupiedRanks := make(map[int]int)

	for _, fishType := range sortedTypes {
		Count := globalFishTypesCount[fishType].Count
		ChatCounts := globalFishTypesCount[fishType].ChatCounts

		// Increment rank only if the count has changed
		if Count != prevCount {
			rank += occupiedRanks[rank]
			occupiedRanks[rank] = 1
		} else {
			rank = prevRank
			occupiedRanks[rank]++
		}

		var found bool
		oldRank := -1
		oldCount := Count
		oldFishInfo, ok := oldLeaderboardCount[fishType]
		if ok {
			found = true
			oldRank = oldFishInfo.Rank
			oldCount = oldFishInfo.Count
		}

		var counts string

		countDifference := Count - oldCount
		if countDifference > 0 {
			counts = fmt.Sprintf("%d (+%d)", Count, countDifference)
		} else {
			counts = fmt.Sprintf("%d", Count)
		}

		changeEmoji := ChangeEmoji(rank, oldRank, found)

		ranks := Ranks(rank)

		_, _ = fmt.Fprintf(file, "| %s %s | %s | %s |", ranks, changeEmoji, fishType, counts)

		for chat, count := range ChatCounts {
			_, _ = fmt.Fprintf(file, " %s(%d) ", chat, count)
		}
		_, _ = fmt.Fprint(file, "|\n")

		prevCount = Count
		prevRank = rank
	}
	return nil
}
