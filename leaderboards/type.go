package leaderboards

import (
	"fmt"
	"gofish/logs"
)

func processType(params LeaderboardParams) {
	board := params.LeaderboardType
	boardInfo := params.BoardInfo
	chatName := params.ChatName
	global := params.Global
	mode := params.Mode

	filePath := returnPath(params)

	oldType, err := getJsonBoardString(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error reading old leaderboard")
		return
	}

	recordType, err := boardInfo.GetFunction(params)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error getting type records")
		return
	}

	AreMapsSame := didFishMapChange(params, oldType, recordType)

	if AreMapsSame && mode != "force" {
		logs.Logs().Warn().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Not updating board because there are no changes")
		return
	}

	// dont update the files and return if mode check
	if mode == "check" {
		logs.Logs().Info().
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Finished checking for new records")
		return
	}

	var title string

	if params.Title == "" {
		title = boardInfo.GetTitleFunction(params)
	} else {
		title = fmt.Sprintf("%s\n", params.Title)
	}

	err = writeType(filePath, recordType, oldType, board, title, global)
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

	err = writeRaw(filePath, recordType)
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

func getTypeRecords(params LeaderboardParams) (map[string]BoardData, error) {
	board := params.LeaderboardType
	boardInfo := params.BoardInfo
	chatName := params.ChatName
	global := params.Global
	pool := params.Pool

	recordType := make(map[string]BoardData)
	var results []BoardData
	var err error

	if !global {

		query := boardInfo.GetQueryFunction(params)

		results, err = ReturnFishSliceQuery(params, query, true)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error querying db")
			return recordType, err
		}

	} else {

		query := boardInfo.GetQueryFunction(params)

		results, err = ReturnFishSliceQuery(params, query, false)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error querying db")
			return recordType, err
		}
	}

	for _, result := range results {

		result.Player, _, result.Verified, _, err = PlayerStuff(result.PlayerID, params, pool)
		if err != nil {
			return recordType, err
		}

		result.FishType, err = FishStuff(result.FishName, params)
		if err != nil {
			return recordType, err
		}

		if global {
			result.ChatPfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", result.Chat, result.Chat)
		}

		recordType[result.FishName] = result
	}

	return recordType, nil
}

func writeType(filePath string, recordType map[string]BoardData, oldType map[string]BoardData, board string, title string, global bool) error {

	header := []string{"Rank", "Fish", "Weight in lbs", "Player", "Date in UTC"}

	if global {
		header = append(header, "Chat")
	}

	var sortedTypes []string

	if board != "typefirst" && board != "typelast" {
		sortedTypes = sortMapStringFishInfo(recordType, "weightdesc")
	} else {
		sortedTypes = sortMapStringFishInfo(recordType, "datedesc")
	}

	var data [][]string

	for _, fishName := range sortedTypes {
		weight := recordType[fishName].Weight
		player := recordType[fishName].Player
		fishName := recordType[fishName].FishName
		fishType := recordType[fishName].FishType
		rank := recordType[fishName].Rank
		date := recordType[fishName].Date

		var found bool

		oldWeight := weight
		oldRank := -1

		if info, ok := oldType[fishName]; ok {
			found = true
			oldWeight = info.Weight
			oldRank = info.Rank
		}

		var fishweight string

		// dont show the weight change for typefirst and typelast
		if board != "typefirst" && board != "typelast" {
			weightDifference := weight - oldWeight

			if weightDifference != 0 {
				if weightDifference > 0 {
					fishweight = fmt.Sprintf("%.2f (+%.2f)", weight, weightDifference)
				} else {
					fishweight = fmt.Sprintf("%.2f (%.2f)", weight, weightDifference)
				}
			} else {
				fishweight = fmt.Sprintf("%.2f", weight)
			}
		} else {
			fishweight = fmt.Sprintf("%.2f", weight)
		}

		botIndicator := ""
		if recordType[fishName].Bot == "supibot" && !recordType[fishName].Verified {
			botIndicator = "*"
		}

		var ranks, changeEmoji string

		// dont show medals and changeemoji for typefirst and typelast
		// because they are sorted by date
		if board != "typefirst" && board != "typelast" {
			changeEmoji = ChangeEmoji(rank, oldRank, found)
			ranks = Ranks(rank)
		} else {
			ranks = fmt.Sprintf("%d", rank)
		}

		row := []string{
			fmt.Sprintf("%s %s", ranks, changeEmoji),
			fmt.Sprintf("%s %s", fishType, fishName),
			fishweight,
			fmt.Sprintf("%s%s", player, botIndicator),
			date.Format("2006-01-02 15:04:05"),
		}

		if global {
			row = append(row, recordType[fishName].ChatPfp)
		}

		data = append(data, row)
	}

	var notes []string

	if board != "typefirst" && board != "typelast" {
		notes = append(notes, "If there are multiple records with the same weight, only the player who caught it first is displayed")
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
