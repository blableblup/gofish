package data

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"gofish/playerdata"
	"gofish/utils"
	"log"
	"regexp"
	"time"

	"github.com/valyala/fasthttp"
)

func FishData(url string, chatName string, fishData []FishInfo) ([]FishInfo, error) {
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

			// Generate unique FishID
			fishID := generateFishID(player, date, weight)
			chat := chatName

			FishData := FishInfo{
				Player:    player,
				Weight:    weight,
				Bot:       bot,
				Date:      date,
				CatchType: catchtype,
				Type:      fishType,
				Chat:      chat,
				FishId:    fishID,
			}

			// Append the record to the fishData slice
			fishData = append(fishData, FishData)
		}

		fmt.Println("Finished storing fish for", url)
		return fishData, nil // Return successfully fetched data
	}

	// Return an error if maximum retries reached
	return nil, fmt.Errorf("reached maximum retries, unable to fetch data from URL: %s", url)
}

func generateFishID(player, date string, weight float64) string {
	// Concatenate player's name, date, and weight
	concatenated := fmt.Sprintf("%s-%s-%f", player, date, weight)

	// Calculate SHA-256 hash of the concatenated string
	hash := sha256.New()
	hash.Write([]byte(concatenated))
	hashed := hash.Sum(nil)

	// Convert the first 20 bytes of the hash to a random 20-digit number
	fishID := hex.EncodeToString(hashed)[:20]

	return fishID
}
