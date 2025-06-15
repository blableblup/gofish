package data

import (
	"gofish/logs"
	"gofish/utils"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type FishInfo struct {
	Player    string  `json:"player,omitempty"`
	PlayerID  int     `json:"playerid,omitempty"`
	Weight    float64 `json:"weight,omitempty"`
	FishType  string  `json:"fishtype,omitempty"`
	FishName  string  `json:"fishname,omitempty"`
	CatchType string  `json:"catchtype,omitempty"`

	Bag []string `json:"bag,omitempty"`

	// for tournament results
	FishPlacement        int     `json:"fishplacement,omitempty"`
	Count                int     `json:"count,omitempty"`
	WeightPlacement      int     `json:"weightplacement,omitempty"`
	TotalWeight          float64 `json:"totalweight,omitempty"`
	BiggestFishPlacement int     `json:"biggestfishplacement,omitempty"`

	Date time.Time `json:"date,omitempty"`
	Chat string    `json:"chat,omitempty"`
	Url  string    `json:"url,omitempty"`
	Bot  string    `json:"bot,omitempty"`
}

var TournamentPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): [@👥]\s?(\w+), (📣 The results are in!|Last week[.][.][.]) You caught 🪣 (\d+) fish: (.*?)[!.] Together they weighed .*? ([\d.]+) lbs: (.*?)[!.] Your biggest catch weighed .*? ([\d.]+) lbs: (.*?)[!.]`)

// The shinies and old jellyfish can have a space in front and behind them idk why
// [2025-01-11 01:30:41] #omie gofishgame: @ritaaww, You caught a 🫧  HailHelix  🫧! It weighs 2.06 lbs. (30m cooldown after a catch) logs.spanix
// [2023-10-1 21:24:45] #breadworms gofishgame: @derinturitierutz, You caught a 🫧 HailHelix  🫧! It weighs 2.21 lbs. (30m cooldown after a catch) logs.joinuv
// [2023-09-30 22:49:23] #psp1g gofishgame: @6blmue, You caught a 🫧 Jellyfish  🫧! It weighs 19.44 lbs. (30m cooldown after a catch) logs.nadeko
// thats why im matching the fish like this \s*(\S+)\s*
var NormalPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): [@👥]\s?(\w+), You caught a [✨🫧] \s*(\S+)\s* [✨🫧]! It weighs ([\d.]+) lbs[.]`)
var MouthPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): [@👥]\s?(\w+), You caught a [✨🫧] \s*(\S+)\s* [✨🫧]! It weighs ([\d.]+) lbs[.] And![.][.][.] \s*(\S+)\s* \(([\d.]+) lbs\) was in its mouth!`)

var ReleasePattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): [@👥]\s?(\w+), Bye bye \s*(\S+)\s*[!] 🫳🌊 [.][.][.]Huh[?] ✨ Something is (glimmering|sparkling|glittering) in the ocean[.][.][.] 🥍 \s*(\S+)\s* Got it!`)
var JumpedPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): [@👥]\s?(\w+), Huh[?][!] ✨ Something jumped out of the water to snatch your rare candy! [.][.][.]Got it! 🥍 \s*(\S+)\s* ([\d.]+) lbs`)

var BirdPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): @(\w+), Huh[?][!] 🪺 is hatching![.][.][.] It's a [✨🪽🫧] \s*(\S+)\s* [✨🪽🫧]! It weighs ([\d.]+) lbs`)
var SquirrelPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): @(\w+), You toss your 🌰! 🫴 Huh[?][!] A [✨🫧] 🐿️ [✨🫧] chased after it! It went into @(\w+)'s bag!`)

var BagPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): [@👥]\s?(\w+), Your (bag|collection): (.+)`)

// var WinterGift = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): @(\w+), You open it, and[.][.][.] [(](\S+) added to bag![)]`)
// could add them ?

func extractFishDataFromPatterns(textContent string, patterns []*regexp.Regexp) []FishInfo {
	var fishCatches []FishInfo

	for _, pattern := range patterns {
		for _, match := range pattern.FindAllStringSubmatch(textContent, -1) {
			var extractFunc func([]string) FishInfo
			switch pattern {
			case ReleasePattern:
				extractFunc = extractInfoFromReleasePattern
			case NormalPattern:
				extractFunc = extractInfoFromNormalPattern
			case MouthPattern:
				extractFunc = extractInfoFromMouthPattern
			case JumpedPattern:
				extractFunc = extractInfoFromNormalPattern
			case BirdPattern:
				extractFunc = extractInfoFromNormalPattern
			case SquirrelPattern:
				extractFunc = extractInfoFromSquirrelPattern
			case BagPattern:
				extractFunc = extractInfoFromBagPattern
			case TournamentPattern:
				extractFunc = extractInfoFromTData
			}

			fishCatches = append(fishCatches, extractFunc(match))

		}
	}

	return fishCatches
}

func extractInfoFromNormalPattern(match []string) FishInfo {
	dateStr := match[1]
	bot := match[2]
	player := match[3]
	fishType := match[4]
	weight, _ := strconv.ParseFloat(match[5], 64)
	catchtype := "normal"

	if strings.Contains(strings.ToLower(match[0]), "jumped") {
		catchtype = "jumped"
	}

	if strings.Contains(strings.ToLower(match[0]), "hatch") {
		catchtype = "egg"
	}

	date, err := utils.ParseDate(dateStr)
	if err != nil {
		logs.Logs().Fatal().Err(err).
			Str("Player", player).
			Str("Date", dateStr).
			Str("FishType", fishType).
			Msgf("Error parsing date for fish")
	}

	return FishInfo{
		Date:      date,
		Bot:       bot,
		Player:    player,
		FishType:  fishType,
		Weight:    weight,
		CatchType: catchtype,
	}
}

func extractInfoFromMouthPattern(match []string) FishInfo {
	dateStr := match[1]
	bot := match[2]
	player := match[3]
	fishType := match[6]
	weight, _ := strconv.ParseFloat(match[7], 64)
	catchtype := "mouth"

	date, err := utils.ParseDate(dateStr)
	if err != nil {
		logs.Logs().Fatal().Err(err).
			Str("Player", player).
			Str("Date", dateStr).
			Str("FishType", fishType).
			Msgf("Error parsing date for fish")
	}

	return FishInfo{
		Date:      date,
		Bot:       bot,
		Player:    player,
		FishType:  fishType,
		Weight:    weight,
		CatchType: catchtype,
	}
}

func extractInfoFromReleasePattern(match []string) FishInfo {
	dateStr := match[1]
	bot := match[2]
	player := match[3]
	fishType := match[6]
	catchtype := "release"

	weight := 0.0

	date, err := utils.ParseDate(dateStr)
	if err != nil {
		logs.Logs().Fatal().Err(err).
			Str("Player", player).
			Str("Date", dateStr).
			Str("FishType", fishType).
			Msgf("Error parsing date for fish")
	}

	return FishInfo{
		Date:      date,
		Bot:       bot,
		Player:    player,
		FishType:  fishType,
		Weight:    weight,
		CatchType: catchtype,
	}
}

func extractInfoFromSquirrelPattern(match []string) FishInfo {
	dateStr := match[1]
	bot := match[2]
	player := match[4] // Could maybe also store thrower ?
	fishType := "🐿️"
	catchtype := "squirrel"
	weight := 0.0

	date, err := utils.ParseDate(dateStr)
	if err != nil {
		logs.Logs().Fatal().Err(err).
			Str("Player", player).
			Str("Date", dateStr).
			Str("FishType", fishType).
			Msgf("Error parsing date for fish")
	}

	return FishInfo{
		Date:      date,
		Bot:       bot,
		Player:    player,
		FishType:  fishType,
		Weight:    weight,
		CatchType: catchtype,
	}
}

func extractInfoFromBagPattern(match []string) FishInfo {
	dateStr := match[1]
	bot := match[2]
	player := match[3]
	bag := strings.Fields(match[5])
	// split the string into a slice and then later store the bag as an array in the db
	catchtype := "bag"

	date, err := utils.ParseDate(dateStr)
	if err != nil {
		logs.Logs().Fatal().Err(err).
			Str("Player", player).
			Str("Date", dateStr).
			Interface("Bag", bag).
			Msgf("Error parsing date for bag")
	}

	return FishInfo{
		Date:      date,
		Bot:       bot,
		Player:    player,
		Bag:       bag,
		CatchType: catchtype,
	}
}

func extractInfoFromTData(match []string) FishInfo {
	dateStr := match[1]
	bot := match[2]
	player := match[3]

	fishCaught, err := strconv.Atoi(match[5])
	if err != nil {
		logs.Logs().Fatal().Err(err).
			Str("Player", player).
			Str("Date", dateStr).
			Msgf("Error converting string to int for for tournament result")
	}

	totalWeight, err := strconv.ParseFloat(match[7], 64)
	if err != nil {
		logs.Logs().Fatal().Err(err).
			Str("Player", player).
			Str("Date", dateStr).
			Msgf("Error converting string to float64 for for tournament result")
	}

	biggestFishWeight, err := strconv.ParseFloat(match[9], 64)
	if err != nil {
		logs.Logs().Fatal().Err(err).
			Str("Player", player).
			Str("Date", dateStr).
			Msgf("Error converting string to float64 for for tournament result")
	}

	date, err := utils.ParseDate(dateStr)
	if err != nil {
		logs.Logs().Fatal().Err(err).
			Str("Player", player).
			Str("Date", dateStr).
			Msgf("Error parsing date for tournament result")
	}

	return FishInfo{
		Date:                 date,
		Bot:                  bot,
		Player:               player,
		CatchType:            "result",
		Count:                fishCaught,
		TotalWeight:          totalWeight,
		Weight:               biggestFishWeight,
		FishPlacement:        getPlacement(match[6]),
		WeightPlacement:      getPlacement(match[8]),
		BiggestFishPlacement: getPlacement(match[10]),
	}
}

func getPlacement(placeStr string) int {

	switch placeStr {
	case "Victory ✨🏆✨":
		return 1
	case "You were the champion ✨🏆✨":
		return 1 // This is only for one result in the very first tournament week
	case "That's runner-up 🥈":
		return 2
	case "That's third 🥉":
		return 3
	case "You got third place 🥉":
		return 3 // This aswell
	default:
		placeStr = strings.TrimSuffix(placeStr, " place")
		place, _ := strconv.Atoi(regexp.MustCompile(`\D+$`).ReplaceAllString(placeStr, ""))
		return place
	}
}
