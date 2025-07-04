package leaderboards

import (
	"context"
	"fmt"
	"gofish/logs"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

func processChannelRecords(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	config := params.Config
	global := params.Global
	title := params.Title
	limit := params.Limit
	mode := params.Mode
	chat := params.Chat

	filePath := returnPath(params)

	oldChannelRecords, err := getJsonBoard(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error reading old leaderboard")
		return
	}

	var weightlimit float64

	if limit == "" {
		weightlimit = chat.WeightlimitRecords
		if weightlimit == 0 {
			weightlimit = config.Chat["default"].WeightlimitRecords
		}
	} else {
		weightlimit, err = strconv.ParseFloat(limit, 64)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Limit", limit).
				Str("Board", board).
				Msg("Error converting custom weight limit to float64")
			return
		}
	}

	records, err := getRecords(params, weightlimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error getting weight records")
		return
	}

	AreMapsSame := didPlayerMapsChange(params, oldChannelRecords, records)

	if AreMapsSame && mode != "force" {
		logs.Logs().Warn().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Not updating board because there are no changes")
		return
	}

	var titlerecords string

	if title == "" {
		if global {
			titlerecords = "### History of global weight records\n"
		} else {
			if strings.HasSuffix(chatName, "s") {
				titlerecords = fmt.Sprintf("### History of channel records in %s' chat\n", chatName)
			} else {
				titlerecords = fmt.Sprintf("### History of channel records in %s's chat\n", chatName)
			}
		}
	} else {
		titlerecords = fmt.Sprintf("%s\n", title)
	}

	err = writeFishList(filePath, records, oldChannelRecords, titlerecords, global, board, weightlimit)
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

	err = writeRaw(filePath, records)
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

// Select the first fish above the weightlimit -> then select the first fish above that fishes weight -> ...
// Do this until there are no bigger fish anymore (if there is the error pgx.ErrNoRows)
// This will only show the oldest fish if there are multiple channel records with the same weight
func getRecords(params LeaderboardParams, weightlimit float64) (map[int]BoardData, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	global := params.Global
	date2 := params.Date2
	pool := params.Pool
	date := params.Date

	recordFish := make(map[int]BoardData)
	var rows pgx.Rows

	for {

		if global {
			rows, _ = pool.Query(context.Background(), `
		SELECT f.playerid, f.weight, f.fishname, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid
		FROM fish f
		JOIN (
			SELECT min(date) AS min_date
			FROM fish 
			WHERE weight >= $1
			AND date < $2
	  		AND date > $3
		) min_fish ON f.date = min_fish.min_date`, weightlimit, date, date2)

		} else {
			rows, _ = pool.Query(context.Background(), `
		SELECT f.playerid, f.weight, f.fishname, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid
		FROM fish f
		JOIN (
			SELECT min(date) AS min_date
			FROM fish 
			WHERE weight >= $1
			AND date < $2
	  		AND date > $3
			AND chat = $4
		) min_fish ON f.date = min_fish.min_date`, weightlimit, date, date2, chatName)

		}

		result, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[BoardData])
		if err != nil && err != pgx.ErrNoRows {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error querying fish database for channel record")
			return nil, err
		} else if err == pgx.ErrNoRows {
			break
		}

		result.Player, _, result.Verified, _, err = PlayerStuff(result.PlayerID, params, pool)
		if err != nil {
			return recordFish, err
		}

		result.FishType, err = FishStuff(result.FishName, params)
		if err != nil {
			return recordFish, err
		}

		if global {
			result.ChatPfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", result.Chat, result.Chat)
		}

		// A new channel record can never be 0.001 lbs bigger than the last one so this should never skip any records
		weightlimit = result.Weight + 0.001
		recordFish[result.FishId] = result
	}

	return recordFish, nil
}
