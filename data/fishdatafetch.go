package data

import (
	"context"
	"errors"
	"fmt"
	"gofish/playerdata"
	"gofish/utils"
	"log"
	"regexp"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/valyala/fasthttp"
)

func FishData(url string, chatName string, fishData []FishInfo, pool *pgxpool.Pool, mode string) ([]FishInfo, error) {
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
			log.Printf("Error fetching fish data from URL %s: %v\n", url, err)
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

		textContent := string(resp.Body())

		cheaters := playerdata.ReadCheaters()

		patterns := []*regexp.Regexp{
			MouthPattern,
			ReleasePattern,
			NormalPattern,
			JumpedPattern,
			BirdPattern,
		}

		ctx := context.Background()
		latestCatchDate, err := getLatestCatchDateFromDatabase(ctx, pool, chatName)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				fmt.Printf("No fish data found for chat '%s', setting default latest catch date\n", chatName)
			} else {
				log.Fatalf("Error while retrieving latest catch date: %v", err)
			}
		}

		fishCatches := extractInfoFromPatterns(textContent, patterns)

		// Process extracted information
		for _, fishCatch := range fishCatches {
			player := fishCatch.Player
			fishType := fishCatch.Type
			weight := fishCatch.Weight
			date := fishCatch.Date
			bot := fishCatch.Bot
			catchtype := fishCatch.CatchType

			if utils.Contains(cheaters, player) {
				continue // Skip processing for ignored players
			}

			// Update fish type if it has an equivalent
			if equivalent := EquivalentFishType(fishType); equivalent != "" {
				fishType = equivalent
			}

			chat := chatName

			if mode == "a" {
				// Mode is "a", add every fish to fishData
				FishData := FishInfo{
					Player:    player,
					Weight:    weight,
					Bot:       bot,
					Date:      date,
					CatchType: catchtype,
					Type:      fishType,
					Chat:      chat,
				}

				fishData = append(fishData, FishData)
			} else {
				// Check if the catch date is after the latest saved date
				if date.After(latestCatchDate) {
					FishData := FishInfo{
						Player:    player,
						Weight:    weight,
						Bot:       bot,
						Date:      date,
						CatchType: catchtype,
						Type:      fishType,
						Chat:      chat,
					}

					fishData = append(fishData, FishData)
				}
			}
		}

		fmt.Println("Finished storing fish for", url)
		return fishData, nil // Return successfully fetched data
	}

	// Log the error and stop the entire program
	log.Fatalf("Reached maximum retries, unable to fetch fish data from URL: %s", url)
	return nil, nil
}

func getLatestCatchDateFromDatabase(ctx context.Context, pool *pgxpool.Pool, chatName string) (time.Time, error) {

	query := "SELECT MAX(date) FROM fish WHERE chat = $1"

	var latestCatchDate time.Time
	err := pool.QueryRow(ctx, query, chatName).Scan(&latestCatchDate)
	if err != nil {
		return time.Time{}, err // Return zero time and error if there are no fish found for that chat
	}

	return latestCatchDate, nil
}
