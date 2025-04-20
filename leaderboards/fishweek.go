package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"path/filepath"
	"strconv"
	"strings"
)

func processFishweek(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	global := params.Global
	config := params.Config
	title := params.Title
	limit := params.Limit
	chat := params.Chat
	path := params.Path
	mode := params.Mode

	var filePath, titlefishw string
	var fishweekLimit int

	if path == "" {
		filePath = filepath.Join("leaderboards", chatName, "fishweek.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", chatName, path)
	}

	oldFishw, err := getJsonBoard(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error reading old leaderboard")
		return
	}

	if limit == "" {
		fishweekLimit = chat.Fishweeklimit
		if fishweekLimit == 0 {
			fishweekLimit = config.Chat["default"].Fishweeklimit
		}
	} else {
		fishweekLimit, err = strconv.Atoi(limit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Limit", limit).
				Str("Board", board).
				Msg("Error converting custom limit to int")
			return
		}
	}

	maxFishInWeek, err := getFishWeek(params, fishweekLimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error getting leaderboard")
		return
	}

	AreMapsSame := didPlayerMapsChange(params, oldFishw, maxFishInWeek)

	if AreMapsSame && mode != "force" {
		logs.Logs().Warn().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Not updating board because there are no changes")
		return
	}

	if title == "" {
		if strings.HasSuffix(chatName, "s") {
			titlefishw = fmt.Sprintf("### Most fish caught in a single week in tournaments in %s' chat\n", chatName)
		} else {
			titlefishw = fmt.Sprintf("### Most fish caught in a single week in tournaments in %s's chat\n", chatName)
		}
	} else {
		titlefishw = fmt.Sprintf("%s\n", title)
	}

	logs.Logs().Info().
		Str("Board", board).
		Str("Chat", chatName).
		Msg("Updating leaderboard")

	err = writeCount(filePath, maxFishInWeek, oldFishw, titlefishw, global, board, fishweekLimit)
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

func getFishWeek(params LeaderboardParams, fishweeklimit int) (map[int]data.FishInfo, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	date2 := params.Date2
	pool := params.Pool
	date := params.Date

	maxFishInWeek := make(map[int]data.FishInfo)

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

	rows, err := pool.Query(context.Background(), query, date, date2, chatName, fishweeklimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error querying database")
		return maxFishInWeek, err
	}
	defer rows.Close()

	for rows.Next() {
		var fishInfo data.FishInfo

		if err := rows.Scan(&fishInfo.PlayerID, &fishInfo.Count, &fishInfo.Bot, &fishInfo.Date); err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error scanning row")
			return maxFishInWeek, err
		}

		// dont overwrite fishinfo date from the query; but fishinfo date isnt used for anything ?
		fishInfo.Player, _, fishInfo.Verified, _, err = PlayerStuff(fishInfo.PlayerID, params, pool)
		if err != nil {
			return maxFishInWeek, err
		}

		maxFishInWeek[fishInfo.PlayerID] = fishInfo
	}

	if err := rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error iterating over query results")
		return maxFishInWeek, err
	}
	return maxFishInWeek, nil
}
