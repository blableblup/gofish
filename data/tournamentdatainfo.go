package data

import (
	"gofish/logs"
	"gofish/utils"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type TrnmInfo struct {
	Player               string
	PlayerID             int
	FishCaught           int
	TotalWeight          float64
	BiggestFish          float64
	FishPlacement        int
	WeightPlacement      int
	BiggestFishPlacement int
	Bot                  string
	Date                 time.Time
	Chat                 string
	TrnmId               int
	ChatId               int
}

func extractInfoFromTData(result string) []TrnmInfo {
	var Results []TrnmInfo

	infoMatch := regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+),`).FindStringSubmatch(result)
	dateStr := infoMatch[1]
	bot := infoMatch[2]
	player := infoMatch[3]

	fishMatch := regexp.MustCompile(`(\d+) fish: (.*?)[!.]`).FindStringSubmatch(result)
	fishCaught, _ := strconv.Atoi(fishMatch[1])

	weightMatch := regexp.MustCompile(`Together they weighed .*? (\d+(?:\.\d+)?) lbs: (.*?)[!.]`).FindStringSubmatch(result)
	totalWeight, _ := strconv.ParseFloat(weightMatch[1], 64)

	biggestFishMatch := regexp.MustCompile(`Your biggest catch weighed .*? (\d+(?:\.\d+)?) lbs: (.*?)[!.]`).FindStringSubmatch(result)
	biggestFishWeight, _ := strconv.ParseFloat(biggestFishMatch[1], 64)

	date, err := utils.ParseDate(dateStr)
	if err != nil {
		logs.Logs().Fatal().Err(err).Str("Player", player).Str("Date", dateStr).Msgf("Error parsing date for tournament result")
	}

	Results = append(Results, TrnmInfo{
		Date:                 date,
		Bot:                  bot,
		Player:               player,
		FishCaught:           fishCaught,
		TotalWeight:          totalWeight,
		BiggestFish:          biggestFishWeight,
		FishPlacement:        getPlacement(fishMatch[2]),
		WeightPlacement:      getPlacement(weightMatch[2]),
		BiggestFishPlacement: getPlacement(biggestFishMatch[2]),
	})

	return Results
}

func getPlacement(placeStr string) int {

	switch placeStr {
	case "Victory ✨🏆✨":
		return 1
	case "You were the champion ✨🏆✨":
		return 1
	case "That's runner-up 🥈":
		return 2
	case "That's third 🥉":
		return 3
	case "You got third place 🥉":
		return 3
	default:
		placeStr = strings.TrimSuffix(placeStr, " place")
		place, _ := strconv.Atoi(regexp.MustCompile(`\D+$`).ReplaceAllString(placeStr, ""))
		return place
	}
}
