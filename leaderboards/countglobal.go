package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"path/filepath"
	"strings"
	"time"
)

func RunCountGlobal(params LeaderboardParams) {
	board := params.LeaderboardType
	config := params.Config
	date2 := params.Date2
	title := params.Title
	pool := params.Pool
	date := params.Date
	path := params.Path

	globalCount := make(map[string]data.FishInfo)
	totalcountLimit := config.Chat["global"].Totalcountlimit

	var filePath string

	if path == "" {
		filePath = filepath.Join("leaderboards", "global", "count.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", "global", path)
	}

	isFish := false
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

		// Query the database to get the count of fish caught by each player
		rows, err := pool.Query(context.Background(), `
            SELECT playerid, COUNT(*) AS fish_count
            FROM fish
            WHERE chat = $1
			AND date < $2
			AND date > $3
            GROUP BY playerid
            `, chatName, date, date2)
		if err != nil {
			logs.Logs().Error().Err(err).Str("Chat", chatName).Str("Board", board).Msg("Error querying database for fish count")
			return
		}
		defer rows.Close()

		// Iterate through the query results and store fish count for each player
		for rows.Next() {
			var fishInfo data.FishInfo
			if err := rows.Scan(&fishInfo.PlayerID, &fishInfo.Count); err != nil {
				logs.Logs().Error().Err(err).Str("Chat", chatName).Str("Board", board).Msg("Error scanning row for fish count")
				return
			}

			err := pool.QueryRow(context.Background(), "SELECT name, firstfishdate FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Player, &fishInfo.Date)
			if err != nil {
				logs.Logs().Error().Err(err).Int("PlayerID", fishInfo.PlayerID).Str("Board", board).Msg("Error retrieving player name for id")
				return
			}
			if fishInfo.Date.Before(time.Date(2023, time.September, 14, 0, 0, 0, 0, time.UTC)) {
				fishInfo.Bot = "supibot"
				err := pool.QueryRow(context.Background(), "SELECT verified FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Verified)
				if err != nil {
					logs.Logs().Error().Err(err).Int("PlayerID", fishInfo.PlayerID).Str("Board", board).Msg("Error retrieving verified status for playerid")
					return
				}
			}

			pfp := fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chatName, chatName)
			existingFishInfo, exists := globalCount[fishInfo.Player]
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
				globalCount[fishInfo.Player] = existingFishInfo
			} else {
				globalCount[fishInfo.Player] = data.FishInfo{
					Player:     fishInfo.Player,
					Count:      fishInfo.Count,
					Chat:       pfp,
					MaxCount:   fishInfo.Count,
					Bot:        fishInfo.Bot,
					Verified:   fishInfo.Verified,
					ChatCounts: map[string]int{pfp: fishInfo.Count},
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

	updateCountLeaderboard(globalCount, oldCount, filePath, board, title, totalcountLimit)
}

func updateCountLeaderboard(globalCount map[string]data.FishInfo, oldCount map[string]LeaderboardInfo, filePath string, board string, title string, countlimit int) {
	logs.Logs().Info().Str("Board", board).Msg("Updating leaderboard")
	var titlecount string
	if title == "" {
		titlecount = "### Most fish caught globally\n"
	} else {
		titlecount = fmt.Sprintf("%s\n", title)
	}
	isGlobal := true
	err := writeCount(filePath, globalCount, oldCount, titlecount, isGlobal, board, countlimit)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Board", board).Msg("Error writing leaderboard")
	} else {
		logs.Logs().Info().Str("Board", board).Msg("Leaderboard updated successfully")
	}
}
