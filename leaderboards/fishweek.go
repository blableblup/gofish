package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"path/filepath"
	"time"
)

func processFishweek(params LeaderboardParams) {
	chatName := params.ChatName
	config := params.Config
	chat := params.Chat
	pool := params.Pool

	fishweekLimit := chat.Fishweeklimit
	if fishweekLimit == 0 {
		fishweekLimit = config.Chat["default"].Fishweeklimit
	}

	maxFishInWeek := make(map[string]data.FishInfo)

	rows, err := pool.Query(context.Background(), `
	SELECT t.playerid, t.fishcaught, t.bot, t.date
	FROM tournaments`+chatName+` t
	JOIN (
		SELECT playerid, MAX(fishcaught) AS max_count
		FROM tournaments`+chatName+`
		GROUP BY playerid
	) max_t ON t.playerid = max_t.playerid AND t.fishcaught = max_t.max_count
	WHERE t.chat = $1 AND max_count >= $2`, chatName, fishweekLimit)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error querying database")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var bot string
		var date time.Time
		var playerid, count int

		if err := rows.Scan(&playerid, &count, &bot, &date); err != nil {
			logs.Logs().Error().Err(err).Msg("Error scanning row")
			continue
		}

		var playerName string
		err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", playerid).Scan(&playerName)
		if err != nil {
			logs.Logs().Error().Err(err).Msgf("Error retrieving player name for id '%d'", playerid)
			continue
		}

		maxFishInWeek[playerName] = data.FishInfo{
			Count: count,
			Bot:   bot,
			Date:  date,
		}
	}

	if err := rows.Err(); err != nil {
		logs.Logs().Error().Err(err).Msg("Error iterating over query results")
		return
	}

	titlefishw := fmt.Sprintf("### Most fish caught in a single week in tournaments in %s's chat\n", chatName)
	filePath := filepath.Join("leaderboards", chatName, "fishweek.md")
	isGlobal, isType := false, false

	oldFishw, err := ReadTotalcountRankings(filePath, pool)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error reading old fishweek leaderboard")
		return
	}

	logs.Logs().Info().Msgf("Updating fishweek leaderboard for chat '%s' with fish count threshold %d...", chatName, fishweekLimit)
	err = writeCount(filePath, maxFishInWeek, oldFishw, titlefishw, isGlobal, isType)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error writing fishweek leaderboard")
	} else {
		logs.Logs().Info().Msg("Fishweek leaderboard updated successfully")
	}
}
