package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"path/filepath"
	"strings"
)

func RunTypeGlobal(params LeaderboardParams) {
	board := params.LeaderboardType
	date2 := params.Date2
	title := params.Title
	pool := params.Pool
	date := params.Date
	path := params.Path
	mode := params.Mode

	globalRecordType := make(map[string]data.FishInfo)

	var filePath string

	if path == "" {
		filePath = filepath.Join("leaderboards", "global", "type.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", "global", path)
	}

	oldType, err := ReadTypeRankings(filePath, pool)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Path", filePath).Str("Board", board).Msg("Error reading old global type leaderboard")
		return
	}

	// Query the database to get the biggest fish per type
	rows, err := pool.Query(context.Background(), `
		SELECT f.weight, f.fishname, f.bot, f.chat AS chatname, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
			SELECT fishname, MAX(weight) AS max_weight
			FROM fish 
			WHERE date < $1
			AND date > $2
			GROUP BY fishname
		) AS sub
		ON f.fishname = sub.fishname AND f.weight = sub.max_weight
		AND f.fishid = (
			SELECT MIN(fishid)
			FROM fish
			WHERE fishname = sub.fishname AND weight = sub.max_weight
		)`, date, date2)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Board", board).Msg("Error querying database")
		return
	}
	defer rows.Close()

	// Iterate through the query results
	for rows.Next() {
		var fishInfo data.FishInfo

		if err := rows.Scan(&fishInfo.Weight, &fishInfo.TypeName, &fishInfo.Bot, &fishInfo.Chat,
			&fishInfo.Date, &fishInfo.CatchType, &fishInfo.FishId, &fishInfo.ChatId, &fishInfo.PlayerID); err != nil {
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
		globalRecordType[fishInfo.Type] = fishInfo
	}

	if err := rows.Err(); err != nil {
		logs.Logs().Error().Err(err).Str("Board", board).Msg("Error iterating over query results")
		return
	}

	logRecord(globalRecordType, oldType, board)

	if mode == "check" {
		logs.Logs().Info().Str("Board", board).Msg("Finished checking for new records")
		return
	}

	updateTypeLeaderboard(globalRecordType, oldType, filePath, board, title)
}

func updateTypeLeaderboard(recordType map[string]data.FishInfo, oldType map[string]LeaderboardInfo, filePath string, board string, title string) {
	logs.Logs().Info().Str("Board", board).Msg("Updating leaderboard")
	var titletype string
	if title == "" {
		titletype = "### Biggest fish per type caught globally\n"
	} else {
		titletype = fmt.Sprintf("%s\n", title)
	}
	isGlobal := true
	err := writeType(filePath, recordType, oldType, titletype, isGlobal)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Board", board).Msg("Error writing leaderboard")
	} else {
		logs.Logs().Info().Str("Board", board).Msg("Leaderboard updated successfully")
	}
}
