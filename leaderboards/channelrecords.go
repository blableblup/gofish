package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"path/filepath"
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
	path := params.Path

	var filePath, titlerecords string
	var weightlimit float64

	if path == "" {
		filePath = filepath.Join("leaderboards", chatName, "records.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", chatName, path)
	}

	oldChannelRecords, err := getJsonBoard(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error reading old leaderboard")
		return
	}

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
func getRecords(params LeaderboardParams, weightlimit float64) (map[int]data.FishInfo, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	global := params.Global
	date2 := params.Date2
	pool := params.Pool
	date := params.Date

	recordFish := make(map[int]data.FishInfo)

	for {
		var fishInfo data.FishInfo

		if global {
			err := pool.QueryRow(context.Background(), `
		SELECT f.playerid, f.weight, f.fishname, f.bot, f.chat AS chatname, f.date, f.catchtype, f.fishid, f.chatid
		FROM fish f
		JOIN (
			SELECT min(date) AS min_date
			FROM fish 
			WHERE weight >= $1
			AND date < $2
	  		AND date > $3
		) min_fish ON f.date = min_fish.min_date`, weightlimit, date, date2).Scan(&fishInfo.PlayerID, &fishInfo.Weight, &fishInfo.TypeName, &fishInfo.Bot,
				&fishInfo.Chat, &fishInfo.Date, &fishInfo.CatchType, &fishInfo.FishId, &fishInfo.ChatId)
			if err != nil && err != pgx.ErrNoRows {
				logs.Logs().Error().Err(err).
					Str("Chat", chatName).
					Str("Board", board).
					Msg("Error querying fish database for channel record")
				return nil, err
			} else if err == pgx.ErrNoRows {
				break
			}
		} else {
			err := pool.QueryRow(context.Background(), `
		SELECT f.playerid, f.weight, f.fishname, f.bot, f.chat AS chatname, f.date, f.catchtype, f.fishid, f.chatid
		FROM fish f
		JOIN (
			SELECT min(date) AS min_date
			FROM fish 
			WHERE weight >= $1
			AND date < $2
	  		AND date > $3
			AND chat = $4
		) min_fish ON f.date = min_fish.min_date`, weightlimit, date, date2, chatName).Scan(&fishInfo.PlayerID, &fishInfo.Weight, &fishInfo.TypeName, &fishInfo.Bot,
				&fishInfo.Chat, &fishInfo.Date, &fishInfo.CatchType, &fishInfo.FishId, &fishInfo.ChatId)
			if err != nil && err != pgx.ErrNoRows {
				logs.Logs().Error().Err(err).
					Str("Chat", chatName).
					Str("Board", board).
					Msg("Error querying fish database for channel record")
				return nil, err
			} else if err == pgx.ErrNoRows {
				break
			}
		}

		err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Player)
		if err != nil {
			logs.Logs().Error().Err(err).
				Int("PlayerID", fishInfo.PlayerID).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error retrieving player name for id")
			return nil, err
		}

		if fishInfo.Bot == "supibot" {
			err := pool.QueryRow(context.Background(), "SELECT verified FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Verified)
			if err != nil {
				logs.Logs().Error().Err(err).
					Int("PlayerID", fishInfo.PlayerID).
					Str("Board", board).
					Str("Chat", chatName).
					Msg("Error retrieving verified status for playerid")
				return nil, err
			}
		}

		err = pool.QueryRow(context.Background(), "SELECT fishtype FROM fishinfo WHERE fishname = $1", fishInfo.TypeName).Scan(&fishInfo.Type)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("FishName", fishInfo.TypeName).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error retrieving fish type for fish name")
			return nil, err
		}

		if global {
			fishInfo.ChatPfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", fishInfo.Chat, fishInfo.Chat)
		}

		// A new channel record can never be 0.001 lbs bigger than the last one so this should never skip any records
		weightlimit = fishInfo.Weight + 0.001
		recordFish[fishInfo.FishId] = fishInfo
	}

	return recordFish, nil
}
