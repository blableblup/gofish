package leaderboards

import (
	"context"
	"gofish/data"
	"gofish/logs"
	"path/filepath"
	"time"
)

func RunCountGlobal(params LeaderboardParams) {
	config := params.Config
	pool := params.Pool

	globalCount := make(map[string]data.FishInfo)
	totalcountLimit := config.Chat["global"].Totalcountlimit

	filePath := filepath.Join("leaderboards", "global", "count.md")
	isFish := false
	oldCount, err := ReadTotalcountRankings(filePath, pool, isFish)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error reading old global count leaderboard")
		return
	}

	// Process all chats
	for chatName, chat := range config.Chat {
		if !chat.CheckFData {
			if chatName != "global" && chatName != "default" {
				logs.Logs().Warn().Msgf("Skipping chat '%s' because checkfdata is false", chatName)
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
			logs.Logs().Error().Err(err).Msg("Error querying database")
			return
		}
		defer rows.Close()

		// Iterate through the query results and store fish count for each player
		for rows.Next() {
			var fishInfo data.FishInfo
			if err := rows.Scan(&fishInfo.PlayerID, &fishInfo.Count); err != nil {
				logs.Logs().Error().Err(err).Msg("Error scanning row")
				continue
			}

			err := pool.QueryRow(context.Background(), "SELECT name, firstfishdate FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Player, &fishInfo.Date)
			if err != nil {
				logs.Logs().Error().Err(err).Msgf("Error retrieving player name for id '%d'", fishInfo.PlayerID)
			}
			if fishInfo.Date.Before(time.Date(2023, time.September, 14, 0, 0, 0, 0, time.UTC)) {
				fishInfo.Bot = "supibot"
				err := pool.QueryRow(context.Background(), "SELECT verified FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Verified)
				if err != nil {
					logs.Logs().Error().Err(err).Msgf("Error retrieving verified status for playerid '%d'", fishInfo.PlayerID)
				}
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
					Verified:   fishInfo.Verified,
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

	updateCountLeaderboard(globalCount, oldCount, filePath)
}

func updateCountLeaderboard(globalCount map[string]data.FishInfo, oldCount map[string]LeaderboardInfo, filePath string) {
	logs.Logs().Info().Msg("Updating global count leaderboard...")
	title := "### Most fish caught globally\n"
	isType := false
	isGlobal := true
	err := writeCount(filePath, globalCount, oldCount, title, isGlobal, isType)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error writing global count leaderboard")
	} else {
		logs.Logs().Info().Msg("Global count leaderboard updated successfully")
	}
}
