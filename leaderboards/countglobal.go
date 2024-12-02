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
	chatName := params.ChatName
	config := params.Config
	global := params.Global
	title := params.Title
	path := params.Path
	mode := params.Mode

	totalcountLimit := config.Chat["global"].Totalcountlimit
	var filePath, titlecount string

	if path == "" {
		filePath = filepath.Join("leaderboards", "global", "count.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", "global", path)
	}

	oldCount, err := getJsonBoard(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error reading old leaderboard")
		return
	}

	globalCount, err := getCountGlobal(params, totalcountLimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error getting leaderboard")
		return
	}

	AreMapsSame := didPlayerMapsChange(params, oldCount, globalCount)

	if AreMapsSame && mode != "force" {
		logs.Logs().Warn().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Not updating board because there are no changes")
		return
	}

	if title == "" {
		titlecount = "### Most fish caught globally\n"
	} else {
		titlecount = fmt.Sprintf("%s\n", title)
	}

	logs.Logs().Info().
		Str("Board", board).
		Msg("Updating leaderboard")

	err = writeCount(filePath, globalCount, oldCount, titlecount, global, board, totalcountLimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error writing leaderboard")
	} else {
		logs.Logs().Info().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Leaderboard updated successfully")
	}
}

func getCountGlobal(params LeaderboardParams, countlimit int) (map[int]data.FishInfo, error) {
	board := params.LeaderboardType
	config := params.Config
	date2 := params.Date2
	pool := params.Pool
	date := params.Date

	globalCount := make(map[int]data.FishInfo)

	// Process all chats
	for chatName, chat := range config.Chat {
		if !chat.CheckFData {
			if chatName != "global" && chatName != "default" {
				logs.Logs().Warn().
					Str("Board", board).
					Str("Chat", chatName).
					Msg("Skipping chat because checkfdata is false")
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
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error querying database for fish count")
			return globalCount, err
		}
		defer rows.Close()

		// Iterate through the query results and store fish count for each player
		for rows.Next() {
			var fishInfo data.FishInfo
			if err := rows.Scan(&fishInfo.PlayerID, &fishInfo.Count); err != nil {
				logs.Logs().Error().Err(err).
					Str("Board", board).
					Str("Chat", chatName).
					Msg("Error scanning row for fish count")
				return globalCount, err
			}

			err := pool.QueryRow(context.Background(), "SELECT name, firstfishdate FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Player, &fishInfo.Date)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Board", board).
					Str("Chat", chatName).
					Int("PlayerID", fishInfo.PlayerID).
					Msg("Error retrieving player name for id")
				return globalCount, err
			}
			if fishInfo.Date.Before(time.Date(2023, time.September, 14, 0, 0, 0, 0, time.UTC)) {
				fishInfo.Bot = "supibot"
				err := pool.QueryRow(context.Background(), "SELECT verified FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Verified)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Board", board).
						Str("Chat", chatName).
						Int("PlayerID", fishInfo.PlayerID).
						Msg("Error retrieving verified status for playerid")
					return globalCount, err
				}
			}

			pfp := fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chatName, chatName)
			existingFishInfo, exists := globalCount[fishInfo.PlayerID]
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
				globalCount[fishInfo.PlayerID] = existingFishInfo
			} else {
				globalCount[fishInfo.PlayerID] = data.FishInfo{
					Player:     fishInfo.Player,
					Count:      fishInfo.Count,
					Chat:       pfp,
					MaxCount:   fishInfo.Count,
					Bot:        fishInfo.Bot,
					Verified:   fishInfo.Verified,
					Date: 		fishInfo.Date,
					ChatCounts: map[string]int{pfp: fishInfo.Count},
				}
			}
		}
	}

	// Filter out players who caught less than the totalcountLimit
	for playerName, fishInfo := range globalCount {
		if fishInfo.Count <= countlimit {
			delete(globalCount, playerName)
		}
	}

	return globalCount, nil
}
