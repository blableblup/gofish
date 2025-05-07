package leaderboards

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store the players in a map, for their verified status, their current name and when they started fishing
// useful when updating all the leaderboards at once; firstfishdate is ignored on some boards where you already get date
// the first fishdate only matters if the player fished on supi, would be better to get min(date) from fish for the player
// firstfishdate is set for when the player first was added and is never updated afterwards,
// so the player might have fished earlier in a chat which wasnt covered for example or during a downtime
// and also, firstfishdate could also be when the player first did + bag
func PlayerStuff(playerID int, params LeaderboardParams, pool *pgxpool.Pool) (string, time.Time, bool, int, error) {

	var name string
	var firstfishdate time.Time
	var verified sql.NullBool
	var twitchID sql.NullInt64

	if _, ok := params.Players[playerID]; !ok {
		err := pool.QueryRow(context.Background(),
			"SELECT name, firstfishdate, verified, twitchid FROM playerdata WHERE playerid = $1",
			playerID).Scan(&name, &firstfishdate, &verified, &twitchID)
		if err != nil {
			logs.Logs().Error().Err(err).
				Int("PlayerID", playerID).
				Str("Chat", params.ChatName).
				Str("Board", params.LeaderboardType).
				Msg("Error retrieving player name for id")
			return name, firstfishdate, verified.Bool, 0, err
		}

		if twitchID.Valid {
			params.Players[playerID] = data.FishInfo{
				Player:   name,
				Date:     firstfishdate,
				Verified: verified.Bool,
				TwitchID: int(twitchID.Int64),
			}
		} else {
			params.Players[playerID] = data.FishInfo{
				Player:   name,
				Date:     firstfishdate,
				Verified: verified.Bool,
				TwitchID: 0,
			}
		}
	}

	return params.Players[playerID].Player, params.Players[playerID].Date, params.Players[playerID].Verified, params.Players[playerID].TwitchID, nil
}

// because some fish had different emotes on supibot, i always get the latest emoji from fishinfo
func FishStuff(fishName string, params LeaderboardParams, pool *pgxpool.Pool) (string, error) {
	var emoji string

	if _, ok := params.FishTypes[fishName]; !ok {
		err := pool.QueryRow(context.Background(), "SELECT fishtype FROM fishinfo WHERE fishname = $1", fishName).Scan(&emoji)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("FishName", fishName).
				Str("Chat", params.ChatName).
				Str("Board", params.LeaderboardType).
				Msg("Error retrieving fish type for fish name")
			return emoji, err
		}

		params.FishTypes[fishName] = emoji

	} else {
		emoji = params.FishTypes[fishName]
	}

	return emoji, nil
}

// get all the fish from the db
func GetAllFishNames(params LeaderboardParams) ([]string, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	pool := params.Pool

	var fishes []string

	err := pool.QueryRow(context.Background(), `select array_agg(fishname) from fishinfo`).Scan(&fishes)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error querying database for all fish")
		return fishes, err
	}

	return fishes, nil
}

// the treasures and mythical fish
// for the player profiles
func ReturnRedAveryTreasure(params LeaderboardParams) ([]string, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	pool := params.Pool

	var redAveryTreasures []string

	err := pool.QueryRow(context.Background(), `select array_agg(fishname) from fishinfo where 'r.a.treasure' = any(tags)`).Scan(&redAveryTreasures)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error querying database for redAveryTreasures")
		return redAveryTreasures, err
	}

	return redAveryTreasures, nil
}

func ReturnOriginalMythicalFish(params LeaderboardParams) ([]string, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	pool := params.Pool

	var ogMythicalFish []string

	err := pool.QueryRow(context.Background(), `select array_agg(fishname) from fishinfo where 'mythic' = any(tags)`).Scan(&ogMythicalFish)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error querying database for ogMythicalFish")
		return ogMythicalFish, err
	}

	return ogMythicalFish, nil
}

func GetAllShinies(params LeaderboardParams) ([]string, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	pool := params.Pool

	var shinies []string

	rows, err := pool.Query(context.Background(), `
		select shiny from fishinfo where shiny != '{}'`)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error querying database")
		return shinies, err
	}
	defer rows.Close()

	for rows.Next() {

		// maybe bread will add multiple shinies for same fish at one point
		// thats why shiny is an array in the fishinfo table
		var shiny []string

		if err := rows.Scan(&shiny); err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error scanning row")
			return shinies, err
		}

		shinies = append(shinies, shiny...)
	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error iterating over rows")
		return shinies, err
	}

	return shinies, nil
}

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

func writeRaw(filePath string, data any) error {

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

func sortMapIntFishInfo(somemap map[int]data.FishInfo, whattosort string) []int {

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

func sortMapStringFishInfo(somemap map[string]data.FishInfo, whattosort string) []string {

	blee := make([]string, 0, len(somemap))
	for whatever := range somemap {
		blee = append(blee, whatever)
	}

	switch whattosort {
	case "dateasc":
		sort.SliceStable(blee, func(i, j int) bool { return somemap[blee[i]].Date.Before(somemap[blee[j]].Date) })
	case "weightdesc":
		sort.SliceStable(blee, func(i, j int) bool { return blee[i] < blee[j] })
		sort.SliceStable(blee, func(i, j int) bool { return somemap[blee[i]].TypeName < somemap[blee[j]].TypeName })
		sort.SliceStable(blee, func(i, j int) bool { return somemap[blee[i]].Weight > somemap[blee[j]].Weight })
	case "countdesc":
		sort.SliceStable(blee, func(i, j int) bool { return blee[i] < blee[j] })
		sort.SliceStable(blee, func(i, j int) bool { return somemap[blee[i]].TypeName < somemap[blee[j]].TypeName })
		sort.SliceStable(blee, func(i, j int) bool { return somemap[blee[i]].Count > somemap[blee[j]].Count })
	default:
		logs.Logs().Warn().
			Str("WhatToSort", whattosort).
			Msg("idk what to do :(")
	}

	return blee
}

// for nameasc and countdesc (only used for playerprofiles)
func sortMapString(somemap map[string]int, whattosort string) []string {

	blee := make([]string, 0, len(somemap))
	for whatever := range somemap {
		blee = append(blee, whatever)
	}

	switch whattosort {
	case "countdesc":
		sort.SliceStable(blee, func(i, j int) bool { return blee[i] < blee[j] })
		sort.SliceStable(blee, func(i, j int) bool { return somemap[blee[i]] > somemap[blee[j]] })
	case "nameasc":
		sort.SliceStable(blee, func(i, j int) bool { return blee[i] < blee[j] })
	default:
		logs.Logs().Warn().
			Str("WhatToSort", whattosort).
			Msg("idk what to do :(")
	}

	return blee
}

// If maps are same length, check if the player renamed or has an updated record
func didPlayerMapsChange(params LeaderboardParams, oldBoard map[int]data.FishInfo, newBoard map[int]data.FishInfo) bool {
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
					Str("FishName", newBoard[playerID].TypeName).
					Str("FishType", newBoard[playerID].Type).
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

			if params.LeaderboardType != "rare" && params.LeaderboardType != "averageweight" {

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

			if oldBoard[fishName].Weight != newBoard[fishName].Weight {

				if params.LeaderboardType != "rare" && params.LeaderboardType != "averageweight" {

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
			// In case the emoji of a fish gets updated
			if oldBoard[fishName].Type != newBoard[fishName].Type {
				mapsarethesame = false
			}
		}
	}
	return mapsarethesame
}

func CatchtypeNames() map[string]string {

	CatchTypesPlayerProfile := map[string]string{
		"normal":       "Normal",
		"egg":          "Hatched egg",
		"release":      "Release bonus",
		"jumped":       "Jumped bonus",
		"mouth":        "Mouth bonus",
		"squirrel":     "Squirrel",
		"squirrelfail": "Squirrel fail", // the squirrels i added manually because bread forgor to update game and you werent supposed to catch them
	}

	return CatchTypesPlayerProfile
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
