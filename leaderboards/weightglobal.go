package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"path/filepath"
	"strings"
)

func RunWeightGlobal(params LeaderboardParams) {
	board := params.LeaderboardType
	config := params.Config
	date2 := params.Date2
	title := params.Title
	pool := params.Pool
	date := params.Date
	path := params.Path
	mode := params.Mode

	globalRecordWeight := make(map[string]data.FishInfo)
	weightLimit := config.Chat["global"].Weightlimit

	var filePath string

	if path == "" {
		filePath = filepath.Join("leaderboards", "global", "weight.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", "global", path)
	}

	oldWeight, err := ReadWeightRankings(filePath, pool)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Path", filePath).Str("Board", board).Msg("Error reading old leaderboard")
		return
	}

	// Query the database to get the biggest fish per player
	rows, err := pool.Query(context.Background(), `
		SELECT f.playerid, f.weight, f.fishname, f.bot, f.chat AS chatname, f.date, f.catchtype, f.fishid, f.chatid
		FROM fish f
		JOIN (
			SELECT playerid, MAX(weight) AS max_weight
			FROM fish 
			WHERE date < $1
			AND date > $2
			GROUP BY playerid
		) max_fish ON f.playerid = max_fish.playerid AND f.weight = max_fish.max_weight
		WHERE f.weight >= $3`, date, date2, weightLimit)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Board", board).Msg("Error querying database")
		return
	}
	defer rows.Close()

	// Iterate through the query results
	for rows.Next() {
		var fishInfo data.FishInfo

		if err := rows.Scan(&fishInfo.PlayerID, &fishInfo.Weight, &fishInfo.TypeName, &fishInfo.Bot,
			&fishInfo.Chat, &fishInfo.Date, &fishInfo.CatchType, &fishInfo.FishId, &fishInfo.ChatId); err != nil {
			logs.Logs().Error().Err(err).Str("Board", board).Msg("Error scanning row")
			return
		}

		err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Player)
		if err != nil {
			logs.Logs().Error().Err(err).Int("PlayerID", fishInfo.PlayerID).Str("Board", board).Msg("Error retrieving player name for id")
			return
		}

		if fishInfo.Bot == "supibot" {
			err := pool.QueryRow(context.Background(), "SELECT verified FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Verified)
			if err != nil {
				logs.Logs().Error().Err(err).Int("PlayerID", fishInfo.PlayerID).Str("Board", board).Msg("Error retrieving verified status for playerid")
				return
			}
		}

		err = pool.QueryRow(context.Background(), "SELECT fishtype FROM fishinfo WHERE fishname = $1", fishInfo.TypeName).Scan(&fishInfo.Type)
		if err != nil {
			logs.Logs().Error().Err(err).Str("FishName", fishInfo.TypeName).Str("Board", board).Msg("Error retrieving fish type for fish name")
			return
		}

		fishInfo.ChatPfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", fishInfo.Chat, fishInfo.Chat)
		globalRecordWeight[fishInfo.Player] = fishInfo

	}

	if err := rows.Err(); err != nil {
		logs.Logs().Error().Err(err).Str("Board", board).Msg("Error iterating over query results")
		return
	}

	logRecord(globalRecordWeight, oldWeight, board)

	if mode == "check" {
		logs.Logs().Info().Str("Board", board).Msg("Finished checking for new records")
		return
	}

	updateWeightLeaderboard(globalRecordWeight, oldWeight, filePath, board, title)
}

func updateWeightLeaderboard(recordWeight map[string]data.FishInfo, oldWeight map[string]LeaderboardInfo, filePath string, board string, title string) {
	logs.Logs().Info().Str("Board", board).Msg("Updating leaderboard")
	var titleweight string
	if title == "" {
		titleweight = "### Biggest fish caught per player globally\n"
	} else {
		titleweight = fmt.Sprintf("%s\n", title)
	}
	isGlobal := true
	err := writeWeight(filePath, recordWeight, oldWeight, titleweight, isGlobal)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Board", board).Msg("Error writing leaderboard")
	} else {
		logs.Logs().Info().Str("Board", board).Msg("Leaderboard updated successfully")
	}
}
