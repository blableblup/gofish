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
	Player     string         `json:"player,omitempty"`
	PlayerID   int            `json:"playerid,omitempty"`
	TwitchID   int            `json:"twitchid,omitempty"`
	Weight     float64        `json:"weight,omitempty"`
	Bot        string         `json:"bot,omitempty"`
	Type       string         `json:"type,omitempty"`
	TypeName   string         `json:"typename,omitempty"`
	CatchType  string         `json:"catchtype,omitempty"`
	Date       time.Time      `json:"date,omitempty"`
	Chat       string         `json:"chat,omitempty"`
	Url        string         `json:"url,omitempty"`
	ChatPfp    string         `json:"chatpfp,omitempty"`
	FishId     int            `json:"fishid,omitempty"`
	ChatId     int            `json:"chatid,omitempty"`
	Count      int            `json:"count,omitempty"`
	MaxCount   int            `json:"maxcount,omitempty"`
	ChatCounts map[string]int `json:"chatcounts,omitempty"`
	Verified   bool           `json:"verified,omitempty"`
	Rank       int            `json:"rank,omitempty"`
}

var MouthPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+), You caught a [✨🫧] (.*?) [✨🫧]! It weighs ([\d.]+) lbs. And!... (.*?)(?: \(([\d.]+) lbs\) was in its mouth)?!`)
var ReleasePattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+\s?(\w+): [@👥]\s?(\w+), Bye bye (.*?)[!] 🫳🌊 ...Huh[?] ✨ Something is (glimmering|sparkling) in the ocean... [🥍] (.*?) Got`)
var JumpedPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+), Huh[?][!] ✨ Something jumped out of the water to snatch your rare candy! ...Got it! 🥍 (.*?) ([\d.]+) lbs`)
var NormalPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+), You caught a [✨🫧] (.*?) [✨🫧]! It weighs ([\d.]+) lbs`)
var BirdPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): @\s?(\w+), Huh[?][!] 🪺 is hatching!... It's a [✨🪽🫧] (.*?) [✨🪽🫧]! It weighs ([\d.]+) lbs`)
var SquirrelPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): @\s?(\w+), You toss your 🌰! 🫴 Huh[?][!] A [✨🫧] 🐿️ [✨🫧] chased after it! It went into @\s?(\w+)'s bag!`)

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
			case SquirrelPattern:
				extractFunc = extractInfoFromSquirrelPattern
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
		logs.Logs().Fatal().Err(err).Str("Player", player).Str("Date", dateStr).Str("FishType", fishType).Msgf("Error parsing date for fish")
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
		logs.Logs().Fatal().Err(err).Str("Player", player).Str("Date", dateStr).Str("FishType", fishType).Msgf("Error parsing date for fish")
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
		logs.Logs().Fatal().Err(err).Str("Player", player).Str("Date", dateStr).Str("FishType", fishType).Msgf("Error parsing date for fish")
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

func extractInfoFromSquirrelPattern(match []string) FishInfo {
	dateStr := match[1]
	bot := match[2]
	player := match[4] // Could maybe also store thrower ?
	fishType := "🐿️"
	catchtype := "squirrel"
	weight := 0.0

	date, err := utils.ParseDate(dateStr)
	if err != nil {
		logs.Logs().Fatal().Err(err).Str("Player", player).Str("Date", dateStr).Str("FishType", fishType).Msgf("Error parsing date for fish")
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
