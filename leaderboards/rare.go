package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"path/filepath"
	"strings"
)

func RunCountFishTypesGlobal(params LeaderboardParams) {
	board := params.LeaderboardType
	config := params.Config
	date2 := params.Date2
	pool := params.Pool
	date := params.Date
	path := params.Path

	globalFishTypesCount := make(map[string]data.FishInfo)

	var filePath string

	if path == "" {
		filePath = filepath.Join("leaderboards", "global", "rare.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", "global", path)
	}

	isFish := true
	oldCount, err := ReadTotalcountRankings(filePath, pool, isFish)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Path", filePath).Str("Board", board).Msg("Error reading old leaderboard")
		return
	}

	// Process all chats
	for chatName, chat := range config.Chat {
		if !chat.CheckFData {
			if chatName != "global" && chatName != "default" {
				logs.Logs().Warn().Str("Chat", chatName).Msg("Skipping chat because checkfdata is false")
			}
			continue
		}

		// Query the database to get the count of each fish type caught in the chat
		rows, err := pool.Query(context.Background(), `
            SELECT fishname, COUNT(*) AS type_count
            FROM fish
            WHERE chat = $1
			AND date < $2
			AND date > $3
            GROUP BY fishname
            `, chatName, date, date2)
		if err != nil {
			logs.Logs().Error().Err(err).Str("Chat", chatName).Msg("Error querying database for rarest fish")
			return
		}
		defer rows.Close()

		// Iterate through the query results and store fish type count for each chat
		for rows.Next() {
			var fishInfo data.FishInfo
			if err := rows.Scan(&fishInfo.TypeName, &fishInfo.Count); err != nil {
				logs.Logs().Error().Err(err).Str("Chat", chatName).Msg("Error scanning row for rarest fish")
				continue
			}

			err = pool.QueryRow(context.Background(), "SELECT fishtype FROM fishinfo WHERE fishname = $1", fishInfo.TypeName).Scan(&fishInfo.Type)
			if err != nil {
				logs.Logs().Error().Err(err).Str("Fish name", fishInfo.TypeName).Msg("Error retrieving fish type for fish name")
				continue
			}

			// Check if the fish type already exists in the map
			pfp := fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chatName, chatName)
			existingFishInfo, exists := globalFishTypesCount[fishInfo.Type]
			if exists {
				existingFishInfo.Count += fishInfo.Count

				if existingFishInfo.ChatCounts == nil {
					existingFishInfo.ChatCounts = make(map[string]int)
				}
				existingFishInfo.ChatCounts[pfp] += fishInfo.Count

				if fishInfo.Count > existingFishInfo.MaxCount {
					existingFishInfo.MaxCount = fishInfo.Count
					existingFishInfo.Chat = pfp
				}
				globalFishTypesCount[fishInfo.Type] = existingFishInfo
			} else {
				globalFishTypesCount[fishInfo.Type] = data.FishInfo{
					Count:      fishInfo.Count,
					Chat:       pfp,
					MaxCount:   fishInfo.Count,
					ChatCounts: map[string]int{pfp: fishInfo.Count},
				}
			}
		}
	}

	updateFishTypesLeaderboard(globalFishTypesCount, oldCount, filePath, board)
}

func updateFishTypesLeaderboard(globalFishTypesCount map[string]data.FishInfo, oldCount map[string]LeaderboardInfo, filePath string, board string) {
	logs.Logs().Info().Str("Board", board).Msg("Updating leaderboard")
	title := "### How many times a fish has been caught\n"
	isGlobal, isType := true, true
	err := writeCount(filePath, globalFishTypesCount, oldCount, title, isGlobal, isType)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Path", filePath).Str("Board", board).Msg("Error writing leaderboard")
	} else {
		logs.Logs().Info().Str("Board", board).Msg("Leaderboard updated successfully")
	}
}
