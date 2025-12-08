package leaderboards

import (
	"fmt"
	"gofish/logs"
	"strconv"
)

func GetTheShiniesForWrappeds(params LeaderboardParams, Wrappeds map[int]*Wrapped, year string) (map[int]*Wrapped, error) {

	Shinies, err := getShinies(params)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", params.ChatName).
			Str("Board", params.LeaderboardType).
			Msg("Error getting shinies for player profiles")
		return Wrappeds, err
	}

	yearInt, err := strconv.Atoi(year)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Year", year).
			Str("Chat", params.ChatName).
			Str("Board", params.LeaderboardType).
			Msg("Error converting year to int")
		return Wrappeds, err
	}

	for _, fish := range Shinies {

		if _, ok := Wrappeds[fish.PlayerID]; ok {

			// check if this was in the year of the wrapped
			if fish.Date.Year() == yearInt {
				fishy := fmt.Sprintf("%s shiny %s", fish.FishType, fish.FishName)

				Wrappeds[fish.PlayerID].RarestFish = append(Wrappeds[fish.PlayerID].RarestFish, fishy)
			}
		}
	}

	return Wrappeds, nil
}

func GetRarestFishForWrappeds(params LeaderboardParams, Wrappeds map[int]*Wrapped, EmojisForFish map[string]string) (map[int]*Wrapped, error) {

	rarestFish, err := GetRarestFishBoard(params)
	if err != nil {
		return Wrappeds, err
	}

	for _, wrapped := range Wrappeds {

		for _, rareFish := range rarestFish {

			for _, seenFish := range wrapped.FishSeen {
				if rareFish == seenFish && len(wrapped.RarestFish) < 5 {
					wrapped.RarestFish = append(wrapped.RarestFish, fmt.Sprintf("%s %s", EmojisForFish[rareFish], rareFish))
				}
			}
		}
	}

	return Wrappeds, nil
}

func GetRarestFishBoard(params LeaderboardParams) ([]string, error) {

	var rarestFish []string

	// get the rarest fish board for that year
	boardData, err := getRarestFish(params)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", "global").
			Str("Board", "rare").
			Msg("Error getting leaderboard")
		return rarestFish, err
	}

	rarestFish = sortMapStringFishInfo(boardData, "countasc")

	return rarestFish, nil
}
