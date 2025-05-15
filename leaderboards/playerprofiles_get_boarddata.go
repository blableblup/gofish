package leaderboards

import (
	"fmt"
	"gofish/data"
	"gofish/logs"
	"path/filepath"
	"sort"
)

type BoardDataProfiles struct {
	Count      map[int]data.FishInfo
	Uniquefish map[int]data.FishInfo
	Weight     map[int]data.FishInfo
	Trophy     map[int]data.FishInfo
	Type       map[string]data.FishInfo
	Typesmall  map[string]data.FishInfo
}

// get the data from the other leaderboards json files for all the chats and globally
// i'll update the player profiles after the other boards so that its always the newest data
func GetOtherBoardDataForPlayerProfiles(params LeaderboardParams) (map[string]*BoardDataProfiles, error) {
	boardName := params.LeaderboardType
	config := params.Config

	otherBoardsData := make(map[string]*BoardDataProfiles)

	boardsToGetDataFrom := []string{"count", "uniquefish", "weight", "type", "typesmall", "trophy"}

	for chatName, chat := range config.Chat {

		if _, ok := otherBoardsData[chatName]; !ok {
			otherBoardsData[chatName] = &BoardDataProfiles{
				Count:      make(map[int]data.FishInfo),
				Uniquefish: make(map[int]data.FishInfo),
				Weight:     make(map[int]data.FishInfo),
				Type:       make(map[string]data.FishInfo),
				Typesmall:  make(map[string]data.FishInfo),
			}
		}

		if !chat.BoardsEnabled {
			continue
		}

		for _, board := range boardsToGetDataFrom {

			filePath := filepath.Join("leaderboards", chatName, fmt.Sprintf("%s.md", board))

			switch board {
			case "count":

				boardData, err := getJsonBoard(filePath)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Chat", chatName).
						Str("Path", filePath).
						Str("Board", boardName).
						Msg("Error getting leaderboard")
					return otherBoardsData, err
				}

				otherBoardsData[chatName].Count = boardData

			case "uniquefish":

				boardData, err := getJsonBoard(filePath)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Chat", chatName).
						Str("Path", filePath).
						Str("Board", boardName).
						Msg("Error getting leaderboard")
					return otherBoardsData, err
				}

				otherBoardsData[chatName].Uniquefish = boardData

			case "weight":

				boardData, err := getJsonBoard(filePath)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Chat", chatName).
						Str("Path", filePath).
						Str("Board", boardName).
						Msg("Error getting leaderboard")
					return otherBoardsData, err
				}

				otherBoardsData[chatName].Weight = boardData

			case "trophy":

				boardData, err := getJsonBoard(filePath)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Chat", chatName).
						Str("Path", filePath).
						Str("Board", boardName).
						Msg("Error getting leaderboard")
					return otherBoardsData, err
				}

				otherBoardsData[chatName].Trophy = boardData

			case "type":

				boardData, err := getJsonBoardString(filePath)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Chat", chatName).
						Str("Path", filePath).
						Str("Board", boardName).
						Msg("Error getting leaderboard")
					return otherBoardsData, err
				}

				otherBoardsData[chatName].Type = boardData

			case "typesmall":

				boardData, err := getJsonBoardString(filePath)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Chat", chatName).
						Str("Path", filePath).
						Str("Board", boardName).
						Msg("Error getting leaderboard")
					return otherBoardsData, err
				}

				otherBoardsData[chatName].Typesmall = boardData

			}
		}
	}

	return otherBoardsData, nil
}

func UpdatePlayerProfilesRecords(params LeaderboardParams, Profiles map[int]*PlayerProfile) (map[int]*PlayerProfile, error) {

	otherBoardData, err := GetOtherBoardDataForPlayerProfiles(params)
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error getting data from other boards for player profiles")
		return Profiles, err
	}

	// sort the channel names first
	blee := make([]string, 0, len(otherBoardData))
	for whatever := range otherBoardData {
		blee = append(blee, whatever)
	}

	sort.SliceStable(blee, func(i, j int) bool { return blee[i] < blee[j] })

	for _, chatName := range blee {

		var text string
		if chatName == "global" {
			text = "globally 🌐"
		} else {
			text = fmt.Sprintf("in channel %s", chatName)
		}

		// give the players Records for their fish caught
		for playerID := range otherBoardData[chatName].Count {
			rank := otherBoardData[chatName].Count[playerID].Rank

			// for count, uniquefish, weight, trophy update their record if they are rank <= 10 on that board
			if rank <= 10 {

				// only update Records if they are in the map
				if _, ok := Profiles[playerID]; ok {
					switch rank {
					default:
						Profiles[playerID].Records = append(Profiles[playerID].Records,
							fmt.Sprintf("%dth most fish caught %s !", rank, text))
					case 1:
						Profiles[playerID].Records = append(Profiles[playerID].Records,
							fmt.Sprintf("🥇 Most fish caught %s !", text))
					case 2:
						Profiles[playerID].Records = append(Profiles[playerID].Records,
							fmt.Sprintf("🥈 Second most fish caught %s !", text))
					case 3:
						Profiles[playerID].Records = append(Profiles[playerID].Records,
							fmt.Sprintf("🥉 Third most fish caught %s !", text))
					}
				}

			}
		}

		// give the players Records for their fish seen
		for playerID := range otherBoardData[chatName].Uniquefish {
			rank := otherBoardData[chatName].Uniquefish[playerID].Rank
			if rank <= 10 {

				if _, ok := Profiles[playerID]; ok {
					switch rank {
					default:
						Profiles[playerID].Records = append(Profiles[playerID].Records,
							fmt.Sprintf("%dth most fish seen %s !", rank, text))
					case 1:
						Profiles[playerID].Records = append(Profiles[playerID].Records,
							fmt.Sprintf("🥇 Most fish seen %s !", text))
					case 2:
						Profiles[playerID].Records = append(Profiles[playerID].Records,
							fmt.Sprintf("🥈 Second most fish seen %s !", text))
					case 3:
						Profiles[playerID].Records = append(Profiles[playerID].Records,
							fmt.Sprintf("🥉 Third most fish seen %s !", text))
					}
				}

			}
		}

		// give the players Records for their biggest fish
		for playerID := range otherBoardData[chatName].Weight {
			rank := otherBoardData[chatName].Weight[playerID].Rank
			if rank <= 10 {

				if _, ok := Profiles[playerID]; ok {
					switch rank {
					default:
						Profiles[playerID].Records = append(Profiles[playerID].Records,
							fmt.Sprintf("%dth biggest fish record %s !", rank, text))
					case 1:
						Profiles[playerID].Records = append(Profiles[playerID].Records,
							fmt.Sprintf("🥇 Biggest fish record %s !", text))
					case 2:
						Profiles[playerID].Records = append(Profiles[playerID].Records,
							fmt.Sprintf("🥈 Second biggest fish record %s !", text))
					case 3:
						Profiles[playerID].Records = append(Profiles[playerID].Records,
							fmt.Sprintf("🥉 Third biggest fish record %s !", text))
					}
				}

			}
		}

		for playerID := range otherBoardData[chatName].Trophy {
			rank := otherBoardData[chatName].Trophy[playerID].Rank
			if rank <= 10 {

				if _, ok := Profiles[playerID]; ok {
					switch rank {
					default:
						Profiles[playerID].Records = append(Profiles[playerID].Records,
							fmt.Sprintf("%dth place on tournament leaderboard %s !", rank, text))
					case 1:
						Profiles[playerID].Records = append(Profiles[playerID].Records,
							fmt.Sprintf("🥇 Most points on tournament leaderboard %s !", text))
					case 2:
						Profiles[playerID].Records = append(Profiles[playerID].Records,
							fmt.Sprintf("🥈 Second most points on tournament leaderboard %s !", text))
					case 3:
						Profiles[playerID].Records = append(Profiles[playerID].Records,
							fmt.Sprintf("🥉 Third most points on tournament leaderboard %s !", text))
					}
				}

			}
		}

		// update the record for the biggest per type
		// but these can be a different fish though; so only store as record if the date is the same ?
		// but can also add the other record as a catch in different field ?
		for fishName, fish := range otherBoardData[chatName].Type {

			playerID := otherBoardData[chatName].Type[fishName].PlayerID

			fishType := fmt.Sprintf("%s %s", fishName, fish.Type)

			if _, ok := Profiles[playerID]; ok {

				if Profiles[playerID].FishData[fishType].Biggest.Fish.Date == otherBoardData[chatName].Type[fishName].Date {

					Profiles[playerID].FishData[fishType].Biggest.IsRecord = append(Profiles[playerID].FishData[fishType].Biggest.IsRecord,
						fmt.Sprintf("🥇 Biggest %s %s record %s !", fish.Type, fishName, text))

				}

			}

		}

		// update the record for the smallest per type
		for fishName, fish := range otherBoardData[chatName].Typesmall {

			playerID := otherBoardData[chatName].Typesmall[fishName].PlayerID

			fishType := fmt.Sprintf("%s %s", fishName, fish.Type)

			if _, ok := Profiles[playerID]; ok {

				if Profiles[playerID].FishData[fishType].Smallest.Fish.Date == otherBoardData[chatName].Typesmall[fishName].Date {

					Profiles[playerID].FishData[fishType].Smallest.IsRecord = append(Profiles[playerID].FishData[fishType].Smallest.IsRecord,
						fmt.Sprintf("🥇 Smallest %s %s record %s !", fish.Type, fishName, text))

				}

			}

		}
	}

	// also check if any valid player caught a shiny
	Profiles, err = GetTheShiniesForPlayerProfiles(params, Profiles)
	if err != nil {
		return Profiles, err
	}

	return Profiles, nil
}

func GetTheShiniesForPlayerProfiles(params LeaderboardParams, Profiles map[int]*PlayerProfile) (map[int]*PlayerProfile, error) {

	// use the function from the shiny board for this
	Shinies, err := getShinies(params)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", params.ChatName).
			Str("Board", params.LeaderboardType).
			Msg("Error getting shinies for player profiles")
		return Profiles, err
	}

	for _, fish := range Shinies {

		// because the leaderboard function doesnt use the validplayers
		// only store the shiny if the player is already in the map
		if _, ok := Profiles[fish.PlayerID]; ok {

			// because that board returns a different struct
			profileFish := ProfileFish{
				Fish:       fmt.Sprintf("%s %s", fish.Type, fish.TypeName),
				Weight:     fish.Weight,
				CatchType:  fish.CatchType,
				DateString: fish.Date.Format("2006-01-02 15:04:05 UTC"),
				Chat:       fish.Chat,
			}

			Profiles[fish.PlayerID].Other.ShinyCatch = append(Profiles[fish.PlayerID].Other.ShinyCatch, profileFish)

			Profiles[fish.PlayerID].Other.HasShiny = true

			// update the achievment
			Profiles[fish.PlayerID].Other.Other = append(Profiles[fish.PlayerID].Other.Other, "Has caught a shiny !")

		}
	}

	return Profiles, nil
}
