package leaderboards

import (
	"context"
	"gofish/data"
	"gofish/logs"
	"path/filepath"
)

func RunCountFishTypesGlobal(params LeaderboardParams) {
	config := params.Config
	pool := params.Pool

	globalFishTypesCount := make(map[string]data.FishInfo)
	filePath := filepath.Join("leaderboards", "global", "rare.md")
	oldCount, err := ReadTotalcountRankings(filePath, pool)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error reading old rarest fish leaderboard")
		return
	}

	// Process all chats
	for chatName, chat := range config.Chat {
		if !chat.CheckEnabled {
			if chatName != "global" && chatName != "default" {
				logs.Logs().Warn().Msgf("Skipping chat '%s' because check_enabled is false", chatName)
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
			logs.Logs().Error().Err(err).Msg("Error querying database")
			return
		}
		defer rows.Close()

		// Iterate through the query results and store fish type count for each chat
		for rows.Next() {
			var fishInfo data.FishInfo
			if err := rows.Scan(&fishInfo.Type, &fishInfo.Count); err != nil {
				logs.Logs().Error().Err(err).Msg("Error scanning row")
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

	updateFishTypesLeaderboard(globalFishTypesCount, oldCount)
}

func updateFishTypesLeaderboard(globalFishTypesCount map[string]data.FishInfo, oldCount map[string]LeaderboardInfo) {
	logs.Logs().Info().Msg("Updating rarest fish leaderboard...")
	title := "### How many times a fish has been caught\n"
	filePath := filepath.Join("leaderboards", "global", "rare.md")
	isGlobal, isType := true, true
	err := writeCount(filePath, globalFishTypesCount, oldCount, title, isGlobal, isType)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error writing rarest fish leaderboard")
	} else {
		logs.Logs().Info().Msg("Rarest fish leaderboard updated successfully")
	}
}
