package leaderboards

import (
	"fmt"
	"gofish/data"
	"gofish/logs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func processType(params LeaderboardParams) {
	board := params.LeaderboardType
	boardInfo := params.BoardInfo
	chatName := params.ChatName
	global := params.Global
	title := params.Title
	path := params.Path
	mode := params.Mode

	var filePath string

	if path == "" {
		filePath = filepath.Join("leaderboards", chatName, board+".md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", chatName, path)
	}

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

	if title == "" {
		title = boardInfo.GetTitleFunction(params)
	} else {
		title = fmt.Sprintf("%s\n", title)
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

func getTypeRecords(params LeaderboardParams) (map[string]data.FishInfo, error) {
	board := params.LeaderboardType
	boardInfo := params.BoardInfo
	chatName := params.ChatName
	global := params.Global
	pool := params.Pool

	recordType := make(map[string]data.FishInfo)
	var results []data.FishInfo
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

		result.Type, err = FishStuff(result.TypeName, params)
		if err != nil {
			return recordType, err
		}

		if global {
			result.ChatPfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", result.Chat, result.Chat)
		}

		recordType[result.TypeName] = result
	}

	return recordType, nil
}

func writeType(filePath string, recordType map[string]data.FishInfo, oldType map[string]data.FishInfo, board string, title string, global bool) error {

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "%s", title)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(file, "| Rank | Fish | Weight in lbs | Player | Date in UTC |"+func() string {
		if global {
			return " Chat |"
		}
		return ""
	}())
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(file, "|------|--------|-----------|---------|------|"+func() string {
		if global {
			return "-------|"
		}
		return ""
	}())
	if err != nil {
		return err
	}

	var sortedTypes []string

	if board != "typefirst" && board != "typelast" {
		sortedTypes = sortMapStringFishInfo(recordType, "weightdesc")
	} else {
		sortedTypes = sortMapStringFishInfo(recordType, "datedesc")
	}

	for _, fishName := range sortedTypes {
		weight := recordType[fishName].Weight
		player := recordType[fishName].Player
		fishName := recordType[fishName].TypeName
		fishType := recordType[fishName].Type
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

		_, _ = fmt.Fprintf(file, "| %s %s | %s %s | %s | %s%s | %s |", ranks, changeEmoji, fishType, fishName, fishweight, player, botIndicator, date.Format("2006-01-02 15:04:05"))
		if global {
			_, _ = fmt.Fprintf(file, " %s |", recordType[fishName].ChatPfp)
		}
		_, err = fmt.Fprintln(file)
		if err != nil {
			return err
		}
	}

	if board != "typefirst" && board != "typelast" {
		_, _ = fmt.Fprint(file, "\n_If there are multiple records with the same weight, only the player who caught it first is displayed_\n")
	}
	_, _ = fmt.Fprintf(file, "\n_Last updated at %s_", time.Now().In(time.UTC).Format("2006-01-02 15:04:05 UTC"))

	return nil
}
