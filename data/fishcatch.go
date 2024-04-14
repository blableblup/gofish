package data

import (
	"fmt"
	"gofish/playerdata"
	"gofish/utils"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

type Record struct {
	Player    string
	Weight    float64
	Bot       string
	Type      string
	CatchType string
	Date      string
	Chat      string
}

// List of all the patterns
var MouthPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+), You caught a [✨🫧] (.*?) [✨🫧]! It weighs ([\d.]+) lbs. And!... (.*?)(?: \(([\d.]+) lbs\) was in its mouth)?!`)
var ReleasePattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+\s?(\w+): [@👥]\s?(\w+), Bye bye (.*?)[!] 🫳🌊 ...Huh[?] ✨ Something is (glimmering|sparkling) in the ocean... [🥍] (.*?) Got`)
var JumpedPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+), Huh[?][!] ✨ Something jumped out of the water to snatch your rare candy! ...Got it! 🥍 (.*?) ([\d.]+) lbs`)
var NormalPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): [@👥]\s?(\w+), You caught a [✨🫧] (.*?) [✨🫧]! It weighs ([\d.]+) lbs`)
var BirdPattern = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{1,2}\s\d{2}:\d{2}:\d{2})\] #\w+ \s?(\w+): @\s?(\w+), Huh[?][!] 🪺 is hatching!... It's a 🪽 (.*?) 🪽! It weighs ([\d.]+) lbs`)

func CatchWeightType(url string, newRecordWeight map[string]Record, newRecordType map[string]Record, Weightlimit float64) (map[string]Record, map[string]Record, error) {
	const maxRetries = 5
	retryDelay := time.Second // Initial delay before first retry

	for retry := 0; retry < maxRetries; retry++ {
		req := fasthttp.AcquireRequest()
		req.SetRequestURI(url)
		defer fasthttp.ReleaseRequest(req)

		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)

		if err := fasthttp.Do(req, resp); err != nil {
			// Log the error and retry
			log.Printf("Error fetching data from URL %s: %v\n", url, err)
			time.Sleep(retryDelay)
			retryDelay *= 5
			continue
		}

		// Check for HTTP error status codes
		if resp.StatusCode() != fasthttp.StatusOK {
			// Log the error and retry
			log.Printf("Unexpected HTTP status code %d for URL: %s\n", resp.StatusCode(), url)
			time.Sleep(retryDelay)
			retryDelay *= 5
			continue
		}

		// Extract text content from the response body
		textContent := string(resp.Body())

		cheaters := playerdata.ReadCheaters()
		renamedChatters := playerdata.ReadRenamedChatters()

		// Define the patterns for fish catches
		patterns := []*regexp.Regexp{
			MouthPattern,
			ReleasePattern,
			NormalPattern,
			JumpedPattern,
			BirdPattern,
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
			catchtype := fishCatch.CatchType

			// Change to the latest name
			newPlayer := renamedChatters[player]
			for newPlayer != "" {
				player = newPlayer
				newPlayer = renamedChatters[player]
			}

			if utils.Contains(cheaters, player) {
				continue // Skip processing for ignored players
			}

			// Update fish type if it has an equivalent
			if equivalent := utils.EquivalentFishType(fishType); equivalent != "" {
				fishType = equivalent
			}

			// Update the record for the biggest fish of the player if weight exceeds Weightlimit
			if weight > newRecordWeight[player].Weight && weight > Weightlimit {
				newRecordWeight[player] = Record{Type: fishType, Weight: weight, Bot: bot, Date: date, CatchType: catchtype}
			}

			// Update the record for the biggest fish for that type of fish
			if weight > newRecordType[fishType].Weight {
				newRecordType[fishType] = Record{Player: player, Weight: weight, Bot: bot, Date: date, CatchType: catchtype}
			}
		}

		fmt.Println("Finished storing weight records for", url)
		return newRecordWeight, newRecordType, nil // Return successfully fetched data
	}

	// Return an error if maximum retries reached
	return nil, nil, fmt.Errorf("reached maximum retries, unable to fetch data from URL: %s", url)
}

// Generic function to extract information using multiple patterns
func extractInfoFromPatterns(textContent string, patterns []*regexp.Regexp) []Record {
	var fishCatches []Record

	for _, pattern := range patterns {
		for _, match := range pattern.FindAllStringSubmatch(textContent, -1) {
			// Determine which extraction function to use based on the pattern
			var extractFunc func([]string) Record
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
func extractInfoFromNormalPattern(match []string) Record {
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

	weight, err := strconv.ParseFloat(fishWeightStr, 64)
	if err != nil {
		// Handle error
	}

	return Record{
		Date:      date,
		Bot:       bot,
		Player:    player,
		Type:      fishType,
		Weight:    weight,
		CatchType: catchtype,
	}
}

// Define a function to extract information for the existing pattern
func extractInfoFromMouthPattern(match []string) Record {
	date := match[1]
	bot := match[2]
	player := match[3]
	fishType := match[6]
	fishWeightStr := match[7]
	catchtype := "mouth"

	weight, err := strconv.ParseFloat(fishWeightStr, 64)
	if err != nil {
	}

	return Record{
		Date:      date,
		Bot:       bot,
		Player:    player,
		Type:      fishType,
		Weight:    weight,
		CatchType: catchtype,
	}
}

// Define a function to extract information for the existing pattern
func extractInfoFromReleasePattern(match []string) Record {
	date := match[1]
	bot := match[2]
	player := match[3]
	fishType := match[6]
	catchtype := "release"

	weight := 0.0

	return Record{
		Date:      date,
		Bot:       bot,
		Player:    player,
		Type:      fishType,
		Weight:    weight,
		CatchType: catchtype,
	}
}

// Define a function to count the amount of fish caught by each player
func CountFishCaught(url string, fishCaught map[string]int) (map[string]int, error) {
	const maxRetries = 5
	retryDelay := time.Second // Initial delay before first retry

	for i := 0; i < maxRetries; i++ {
		req := fasthttp.AcquireRequest()
		req.SetRequestURI(url)
		defer fasthttp.ReleaseRequest(req)

		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)

		if err := fasthttp.Do(req, resp); err != nil {
			// Log the error and retry
			log.Printf("Error fetching data from URL %s: %v\n", url, err)
			time.Sleep(retryDelay)
			retryDelay *= 5
			continue
		}

		// Check for HTTP error status codes
		if resp.StatusCode() != fasthttp.StatusOK {
			// Log the error and retry
			log.Printf("Unexpected HTTP status code %d for URL: %s\n", resp.StatusCode(), url)
			time.Sleep(retryDelay)
			retryDelay *= 5
			continue
		}

		// Extract text content from the response body
		textContent := string(resp.Body())

		cheaters := playerdata.ReadCheaters()
		renamedChatters := playerdata.ReadRenamedChatters()

		// Define the patterns for fish catches
		patterns := []*regexp.Regexp{
			MouthPattern,
			ReleasePattern,
			NormalPattern,
			JumpedPattern,
			BirdPattern,
		}

		// Extract information about fish catches from the text content using multiple patterns
		fishCatches := extractInfoFromPatterns(textContent, patterns)

		// Process extracted information and count the number of fish caught by each player
		for _, fishCatch := range fishCatches {
			player := fishCatch.Player

			// Change to the latest name
			newPlayer := renamedChatters[player]
			for newPlayer != "" {
				player = newPlayer
				newPlayer = renamedChatters[player]
			}

			if utils.Contains(cheaters, player) {
				continue // Skip processing for ignored players
			}

			// Increase the count of fish caught by the player
			fishCaught[player]++
		}

		fmt.Println("Finished counting fish caught for", url)
		return fishCaught, nil // Return successfully fetched data
	}

	// Return an error if maximum retries reached
	return nil, fmt.Errorf("reached maximum retries, unable to fetch data from URL: %s", url)
}
