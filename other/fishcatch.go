package other

import (
	"fmt"
	"gofish/lists"
	"regexp"
	"strconv"

	"github.com/valyala/fasthttp"
)

type Record struct {
	Player string
	Weight float64
	Bot    string
	Type   string
	Date   string
}

// List of all the patterns
var mouthPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+), You caught a [✨🫧] (.*?) [✨🫧]! It weighs ([\d.]+) lbs. And!... (.*?)(?: \(([\d.]+) lbs\) was in its mouth)?!`)
var releasePattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+\s?(\w+): [@👥]\s?(\w+), Bye bye (.*?)[!] 🫳🌊 ...Huh[?] ✨ Something is (glimmering|sparkling) in the ocean... [🥍] (.*?) Got`)
var jumpedPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+), Huh[?][!] ✨ Something jumped out of the water to snatch your rare candy! ...Got it! 🥍 (.*?) ([\d.]+) lbs`)
var normalPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+), You caught a [✨🫧] (.*?) [✨🫧]! It weighs ([\d.]+) lbs`)

func CatchWeightType(url string, newRecordWeight map[string]Record, newRecordType map[string]Record, Weightlimit string) (map[string]Record, map[string]Record, error) {

	// Fetch data from the URL
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(url)
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	if err := fasthttp.Do(req, resp); err != nil {
		return nil, nil, err
	}

	// Extract text content from the response body
	textContent := string(resp.Body())

	cheaters := lists.ReadCheaters()
	renamedChatters := lists.ReadRenamedChatters()

	// Define the patterns for fish catches
	patterns := []*regexp.Regexp{
		mouthPattern,
		releasePattern,
		normalPattern,
		jumpedPattern,
	}

	// Extract information about fish catches from the text content using multiple patterns
	fishCatches := extractInfoFromPatterns(textContent, patterns)

	// Process extracted information and update records
	for _, fishCatch := range fishCatches {

		player := fishCatch.Player
		fishType := fishCatch.Type
		weight := fishCatch.Weight
		date := fishCatch.Date
		bot := fishCatch.Bot

		// Skip processing for ignored players
		found := false
		for _, c := range cheaters {
			if c == player {
				found = true
				break
			}
		}
		if found {
			continue // Skip processing for ignored players
		}

		// Change to the latest name
		newPlayer := renamedChatters[player]
		for newPlayer != "" {
			player = newPlayer
			newPlayer = renamedChatters[player]
		}

		// Update fish type if it has an equivalent
		if equivalent := EquivalentFishType(fishType); equivalent != "" {
			fishType = equivalent
		}

		// Convert Weightlimit to float64
		weightLimit, err := strconv.ParseFloat(Weightlimit, 64)
		if err != nil {
			fmt.Println("Error converting Weightlimit to float64:", err)
		}

		// Update the record for the biggest fish of the player if weight exceeds Weightlimit
		if weight > newRecordWeight[player].Weight && weight > weightLimit {
			newRecordWeight[player] = Record{Type: fishType, Weight: weight, Bot: bot, Date: date}
		}

		// Update the record for the biggest fish for that type of fish
		if weight > newRecordType[fishType].Weight {
			newRecordType[fishType] = Record{Player: player, Weight: weight, Bot: bot, Date: date}
		}
	}
	fmt.Println("Finished storing weight records for", url)
	return newRecordWeight, newRecordType, nil
}

// Generic function to extract information using multiple patterns
func extractInfoFromPatterns(textContent string, patterns []*regexp.Regexp) []Record {
	var fishCatches []Record

	for _, pattern := range patterns {
		for _, match := range pattern.FindAllStringSubmatch(textContent, -1) {
			// Determine which extraction function to use based on the pattern
			var extractFunc func([]string) Record
			switch pattern {
			case releasePattern:
				extractFunc = extractInfoFromReleasePattern
			case normalPattern:
				extractFunc = extractInfoFromNormalPattern
			case mouthPattern:
				extractFunc = extractInfoFromMouthPattern
			case jumpedPattern:
				extractFunc = extractInfoFromNormalPattern
			}

			// Call the appropriate extraction function
			fishCatches = append(fishCatches, extractFunc(match))
		}
	}

	return fishCatches
}

// Define a function to extract information for the existing pattern
func extractInfoFromNormalPattern(match []string) Record {
	date := match[1]
	bot := match[2]
	player := match[3]
	fishType := match[4]
	fishWeightStr := match[5]

	weight, err := strconv.ParseFloat(fishWeightStr, 64)
	if err != nil {
	}

	return Record{
		Date:   date,
		Bot:    bot,
		Player: player,
		Type:   fishType,
		Weight: weight,
	}
}

// Define a function to extract information for the existing pattern
func extractInfoFromMouthPattern(match []string) Record {
	date := match[1]
	bot := match[2]
	player := match[3]
	fishType := match[6]
	fishWeightStr := match[7]

	weight, err := strconv.ParseFloat(fishWeightStr, 64)
	if err != nil {
	}

	return Record{
		Date:   date,
		Bot:    bot,
		Player: player,
		Type:   fishType,
		Weight: weight,
	}
}

// Define a function to extract information for the existing pattern
func extractInfoFromReleasePattern(match []string) Record {
	date := match[1]
	bot := match[2]
	player := match[3]
	fishType := match[6]

	weight := 0.0

	return Record{
		Date:   date,
		Bot:    bot,
		Player: player,
		Type:   fishType,
		Weight: weight,
	}
}

// Define a function to count the amount of fish caught by each player
func CountFishCaught(url string, fishCaught map[string]int) (map[string]int, error) {
	// Fetch data from the URL
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(url)
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	if err := fasthttp.Do(req, resp); err != nil {
		return nil, err
	}

	// Extract text content from the response body
	textContent := string(resp.Body())

	cheaters := lists.ReadCheaters()
	renamedChatters := lists.ReadRenamedChatters()

	// Define the patterns for fish catches
	patterns := []*regexp.Regexp{
		mouthPattern,
		releasePattern,
		normalPattern,
		jumpedPattern,
	}

	// Extract information about fish catches from the text content using multiple patterns
	fishCatches := extractInfoFromPatterns(textContent, patterns)

	// Process extracted information and count the number of fish caught by each player
	for _, fishCatch := range fishCatches {
		player := fishCatch.Player

		// Skip processing for ignored players
		found := false
		for _, c := range cheaters {
			if c == player {
				found = true
				break
			}
		}
		if found {
			continue // Skip processing for ignored players
		}

		// Change to the latest name
		newPlayer := renamedChatters[player]
		for newPlayer != "" {
			player = newPlayer
			newPlayer = renamedChatters[player]
		}

		// Increase the count of fish caught by the player
		fishCaught[player]++
	}
	fmt.Println("Finished counting fish caught for", url)
	return fishCaught, nil
}
