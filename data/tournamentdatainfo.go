package data

import (
	"fmt"
	"gofish/utils"
	"regexp"
	"strconv"
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

var TrnmPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+), (?:\p{So}\s)?The results are in! You caught 🪣 (\d+) fish: (\d+)th place\. Together they weighed (?:\p{So}\s)?(\d+(?:\.\d+)?) lbs: (\d+)th place\. Your biggest catch weighed 🎣 (\d+(?:\.\d+)?) lbs: (\d+)th place\.`)
var Trnm2Pattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+), Last week... You caught 🪣 (\d+) fish: (\d+)th place\. Together they weighed (?:\p{So}\s)?(\d+(?:\.\d+)?) lbs: (\d+)th place\. Your biggest catch weighed 🎣 (\d+(?:\.\d+)?) lbs: (\d+)th place\.`)

func extractInfoFromTPatterns(textContent string, patterns []*regexp.Regexp) []TrnmInfo {
	var Results []TrnmInfo

	for _, pattern := range patterns {
		for _, match := range pattern.FindAllStringSubmatch(textContent, -1) {

			var extractFunc func([]string) TrnmInfo
			switch pattern {
			case TrnmPattern:
				extractFunc = extractInfoFromTrnmPattern
			case Trnm2Pattern:
				extractFunc = extractInfoFromTrnmPattern
			}

			Results = append(Results, extractFunc(match))
		}
	}

	return Results
}

func extractInfoFromTrnmPattern(match []string) TrnmInfo {
	dateStr := match[1]
	bot := match[2]
	player := match[3]
	fishCaught, _ := strconv.Atoi(match[4])
	fishPlacement, _ := strconv.Atoi(match[5])
	totalWeight, _ := strconv.ParseFloat(match[6], 64)
	weightPlacement, _ := strconv.Atoi(match[7])
	biggestFishWeight, _ := strconv.ParseFloat(match[8], 64)
	biggestFishPlacement, _ := strconv.Atoi(match[9])

	date, err := utils.ParseDate(dateStr)
	if err != nil {
		fmt.Printf("Error parsing date for result by player '%s' at '%s'.", player, dateStr)
	}

	return TrnmInfo{
		Date:                 date,
		Bot:                  bot,
		Player:               player,
		FishCaught:           fishCaught,
		TotalWeight:          totalWeight,
		BiggestFish:          biggestFishWeight,
		FishPlacement:        fishPlacement,
		WeightPlacement:      weightPlacement,
		BiggestFishPlacement: biggestFishPlacement,
	}
}
