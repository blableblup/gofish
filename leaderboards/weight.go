package leaderboards

import (
	"context"
	"fmt"
	"gofish/logs"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

func processWeight(params LeaderboardParams) {
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

	var weightlimit float64

	if limit == "" {
		weightlimit = chat.Weightlimit
		if weightlimit == 0 {
			weightlimit = config.Chat["default"].Weightlimit
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

	recordWeight, err := getWeightRecords(params, weightlimit)
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
				title = fmt.Sprintf("### Biggest fish caught per player in %s' chat\n", chatName)
			} else {
				title = fmt.Sprintf("### Biggest fish caught per player in %s's chat\n", chatName)
			}
		} else {
			title = "### Biggest fish caught per player globally\n"
		}
	} else {
		title = fmt.Sprintf("%s\n", params.Title)
	}

	err = writeWeight(filePath, recordWeight, oldRecordWeight, title, global, board, weightlimit)
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

func getWeightRecords(params LeaderboardParams, weightlimit float64) (map[int]BoardData, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	global := params.Global
	date2 := params.Date2
	pool := params.Pool
	date := params.Date

	recordWeight := make(map[int]BoardData)
	var rows pgx.Rows
	var err error

	// Query the database to get the biggest fish per player for the specific chat or globally
	if !global {
		rows, err = pool.Query(context.Background(), `
		SELECT f.playerid, f.weight, f.fishname, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid,
		RANK() OVER (ORDER BY f.weight DESC)
		FROM fish f
		JOIN (
			SELECT playerid, MAX(weight) AS max_weight
			FROM fish 
			WHERE chat = $1
			AND date < $3
	  		AND date > $4
			GROUP BY playerid
		) max_fish ON f.playerid = max_fish.playerid AND f.weight = max_fish.max_weight
		WHERE f.chat = $1 AND f.weight >= $2`, chatName, weightlimit, date, date2)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error querying database")
			return recordWeight, err
		}
	} else {
		rows, err = pool.Query(context.Background(), `
		SELECT f.playerid, f.weight, f.fishname, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid,
		RANK() OVER (ORDER BY f.weight DESC)
		FROM fish f
		JOIN (
			SELECT playerid, MAX(weight) AS max_weight
			FROM fish 
			WHERE date < $1
			AND date > $2
			GROUP BY playerid
		) max_fish ON f.playerid = max_fish.playerid AND f.weight = max_fish.max_weight
		WHERE f.weight >= $3`, date, date2, weightlimit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error querying database")
			return recordWeight, err
		}
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[BoardData])
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

		result.FishType, err = FishStuff(result.FishName, params)
		if err != nil {
			return recordWeight, err
		}

		if global {
			result.ChatPfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", result.Chat, result.Chat)
		}

		recordWeight[result.PlayerID] = result

	}

	return recordWeight, nil
}

func writeWeight(filePath string, recordWeight map[int]BoardData, oldRecordWeight map[int]BoardData, title string, global bool, board string, weightlimit float64) error {

	header := []string{"Rank", "Player", "Fish", "Weight in lbs", "Date in UTC"}

	if global {
		header = append(header, "Chat")
	}

	sortedWeightRecords := sortMapIntFishInfo(recordWeight, "weightdesc")

	var data [][]string

	for _, playerID := range sortedWeightRecords {
		weight := recordWeight[playerID].Weight
		fishType := recordWeight[playerID].FishType
		fishName := recordWeight[playerID].FishName
		rank := recordWeight[playerID].Rank
		player := recordWeight[playerID].Player
		date := recordWeight[playerID].Date

		var found bool

		oldWeight := weight
		oldRank := -1

		if info, ok := oldRecordWeight[playerID]; ok {
			found = true
			oldWeight = info.Weight
			oldRank = info.Rank
		}

		changeEmoji := ChangeEmoji(rank, oldRank, found)

		var fishweight string

		weightDifference := weight - oldWeight

		if weightDifference > 0 {
			fishweight = fmt.Sprintf("%.2f (+%.2f)", weight, weightDifference)
		} else {
			fishweight = fmt.Sprintf("%.2f", weight)
		}

		botIndicator := ""
		if recordWeight[playerID].Bot == "supibot" && !recordWeight[playerID].Verified {
			botIndicator = "*"
		}

		ranks := Ranks(rank)

		row := []string{
			fmt.Sprintf("%s %s", ranks, changeEmoji),
			fmt.Sprintf("%s%s", player, botIndicator),
			fmt.Sprintf("%s %s", fishType, fishName),
			fishweight,
			date.Format("2006-01-02 15:04:05"),
		}

		if global {
			row = append(row, recordWeight[playerID].ChatPfp)
		}

		data = append(data, row)

	}

	var notes []string

	if board == "weight" || board == "weightglobal" {
		notes = append(notes, fmt.Sprintf("Only showing fish weighing >= %v lbs", weightlimit))
	}

	err := writeBoard(filePath, title, header, data, notes)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Msg("Error writing leaderboard")
		return err
	}

	return nil
}
