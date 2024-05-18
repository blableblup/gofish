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
	Player     string
	PlayerID   int
	Weight     float64
	Bot        string
	Type       string
	TypeName   string
	CatchType  string
	Date       time.Time
	Chat       string
	FishId     int
	ChatId     int
	Count      int
	MaxCount   int
	ChatCounts map[string]int
}

var MouthPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+), You caught a [✨🫧] (.*?) [✨🫧]! It weighs ([\d.]+) lbs. And!... (.*?)(?: \(([\d.]+) lbs\) was in its mouth)?!`)
var ReleasePattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+\s?(\w+): [@👥]\s?(\w+), Bye bye (.*?)[!] 🫳🌊 ...Huh[?] ✨ Something is (glimmering|sparkling) in the ocean... [🥍] (.*?) Got`)
var JumpedPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+), Huh[?][!] ✨ Something jumped out of the water to snatch your rare candy! ...Got it! 🥍 (.*?) ([\d.]+) lbs`)
var NormalPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+), You caught a [✨🫧] (.*?) [✨🫧]! It weighs ([\d.]+) lbs`)
var BirdPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): @\s?(\w+), Huh[?][!] 🪺 is hatching!... It's a 🪽 (.*?) 🪽! It weighs ([\d.]+) lbs`)

func extractInfoFromPatterns(textContent string, patterns []*regexp.Regexp) []FishInfo {
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
		logs.Logs().Error().Err(err).Msgf("Error parsing date for fish '%s' caught by '%s' at '%s'", fishType, player, dateStr)
	}

	return FishInfo{
		Date:      date,
		Bot:       bot,
		Player:    player,
		Type:      fishType,
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
		logs.Logs().Error().Err(err).Msgf("Error parsing date for fish '%s' caught by '%s' at '%s'", fishType, player, dateStr)
	}

	return FishInfo{
		Date:      date,
		Bot:       bot,
		Player:    player,
		Type:      fishType,
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
		logs.Logs().Error().Err(err).Msgf("Error parsing date for fish '%s' caught by '%s' at '%s'", fishType, player, dateStr)
	}

	return FishInfo{
		Date:      date,
		Bot:       bot,
		Player:    player,
		Type:      fishType,
		Weight:    weight,
		CatchType: catchtype,
	}
}

var equivalentFishTypes = map[string]string{
	"Jellyfish":  "🪼",
	"Jellyfish ": "🪼",
	"HailHelix ": "HailHelix",
	"SabaPing ":  "SabaPing",
}

// This is now mainly used to get rid of the trailing space behind some fish which are not emojis
// I dont know why HailHelix and SabaPing had a space behind
func EquivalentFishType(fishType string) string {
	equivalent, ok := equivalentFishTypes[fishType]
	if ok {
		return equivalent
	}
	return fishType // Return the original fish type if no equivalent is found
}
