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

func getJsonBoardString(filePath string) (map[string]data.FishInfo, error) {

	oldBoard := make(map[string]data.FishInfo)

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

func writeRawString(filePath string, board map[string]data.FishInfo) error {

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

func sortPlayerRecords(record map[int]data.FishInfo) []int {

	ids := make([]int, 0, len(record))
	for playerID := range record {
		ids = append(ids, playerID)
	}

	sort.SliceStable(ids, func(i, j int) bool { return record[ids[i]].Player < record[ids[j]].Player })
	sort.SliceStable(ids, func(i, j int) bool { return record[ids[i]].Weight > record[ids[j]].Weight })
	sort.SliceStable(ids, func(i, j int) bool { return record[ids[i]].Count > record[ids[j]].Count })

	return ids

}

func sortFishRecords(recordFish map[string]data.FishInfo) []string {

	fishy := make([]string, 0, len(recordFish))
	for fish := range recordFish {
		fishy = append(fishy, fish)
	}

	sort.SliceStable(fishy, func(i, j int) bool { return recordFish[fishy[i]].Player < recordFish[fishy[j]].Player })
	sort.SliceStable(fishy, func(i, j int) bool { return recordFish[fishy[i]].TypeName < recordFish[fishy[j]].TypeName })
	sort.SliceStable(fishy, func(i, j int) bool { return recordFish[fishy[i]].Weight > recordFish[fishy[j]].Weight })
	sort.SliceStable(fishy, func(i, j int) bool { return recordFish[fishy[i]].Count > recordFish[fishy[j]].Count })

	return fishy
}

// If maps are same length, check if the player renamed or has an updated record
// Trophy board is also using this. For trophy the weights are the players points
func didWeightMapsChange(params LeaderboardParams, oldBoard map[int]data.FishInfo, newBoard map[int]data.FishInfo) bool {
	var mapsarethesame = true

	for playerID := range newBoard {
		_, exists := oldBoard[playerID]
		if !exists {
			if params.LeaderboardType != "trophy" {
				logs.Logs().Info().
					Str("Board", params.LeaderboardType).
					Str("Chat", newBoard[playerID].Chat).
					Str("Date", newBoard[playerID].Date.Format("2006-01-02 15:04:05 UTC")).
					Float64("Weight", newBoard[playerID].Weight).
					Str("CatchType", newBoard[playerID].CatchType).
					Str("FishName", newBoard[playerID].TypeName).
					Str("FishType", newBoard[playerID].Type).
					Str("Player", newBoard[playerID].Player).
					Msg("New weight record")
			}
			mapsarethesame = false
		} else {
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
						Msg("Updated weight record")
				}
				mapsarethesame = false
			}
			if oldBoard[playerID].Player != newBoard[playerID].Player {
				mapsarethesame = false
			}
		}
	}
	return mapsarethesame
}

// For the fish leaderboards
func didFishMapChange(params LeaderboardParams, oldBoard map[string]data.FishInfo, newBoard map[string]data.FishInfo) bool {
	var mapsarethesame = true

	for fishName := range newBoard {
		_, exists := oldBoard[fishName]
		if !exists {

			if params.LeaderboardType != "rare" {

				logs.Logs().Info().
					Str("Board", params.LeaderboardType).
					Str("Chat", newBoard[fishName].Chat).
					Str("Date", newBoard[fishName].Date.Format("2006-01-02 15:04:05 UTC")).
					Float64("Weight", newBoard[fishName].Weight).
					Str("CatchType", newBoard[fishName].CatchType).
					Str("FishName", newBoard[fishName].TypeName).
					Str("FishType", newBoard[fishName].Type).
					Str("Player", newBoard[fishName].Player).
					Msg("New fish record")
			}

			mapsarethesame = false
		} else {
			if params.LeaderboardType != "rare" {

				if oldBoard[fishName].Weight != newBoard[fishName].Weight {

					logs.Logs().Info().
						Str("Board", params.LeaderboardType).
						Str("Chat", newBoard[fishName].Chat).
						Str("Date", newBoard[fishName].Date.Format("2006-01-02 15:04:05 UTC")).
						Float64("WeightOld", oldBoard[fishName].Weight).
						Float64("Weight", newBoard[fishName].Weight).
						Str("CatchType", newBoard[fishName].CatchType).
						Str("FishName", newBoard[fishName].TypeName).
						Str("FishType", newBoard[fishName].Type).
						Str("Player", newBoard[fishName].Player).
						Msg("Updated fish record")

					mapsarethesame = false
				}
				if oldBoard[fishName].Player != newBoard[fishName].Player {
					mapsarethesame = false
				}
			}

			if params.LeaderboardType == "rare" {
				if oldBoard[fishName].Count != newBoard[fishName].Count {
					mapsarethesame = false
				}
			}
			// In case the emoji of a fish gets updated
			if oldBoard[fishName].Type != newBoard[fishName].Type {
				mapsarethesame = false
			}
		}
	}
	return mapsarethesame
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
