package data

import (
	"regexp"
	"strconv"
	"strings"
)

type FishInfo struct {
	Player    string
	PlayerID  int
	Weight    float64
	Bot       string
	Type      string
	TypeName  string
	CatchType string
	Date      string
	Chat      string
	FishId    string
}

// List of all the patterns
var MouthPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+), You caught a [✨🫧] (.*?) [✨🫧]! It weighs ([\d.]+) lbs. And!... (.*?)(?: \(([\d.]+) lbs\) was in its mouth)?!`)
var ReleasePattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+\s?(\w+): [@👥]\s?(\w+), Bye bye (.*?)[!] 🫳🌊 ...Huh[?] ✨ Something is (glimmering|sparkling) in the ocean... [🥍] (.*?) Got`)
var JumpedPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+), Huh[?][!] ✨ Something jumped out of the water to snatch your rare candy! ...Got it! 🥍 (.*?) ([\d.]+) lbs`)
var NormalPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+), You caught a [✨🫧] (.*?) [✨🫧]! It weighs ([\d.]+) lbs`)
var BirdPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): @\s?(\w+), Huh[?][!] 🪺 is hatching!... It's a 🪽 (.*?) 🪽! It weighs ([\d.]+) lbs`)

// Generic function to extract information using multiple patterns
func extractInfoFromPatterns(textContent string, patterns []*regexp.Regexp) []FishInfo {
	var fishCatches []FishInfo

	for _, pattern := range patterns {
		for _, match := range pattern.FindAllStringSubmatch(textContent, -1) {
			// Determine which extraction function to use based on the pattern
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

			// Call the appropriate extraction function
			fishCatches = append(fishCatches, extractFunc(match))
		}
	}

	return fishCatches
}

// Define a function to extract information for the existing pattern
func extractInfoFromNormalPattern(match []string) FishInfo {
	date := match[1]
	bot := match[2]
	player := match[3]
	fishType := match[4]
	fishWeightStr := match[5]
	catchtype := "normal"

	// Check if the match contains the word "jumped"
	if strings.Contains(strings.ToLower(match[0]), "jumped") {
		catchtype = "jumped"
	}

	// Check if the match contains the word "hatch"
	if strings.Contains(strings.ToLower(match[0]), "hatch") {
		catchtype = "egg"
	}

	weight, err := strconv.ParseFloat(fishWeightStr, 64)
	if err != nil {
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

// Define a function to extract information for the existing pattern
func extractInfoFromMouthPattern(match []string) FishInfo {
	date := match[1]
	bot := match[2]
	player := match[3]
	fishType := match[6]
	fishWeightStr := match[7]
	catchtype := "mouth"

	weight, err := strconv.ParseFloat(fishWeightStr, 64)
	if err != nil {
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

// Define a function to extract information for the existing pattern
func extractInfoFromReleasePattern(match []string) FishInfo {
	date := match[1]
	bot := match[2]
	player := match[3]
	fishType := match[6]
	catchtype := "release"

	weight := 0.0

	return FishInfo{
		Date:      date,
		Bot:       bot,
		Player:    player,
		Type:      fishType,
		Weight:    weight,
		CatchType: catchtype,
	}
}

// Define a mapping for equivalent fish types
var equivalentFishTypes = map[string]string{
	"🕷":          "🕷️",
	"🗡":          "🗡️",
	"🕶":          "🕶️",
	"☂":          "☂️",
	"⛸":          "⛸️",
	"🧜♀":         "🧜‍♀️",
	"🧜♀️":        "🧜‍♀️",
	"🧜‍♀":        "🧜‍♀️",
	"🐻‍❄️":       "🐻‍❄",
	"🧞‍♂️":       "🧞‍♂",
	"Jellyfish":  "🪼", // Add a space behind for fish which are/were not supported as emotes
	"Jellyfish ": "🪼",
	"HailHelix ": "🐚",
	"HailHelix":  "🐚",
	"SabaPing ":  "🐟",
	"SabaPing":   "🐟",
}

// EquivalentFishType checks if the current fish type is in the list of equivalent fish types
// and returns the corresponding equivalent fish type if it exists.
func EquivalentFishType(fishType string) string {
	equivalent, ok := equivalentFishTypes[fishType]
	if ok {
		return equivalent
	}
	return fishType // Return the original fish type if no equivalent is found
}
