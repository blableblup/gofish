package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// can merge this with the normal weight board
// but this ones limit is in int and the others is in float64
// idk doesnt matter
func processWeight2(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	config := params.Config
	global := params.Global
	limit := params.Limit
	chat := params.Chat
	mode := params.Mode

	filePath := returnPath(params)

	oldRecordWeight, err := getJsonBoard(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error reading old leaderboard")
		return
	}

	var rowlimit int

	if limit == "" {
		rowlimit = chat.Rowlimit
		if rowlimit == 0 {
			rowlimit = config.Chat["default"].Rowlimit
		}
	} else {
		rowlimit, err = strconv.Atoi(limit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Limit", limit).
				Str("Board", board).
				Msg("Error converting custom weight limit to int")
			return
		}
	}

	recordWeight, err := getWeightRecords2(params, rowlimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error getting weight records")
		return
	}

	AreMapsSame := didPlayerMapsChange(params, oldRecordWeight, recordWeight)

	if AreMapsSame && mode != "force" {
		logs.Logs().Warn().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Not updating board because there are no changes")
		return
	}

	// Stops the program if it is in "just checking" mode
	if mode == "check" {
		logs.Logs().Info().
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Finished checking for new records")
		return
	}

	var title string

	if params.Title == "" {
		if !global {
			if strings.HasSuffix(chatName, "s") {
				title = fmt.Sprintf("### %d biggest fish caught in %s' chat\n", rowlimit, chatName)
			} else {
				title = fmt.Sprintf("### %d biggest fish caught in %s's chat\n", rowlimit, chatName)
			}
		} else {
			title = fmt.Sprintf("### %d biggest fish caught globally\n", rowlimit)
		}
	} else {
		title = fmt.Sprintf("%s\n", params.Title)
	}

	notlimit := 0.0 // Because the limit for this board is in the title but the func still needs a limit
	err = writeWeight(filePath, recordWeight, oldRecordWeight, title, global, board, notlimit)
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

	err = writeRaw(filePath, recordWeight)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error writing raw leaderboard")
	} else {
		logs.Logs().Info().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Raw leaderboard updated successfully")
	}
}

func getWeightRecords2(params LeaderboardParams, limit int) (map[int]data.FishInfo, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	global := params.Global
	date2 := params.Date2
	pool := params.Pool
	date := params.Date

	recordWeight := make(map[int]data.FishInfo)
	var rows pgx.Rows
	var err error

	if !global {
		rows, err = pool.Query(context.Background(), `
		SELECT playerid, weight, fishname as typename, bot, chat, date, catchtype, fishid, chatid,
		RANK() OVER (ORDER BY weight DESC)
		FROM fish 
		WHERE chat = $1
		AND date < $2
		AND date > $3
		LIMIT $4`, chatName, date, date2, limit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error querying database")
			return recordWeight, err
		}
	} else {
		rows, err = pool.Query(context.Background(), `
		SELECT playerid, weight, fishname as typename, bot, chat, date, catchtype, fishid, chatid,
		RANK() OVER (ORDER BY weight DESC)
		FROM fish 
		WHERE date < $1
		AND date > $2
		LIMIT $3`, date, date2, limit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error querying database")
			return recordWeight, err
		}
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[data.FishInfo])
	if err != nil && err != pgx.ErrNoRows {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error collecting rows")
		return recordWeight, err
	}

	for _, result := range results {

		result.Player, _, result.Verified, _, err = PlayerStuff(result.PlayerID, params, pool)
		if err != nil {
			return recordWeight, err
		}

		result.Type, err = FishStuff(result.TypeName, params)
		if err != nil {
			return recordWeight, err
		}

		if global {
			result.ChatPfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", result.Chat, result.Chat)
		}

		recordWeight[result.FishId] = result

	}

	return recordWeight, nil
}
