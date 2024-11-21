package leaderboards

import (
	"encoding/json"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func getJsonBoard(filePath string) (map[int]data.FishInfo, error) {

	oldBoard := make(map[int]data.FishInfo)

	filePath = strings.TrimSuffix(filePath, filepath.Ext(filePath))

	wd, err := os.Getwd()
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("FilePath", filePath).
			Msg("Error getting current working directory")
		return oldBoard, err
	}

	rawboard := filepath.Join(wd, filePath+".json")

	// This doesnt have to count as an error, because the board could be new
	file, err := os.Open(rawboard)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("FilePath", filePath).
			Msg("Error opening json board")
		return oldBoard, nil
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&oldBoard)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("FilePath", filePath).
			Msg("Error parsing raw board file")
		return oldBoard, err
	}

	return oldBoard, nil
}

// Mabye dont write the entire board but instead only the relevant fields
func writeRaw(filePath string, board map[int]data.FishInfo) error {

	filePath = strings.TrimSuffix(filePath, filepath.Ext(filePath))

	file, err := os.Create(filePath + ".json")
	if err != nil {
		return err
	}
	defer file.Close()

	bytes, err := json.MarshalIndent(board, "", "\t")
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(file, "%s", bytes)

	return nil
}

func sortWeightRecords(recordWeight map[int]data.FishInfo) []int {

	ids := make([]int, 0, len(recordWeight))
	for playerID := range recordWeight {
		ids = append(ids, playerID)
	}

	sort.SliceStable(ids, func(i, j int) bool { return recordWeight[ids[i]].Player < recordWeight[ids[j]].Player })
	sort.SliceStable(ids, func(i, j int) bool { return recordWeight[ids[i]].Weight > recordWeight[ids[j]].Weight })

	return ids

}

// If maps are same length, check if the player renamed or has an updated record
// This replaced the log record function
func didWeightMapsChange(params LeaderboardParams, oldBoard map[int]data.FishInfo, newBoard map[int]data.FishInfo) bool {
	var bla = true

	if len(oldBoard) == len(newBoard) {
		for playerID := range newBoard {
			if oldBoard[playerID].Weight != newBoard[playerID].Weight {
				if params.LeaderboardType != "trophy" {
					logs.Logs().Info().
						Str("Board", params.LeaderboardType).
						Str("Chat", newBoard[playerID].Chat).
						Str("Date", newBoard[playerID].Date.Format("2006-01-02 15:04:05 UTC")).
						Float64("WeightOld", oldBoard[playerID].Weight).
						Float64("Weight", newBoard[playerID].Weight).
						Str("CatchType", newBoard[playerID].CatchType).
						Str("FishName", newBoard[playerID].TypeName).
						Str("FishType", newBoard[playerID].Type).
						Str("Player", newBoard[playerID].Player).
						Msg("Updated/New weight record")
				}
				bla = false
			}
			if oldBoard[playerID].Player != newBoard[playerID].Player {
				bla = false
			}
		}
		return bla
	} else {
		bla = false
		return bla
	}
}

func logRecord(newRecords map[string]data.FishInfo, oldRecords map[string]LeaderboardInfo, board string) {

	// Log new or updated records for the (global) type and weight leaderboards
	for playerORfish, newRecord := range newRecords {
		oldRecord, exists := oldRecords[playerORfish]
		if !exists {
			logs.Logs().Info().
				Str("Date", newRecord.Date.Format(time.RFC3339)).
				Str("Chat", newRecord.Chat).
				Float64("Weight", newRecord.Weight).
				Str("TypeName", newRecord.TypeName).
				Str("CatchType", newRecord.CatchType).
				Str("FishType", newRecord.Type).
				Str("Player", newRecord.Player).
				Str("Board", board).
				Int("ChatID", newRecord.ChatId).
				Int("FishID", newRecord.FishId).
				Msg("New Record")
		} else {
			if board != "typesmall" && board != "typesmallglobal" {
				if newRecord.Weight > oldRecord.Weight {
					logs.Logs().Info().
						Str("Date", newRecord.Date.Format(time.RFC3339)).
						Str("Chat", newRecord.Chat).
						Float64("New_Weight", newRecord.Weight).
						Float64("Old_Weight", oldRecord.Weight).
						Str("TypeName", newRecord.TypeName).
						Str("CatchType", newRecord.CatchType).
						Str("FishType", newRecord.Type).
						Str("Player", newRecord.Player).
						Str("Board", board).
						Int("ChatID", newRecord.ChatId).
						Int("FishID", newRecord.FishId).
						Msg("Updated Record")
				}
			} else {
				if newRecord.Weight < oldRecord.Weight {
					logs.Logs().Info().
						Str("Date", newRecord.Date.Format(time.RFC3339)).
						Str("Chat", newRecord.Chat).
						Float64("New_Weight", newRecord.Weight).
						Float64("Old_Weight", oldRecord.Weight).
						Str("TypeName", newRecord.TypeName).
						Str("CatchType", newRecord.CatchType).
						Str("FishType", newRecord.Type).
						Str("Player", newRecord.Player).
						Str("Board", board).
						Int("ChatID", newRecord.ChatId).
						Int("FishID", newRecord.FishId).
						Msg("Updated Record")
				}
			}
		}
	}
}

func SortMapByCountDesc(fishCaught map[string]data.FishInfo) []string {
	// Create a slice of player names
	players := make([]string, 0, len(fishCaught))
	for player := range fishCaught {
		players = append(players, player)
	}

	sort.SliceStable(players, func(i, j int) bool { return fishCaught[players[i]].Player < fishCaught[players[j]].Player })
	sort.SliceStable(players, func(i, j int) bool { return fishCaught[players[i]].Count > fishCaught[players[j]].Count })

	return players
}

func SortMapByWeightDesc(fishCaught map[string]data.FishInfo) []string {
	// Create a slice of player names
	players := make([]string, 0, len(fishCaught))
	for player := range fishCaught {
		players = append(players, player)
	}

	sort.SliceStable(players, func(i, j int) bool { return fishCaught[players[i]].Player < fishCaught[players[j]].Player })
	sort.SliceStable(players, func(i, j int) bool { return fishCaught[players[i]].Weight > fishCaught[players[j]].Weight })

	return players
}

func Ranks(rank int) string {
	var ranks string

	switch rank {
	case 1:
		ranks = fmt.Sprintf("%d 🥇", rank)
	case 2:
		ranks = fmt.Sprintf("%d 🥈", rank)
	case 3:
		ranks = fmt.Sprintf("%d 🥉", rank)
	default:
		ranks = fmt.Sprintf("%d", rank)
	}

	return ranks
}

func ChangeEmoji(rank int, oldRank int, found bool) string {
	var changeEmoji string

	if found {
		if rank < oldRank {
			changeEmoji = "⬆"
		} else if rank > oldRank {
			changeEmoji = "⬇"
		} else {
			changeEmoji = ""
		}
	} else {
		changeEmoji = "🆕"
	}

	return changeEmoji
}
