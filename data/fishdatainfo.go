package data

import (
	"gofish/logs"
	"gofish/utils"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
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

type FishCatch struct {
	Pattern              *regexp.Regexp
	Type                 string
	ExtractFunc          func([]string) FishInfo
	ExtractFuncSlice     func([]string) []FishInfo
	ExtractFuncSliceBool func([]string) ([]FishInfo, bool)
}

var TournamentPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): [@👥]\s?(\w+), (📣 The results are in!|Last week[.][.][.]) You caught 🪣 (\d+) fish: (.*?)[!.] Together they weighed .*? ([\d.]+) lbs: (.*?)[!.] Your biggest catch weighed .*? ([\d.]+) lbs: (.*?)[!.]`)

// The shinies and old jellyfish can have a space in front and behind them idk why
// [2025-01-11 01:30:41] #omie gofishgame: @ritaaww, You caught a 🫧  HailHelix  🫧! It weighs 2.06 lbs. (30m cooldown after a catch) logs.spanix
// [2023-10-1 21:24:45] #breadworms gofishgame: @derinturitierutz, You caught a 🫧 HailHelix  🫧! It weighs 2.21 lbs. (30m cooldown after a catch) logs.joinuv
// [2023-09-30 22:49:23] #psp1g gofishgame: @6blmue, You caught a 🫧 Jellyfish  🫧! It weighs 19.44 lbs. (30m cooldown after a catch) logs.nadeko
// thats why im matching the fish like this \s*(\S+)\s*
var NormalPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): [@👥]\s?(\w+), You caught a [✨🫧] \s*(\S+)\s* [✨🫧]! It weighs ([\d.]+) lbs[.]`)
var MouthPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): [@👥]\s?(\w+), You caught a [✨🫧] \s*(\S+)\s* [✨🫧]! It weighs ([\d.]+) lbs[.] And![.][.][.] \s*(\S+)\s* \(([\d.]+) lbs\) was in its mouth!`)

var ReleasePattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): [@👥]\s?(\w+), Bye bye \s*(\S+)\s*[!] (🫳🌊|🫴🌅) [.][.][.]Huh[?] ✨ Something is (glimmering|sparkling|glittering) in the ocean[.][.][.] 🥍 \s*(\S+)\s* Got it!`)
var ReleasePatternPumpkin = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): @(\w+), Bye bye 🎃[!] 🫳🌊 [.][.][.]Huh[?] There was a 🕯️ inside of its hollow interior!`)

var JumpedPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): [@👥]\s?(\w+), Huh[?][!] ✨ Something jumped out of the water to snatch your rare candy! [.][.][.]Got it! 🥍 \s*(\S+)\s* ([\d.]+) lbs`)

var BirdPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): @(\w+), Huh[?][!] 🪺 is hatching![.][.][.] It's a [✨🪽🫧] \s*(\S+)\s* [✨🪽🫧]! It weighs ([\d.]+) lbs`)
var SquirrelPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): @(\w+), You toss your 🌰! 🫴 Huh[?][!] A [✨🫧] 🐿️ [✨🫧] chased after it! It went into @(\w+)'s bag!`)

// the one which shows weight was the original one and bread changed it
var SonnyThrowWeight = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): @(\w+), 🙆 "Hey kid, catch!" You got a [✨🫧] \s*(\S+)\s* [✨🫧]! It weighs ([\d.]+) lbs`)
var SonnyThrow = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): @(\w+), Huh[?] 🙆 "Hey kid, catch!" 🤲 He gave you a \s*(\S+)\s*! Awesome`)

// winter event stuff
var WinterPresent2024 = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): @(\w+), Huh[?] This was addressed to you[.] [(]🎁 added to bag![)]`)
var WinterPresent = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): @(\w+), Huh[?] You find a 🎁 addressed to you[.] You wonder who it's from[.]`)
var WinterPresentOpening2024 = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): @(\w+), You open it, and[.][.][.] [(](\S+) added to bag![)]`)
var BellGift = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): @(\w+), 🎅 Heya there! Take this and play with me, (won't ya[?]|wontcha[?]) [(]🔔 added to bag![)]`)
var BellGift2025 = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): @(\w+), Huh[?] You pick up a 🔔 that was lying around[.] Who's that running away[?] 🏃‍➡️`)

var BagPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): [@👥]\s?(\w+), Your (bag|collection): (.+)`)

var AmbientPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ (\w+): [@👥]\s?(\w+), (.+) [(]30s cooldown[)]`)

func allTheCatchPatterns() map[string]FishCatch {

	catches := map[string]FishCatch{
		"ambient": {Pattern: AmbientPattern, Type: "ambient", ExtractFuncSliceBool: extractInfoFromAmbientPattern},

		"normal":               {Pattern: NormalPattern, Type: "fish", ExtractFunc: extractInfoFromNormalPattern},
		"mouth":                {Pattern: MouthPattern, Type: "fish", ExtractFunc: extractInfoFromMouthPattern},
		"release":              {Pattern: ReleasePattern, Type: "fish", ExtractFunc: extractInfoFromReleasePattern},
		"releasepumpkin":       {Pattern: ReleasePatternPumpkin, Type: "fish", ExtractFunc: extractInfoFromReleasePattern},
		"jumped":               {Pattern: JumpedPattern, Type: "fish", ExtractFunc: extractInfoFromNormalPattern},
		"bird":                 {Pattern: BirdPattern, Type: "fish", ExtractFunc: extractInfoFromNormalPattern},
		"squirrel":             {Pattern: SquirrelPattern, Type: "fish", ExtractFunc: extractInfoFromSquirrelPattern},
		"sonnythrow":           {Pattern: SonnyThrow, Type: "fish", ExtractFunc: extractInfoFromNormalPattern},
		"sonnythrowweight":     {Pattern: SonnyThrowWeight, Type: "fish", ExtractFunc: extractInfoFromNormalPattern},
		"winterpresentopening": {Pattern: WinterPresentOpening2024, Type: "fish", ExtractFuncSlice: extractInfoFromWinterGift},
		"winterpresent":        {Pattern: WinterPresent, Type: "fish", ExtractFunc: extractInfoFromReleasePattern},
		"winterpresent2024":    {Pattern: WinterPresent2024, Type: "fish", ExtractFunc: extractInfoFromReleasePattern},
		"bellgift":             {Pattern: BellGift, Type: "fish", ExtractFunc: extractInfoFromReleasePattern},
		"bellgift2025":         {Pattern: BellGift2025, Type: "fish", ExtractFunc: extractInfoFromReleasePattern},

		"bag": {Pattern: BagPattern, Type: "bag", ExtractFunc: extractInfoFromBagPattern},

		"tournament": {Pattern: TournamentPattern, Type: "tourney", ExtractFunc: extractInfoFromTData},
	}

	return catches
}

func returnTheCatchPatterns(selectedCatches string) []FishCatch {
	var catches []FishCatch

	allCatches := allTheCatchPatterns()

	switch selectedCatches {
	case "all":

		for _, catch := range allCatches {
			// temporary
			if catch.Type != "ambient" {
				catches = append(catches, catch)
			}
		}

	case "fish":

		for _, catch := range allCatches {
			if catch.Type == "fish" {
				catches = append(catches, catch)
			}
		}

	case "tourney":

		for _, catch := range allCatches {
			if catch.Type == "tourney" {
				catches = append(catches, catch)
			}
		}

	default:

		catchesSplit := strings.Split(selectedCatches, ",")

		// to only check some catches
		for _, catch := range catchesSplit {

			catchh, ja := allCatches[catch]

			if ja {
				catches = append(catches, catchh)
			} else {
				logs.Logs().Fatal().
					Str("Catch", catch).
					Msg("idk what catch this is :(")
			}
		}
	}

	return catches
}

func extractFishDataFromPatterns(textContent string, catches []FishCatch) []FishInfo {
	var fishys []FishInfo

	for _, catch := range catches {

		for _, match := range catch.Pattern.FindAllStringSubmatch(textContent, -1) {

			switch catch.Pattern {
			case WinterPresentOpening2024:
				fishys = append(fishys, catch.ExtractFuncSlice(match)...)

			case AmbientPattern:
				ambiences, yes := catch.ExtractFuncSliceBool(match)

				if yes {
					fishys = append(fishys, ambiences...)
				}

			default:
				fishys = append(fishys, catch.ExtractFunc(match))
			}
		}

	}

	return fishys
}

func extractInfoFromNormalPattern(match []string) FishInfo {
	dateStr := match[1]
	bot := match[2]
	player := match[3]
	fishType := match[4]
	catchtype := "normal"
	weight := 0.0

	if strings.Contains(match[0], "...Got it! 🥍") {
		catchtype = "jumped"
	}

	if strings.Contains(match[0], "🪺 is hatching!...") {
		catchtype = "egg"
	}

	// only parse the weight if the catch is NOT sonny throw catch
	// sonny catches werent supposed to have weight, but the original catches showed it
	// so the weight of all original catches is 0 lbs
	if strings.Contains(match[0], "🙆") {
		catchtype = "sonnythrow"
	} else {
		weight, _ = strconv.ParseFloat(match[5], 64)
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

	var fishType, catchtype string

	switch {
	case strings.Contains(match[0], "There was a 🕯️ inside of its hollow interior!"):
		fishType = "🕯️"
		catchtype = "releasepumpkin"

	case strings.Contains(match[0], "🎅 Heya there!"):
		fishType = "🔔"
		catchtype = "giftbell"

	case strings.Contains(match[0], "You pick up a 🔔"):
		fishType = "🔔"
		catchtype = "giftbell"

	case strings.Contains(match[0], "This was addressed to you"):
		fishType = "🎁"
		catchtype = "giftpresent"

	case strings.Contains(match[0], "You find a 🎁 addressed to you"):
		fishType = "🎁"
		catchtype = "giftpresent"

	default:
		fishType = match[7]
		catchtype = "release"
	}

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

func extractInfoFromWinterGift(match []string) []FishInfo {
	dateStr := match[1]
	bot := match[2]
	player := match[3]
	catchtype := "giftwinter"

	date, err := utils.ParseDate(dateStr)
	if err != nil {
		logs.Logs().Fatal().Err(err).
			Str("Player", player).
			Str("Date", dateStr).
			Msgf("Error parsing date for fish")
	}

	stuff := strings.Split(match[4], "")

	var gifts []FishInfo

	for _, gift := range stuff {

		// because the ✉️ and 🕯️ is two things
		// and gets split idk it is U+2709 U+FE0F

		var skipthingy bool

		for _, runeidk := range gift {
			// this is to skip the U+FE0F thing
			if unicode.IsMark(runeidk) {
				skipthingy = true
			}
		}

		if skipthingy {
			continue
		}

		if gift == "✉" {
			gift = "✉️"
		}

		if gift == "🕯" {
			gift = "🕯️"
		}

		fish := FishInfo{
			Date:      date,
			Bot:       bot,
			Player:    player,
			FishType:  gift,
			Weight:    0.0,
			CatchType: catchtype,
		}

		gifts = append(gifts, fish)
	}

	return gifts

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

func extractInfoFromAmbientPattern(match []string) ([]FishInfo, bool) {
	dateStr := match[1]
	bot := match[2]
	player := match[3]

	date, err := utils.ParseDate(dateStr)
	if err != nil {
		logs.Logs().Fatal().Err(err).
			Str("Player", player).
			Str("Date", dateStr).
			Msgf("Error parsing date for ambient pattern")
	}

	// to skip the other fishing game in supibot logs
	if strings.Contains(match[0], "No luck...") {
		return []FishInfo{}, false
	}

	var ambiences []FishInfo

	var ambience string

	// check if there are other fishers in same log message
	if strings.Contains(match[4], "@") {

		splitThing := strings.Split(match[4], ",")

		// the ambient message is always the last thing
		for i, thing := range splitThing {

			if i+1 == len(splitThing) {
				ambience = strings.TrimPrefix(thing, " ")
			}
		}

		// add the first @ed player
		ambiences = append(ambiences, FishInfo{
			Date:      date,
			Bot:       bot,
			Player:    player,
			FishType:  ambience,
			CatchType: "ambient",
		})

		var playerMatch = regexp.MustCompile(`@(\w+),`)

		players := playerMatch.FindAllStringSubmatch(match[4], -1)

		// and then the other ones
		for _, moreplayer := range players {

			ambiences = append(ambiences, FishInfo{
				Date:      date,
				Bot:       bot,
				Player:    moreplayer[1],
				FishType:  ambience,
				CatchType: "ambient",
			})
		}

	} else {

		ambience = match[4]

		ambiences = append(ambiences, FishInfo{
			Date:      date,
			Bot:       bot,
			Player:    player,
			FishType:  ambience,
			CatchType: "ambient",
		})
	}

	return ambiences, true
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
