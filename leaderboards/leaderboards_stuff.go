package leaderboards

import (
	"encoding/json"
	"fmt"
	"gofish/logs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// name of the md file is by default name of the board in leaderboardconfig
func returnPath(params LeaderboardParams) string {
	board := params.LeaderboardType
	chatName := params.ChatName
	path := params.Path

	var filePath string

	if path == "" {
		filePath = filepath.Join("leaderboards", chatName, board+".md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", chatName, path)
	}

	return filePath
}

// to print whatever struct / map as a json file
func writeRaw(filePath string, data any) error {

	// because for the leaderboards, the filepath used ends with .md
	// need to change that and replace with json
	filePath = strings.TrimSuffix(filePath, filepath.Ext(filePath))

	file, err := os.Create(filePath + ".json")
	if err != nil {
		return err
	}
	defer file.Close()

	bytes, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(file, "%s", bytes)

	return nil
}

func getJsonBoard(filePath string) (map[int]BoardData, error) {

	oldBoard := make(map[int]BoardData)

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

func getJsonBoardString(filePath string) (map[string]BoardData, error) {

	oldBoard := make(map[string]BoardData)

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

func sortMapStringInt(somemap map[string]int, whattosort string) []string {

	blee := make([]string, 0, len(somemap))
	for whatever := range somemap {
		blee = append(blee, whatever)
	}

	switch whattosort {
	case "nameasc":
		sort.SliceStable(blee, func(i, j int) bool { return blee[i] < blee[j] })

	default:
		logs.Logs().Warn().
			Str("WhatToSort", whattosort).
			Msg("idk what to do :(")
	}

	return blee
}

func sortMapIntFishInfo(somemap map[int]BoardData, whattosort string) []int {

	blee := make([]int, 0, len(somemap))
	for whatever := range somemap {
		blee = append(blee, whatever)
	}

	switch whattosort {
	case "datedesc":
		sort.SliceStable(blee, func(i, j int) bool { return somemap[blee[i]].Date.After(somemap[blee[j]].Date) })
	case "weightdesc":
		sort.SliceStable(blee, func(i, j int) bool { return somemap[blee[i]].Player < somemap[blee[j]].Player })
		sort.SliceStable(blee, func(i, j int) bool { return somemap[blee[i]].Weight > somemap[blee[j]].Weight })
	case "countdesc":
		sort.SliceStable(blee, func(i, j int) bool { return somemap[blee[i]].Player < somemap[blee[j]].Player })
		sort.SliceStable(blee, func(i, j int) bool { return somemap[blee[i]].Count > somemap[blee[j]].Count })
	default:
		logs.Logs().Warn().
			Str("WhatToSort", whattosort).
			Msg("idk what to do :(")
	}

	return blee
}

func sortMapStringFishInfo(somemap map[string]BoardData, whattosort string) []string {

	blee := make([]string, 0, len(somemap))
	for whatever := range somemap {
		blee = append(blee, whatever)
	}

	switch whattosort {
	case "dateasc":
		sort.SliceStable(blee, func(i, j int) bool { return somemap[blee[i]].Date.Before(somemap[blee[j]].Date) })
	case "datedesc":
		sort.SliceStable(blee, func(i, j int) bool { return somemap[blee[i]].Date.After(somemap[blee[j]].Date) })
	case "weightdesc":
		sort.SliceStable(blee, func(i, j int) bool { return blee[i] < blee[j] })
		sort.SliceStable(blee, func(i, j int) bool { return somemap[blee[i]].FishName < somemap[blee[j]].FishName })
		sort.SliceStable(blee, func(i, j int) bool { return somemap[blee[i]].Weight > somemap[blee[j]].Weight })
	case "countdesc":
		sort.SliceStable(blee, func(i, j int) bool { return blee[i] < blee[j] })
		sort.SliceStable(blee, func(i, j int) bool { return somemap[blee[i]].FishName < somemap[blee[j]].FishName })
		sort.SliceStable(blee, func(i, j int) bool { return somemap[blee[i]].Count > somemap[blee[j]].Count })
	default:
		logs.Logs().Warn().
			Str("WhatToSort", whattosort).
			Msg("idk what to do :(")
	}

	return blee
}

// If maps are same length, check if the player renamed or has an updated record
func didPlayerMapsChange(params LeaderboardParams, oldBoard map[int]BoardData, newBoard map[int]BoardData) bool {
	var mapsarethesame = true

	for playerID := range newBoard {
		_, exists := oldBoard[playerID]
		if !exists {
			if params.LeaderboardType == "weight" || params.LeaderboardType == "weightglobal" {
				logs.Logs().Info().
					Str("Board", params.LeaderboardType).
					Str("Chat", newBoard[playerID].Chat).
					Str("Date", newBoard[playerID].Date.Format("2006-01-02 15:04:05 UTC")).
					Float64("Weight", newBoard[playerID].Weight).
					Str("CatchType", newBoard[playerID].CatchType).
					Str("FishName", newBoard[playerID].FishName).
					Str("FishType", newBoard[playerID].FishType).
					Str("Player", newBoard[playerID].Player).
					Msg("New weight record")
			}
			mapsarethesame = false
		} else {
			if oldBoard[playerID].Count != newBoard[playerID].Count {
				mapsarethesame = false
			}

			if oldBoard[playerID].Weight != newBoard[playerID].Weight {
				if params.LeaderboardType == "weight" || params.LeaderboardType == "weightglobal" {
					logs.Logs().Info().
						Str("Board", params.LeaderboardType).
						Str("Chat", newBoard[playerID].Chat).
						Str("Date", newBoard[playerID].Date.Format("2006-01-02 15:04:05 UTC")).
						Float64("WeightOld", oldBoard[playerID].Weight).
						Float64("Weight", newBoard[playerID].Weight).
						Str("CatchType", newBoard[playerID].CatchType).
						Str("FishName", newBoard[playerID].FishName).
						Str("FishType", newBoard[playerID].FishType).
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
func didFishMapChange(params LeaderboardParams, oldBoard map[string]BoardData, newBoard map[string]BoardData) bool {
	board := params.LeaderboardType
	var mapsarethesame = true

	for fishName := range newBoard {
		_, exists := oldBoard[fishName]
		if !exists {

			// not logging typelast because that always has changes every week
			if board != "rare" && board != "averageweight" && board != "typelast" {

				logs.Logs().Info().
					Str("Board", board).
					Str("Chat", newBoard[fishName].Chat).
					Str("Date", newBoard[fishName].Date.Format("2006-01-02 15:04:05 UTC")).
					Float64("Weight", newBoard[fishName].Weight).
					Str("CatchType", newBoard[fishName].CatchType).
					Str("FishName", newBoard[fishName].FishName).
					Str("FishType", newBoard[fishName].FishType).
					Str("Player", newBoard[fishName].Player).
					Msg("New fish record")
			}

			mapsarethesame = false
		} else {

			if oldBoard[fishName].Weight != newBoard[fishName].Weight {

				if board != "rare" && board != "averageweight" && board != "typelast" {

					logs.Logs().Info().
						Str("Board", board).
						Str("Chat", newBoard[fishName].Chat).
						Str("Date", newBoard[fishName].Date.Format("2006-01-02 15:04:05 UTC")).
						Float64("WeightOld", oldBoard[fishName].Weight).
						Float64("Weight", newBoard[fishName].Weight).
						Str("CatchType", newBoard[fishName].CatchType).
						Str("FishName", newBoard[fishName].FishName).
						Str("FishType", newBoard[fishName].FishType).
						Str("Player", newBoard[fishName].Player).
						Msg("Updated fish record")
				}
				mapsarethesame = false
			}

			if oldBoard[fishName].Player != newBoard[fishName].Player {
				mapsarethesame = false

			}

			if params.LeaderboardType == "rare" {
				if oldBoard[fishName].Count != newBoard[fishName].Count {
					mapsarethesame = false
				}
			}
			// In case the emoji of a fish gets updated (?)
			if oldBoard[fishName].FishType != newBoard[fishName].FishType {
				mapsarethesame = false
			}
		}
	}
	return mapsarethesame
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

// the date format for the leaderboards is YYYY-MM-DD
// can also be YYYY-MM-DD HH:MM:SS
func isValidDate(date string) bool {
	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}( \d{2}:\d{2}:\d{2})?$`)
	return re.MatchString(date)
}
