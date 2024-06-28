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

func processFishweek(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	config := params.Config
	date2 := params.Date2
	title := params.Title
	chat := params.Chat
	pool := params.Pool
	date := params.Date
	path := params.Path
	mode := params.Mode

	var filePath string

	if path == "" {
		filePath = filepath.Join("leaderboards", chatName, "fishweek.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", chatName, path)
	}

	isFish := false
	oldFishw, err := ReadTotalcountRankings(filePath, pool, isFish)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Board", board).Str("Chat", chatName).Msg("Error reading old leaderboard")
		return
	}

	fishweekLimit := chat.Fishweeklimit
	if fishweekLimit == 0 {
		fishweekLimit = config.Chat["default"].Fishweeklimit
	}

	maxFishInWeek := make(map[string]data.FishInfo)

	query := fmt.Sprintf(`
		SELECT t.playerid, t.fishcaught, t.bot, t.date
		FROM tournaments%s t
		JOIN (
			SELECT playerid, MAX(fishcaught) AS max_count
			FROM tournaments%s
			WHERE date < $1 AND date > $2
			GROUP BY playerid
		) max_t ON t.playerid = max_t.playerid AND t.fishcaught = max_t.max_count
		WHERE t.chat = $3 AND max_count >= $4`, chatName, chatName)

	rows, err := pool.Query(context.Background(), query, date, date2, chatName, fishweekLimit)
	if err != nil {
		logs.Logs().Error().Str("Board", board).Str("Chat", chatName).Err(err).Msg("Error querying database")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var fishInfo data.FishInfo

		if err := rows.Scan(&fishInfo.PlayerID, &fishInfo.Count, &fishInfo.Bot, &fishInfo.Date); err != nil {
			logs.Logs().Error().Err(err).Str("Chat", chatName).Str("Board", board).Msg("Error scanning row")
			return
		}

		err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Player)
		if err != nil {
			logs.Logs().Error().Err(err).Int("PlayerID", fishInfo.PlayerID).Str("Board", board).Str("Chat", chatName).Msg("Error retrieving player name for id")
			return
		}

		if fishInfo.Bot == "supibot" {
			err := pool.QueryRow(context.Background(), "SELECT verified FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Verified)
			if err != nil {
				logs.Logs().Error().Err(err).Int("PlayerID", fishInfo.PlayerID).Str("Board", board).Str("Chat", chatName).Msg("Error retrieving verified status for playerid")
				return
			}
		}

		maxFishInWeek[fishInfo.Player] = fishInfo
	}

	if err := rows.Err(); err != nil {
		logs.Logs().Error().Str("Board", board).Str("Chat", chatName).Err(err).Msg("Error iterating over query results")
		return
	}

	// Log new or updated fishweek records
	for playerName, newFishRecord := range maxFishInWeek {
		oldFishRecord, exists := oldFishw[playerName]
		if !exists {
			logs.Logs().Info().
				Str("Date", newFishRecord.Date.Format(time.RFC3339)).
				Str("Chat", newFishRecord.Chat).
				Int("FishCaught", newFishRecord.Count).
				Str("Player", playerName).
				Str("Board", board).
				Msg("New Record")
		} else {
			if newFishRecord.Count > oldFishRecord.Count {
				logs.Logs().Info().
					Str("Date", newFishRecord.Date.Format(time.RFC3339)).
					Str("Chat", newFishRecord.Chat).
					Int("FishCaught", newFishRecord.Count).
					Str("Player", playerName).
					Str("Board", board).
					Msg("Updated Record")
			}
		}
	}

	if mode == "check" {
		logs.Logs().Info().Str("Chat", chatName).Str("Board", board).Msg("Finished checking for new records")
		return
	}

	var titlefishw string
	if title == "" {
		titlefishw = fmt.Sprintf("### Most fish caught in a single week in tournaments in %s's chat\n", chatName)
	} else {
		titlefishw = fmt.Sprintf("%s\n", title)
	}

	isGlobal, isType := false, false

	logs.Logs().Info().Str("Board", board).Str("Chat", chatName).Msg("Updating leaderboard")
	err = writeCount(filePath, maxFishInWeek, oldFishw, titlefishw, isGlobal, isType)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Board", board).Str("Chat", chatName).Msg("Error writing leaderboard")
	} else {
		logs.Logs().Info().Str("Board", board).Str("Chat", chatName).Msg("Leaderboard updated successfully")
	}
}
