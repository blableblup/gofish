package data

import (
	"context"
	"database/sql"
	"gofish/logs"
	"gofish/playerdata"
	"gofish/utils"
	"regexp"
	"time"

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
			logs.Logs().Error().Err(err).Msgf("Error fetching fish data from URL: %s", url)
			time.Sleep(retryDelay)
			retryDelay *= 5
			continue
		}

		// Check for HTTP error status codes
		if resp.StatusCode() != fasthttp.StatusOK {
			// Log the error and retry
			logs.Logs().Error().Msgf("Unexpected HTTP status code %d for URL: %s", resp.StatusCode(), url)
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

		var latestCatchDate time.Time

		if mode == "a" {
			latestCatchDate = time.Time{}
		} else {
			var err error
			latestCatchDate, err = getLatestCatchDateFromDatabase(ctx, pool, chatName)
			if err != nil {
				logs.Logs().Fatal().Err(err).Msgf("Error while retrieving latest catch date for chat '%s'", chatName)
			}
		}

		fishCatches := extractInfoFromPatterns(textContent, patterns)

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

		logs.Logs().Info().Msgf("Finished storing fish for %s", url)
		return fishData, nil // Return successfully fetched data
	}

	// Log the error and stop the entire program
	logs.Logs().Fatal().Msgf("Reached maximum retries, unable to fetch fish data from URL: %s", url)
	return nil, nil
}

func getLatestCatchDateFromDatabase(ctx context.Context, pool *pgxpool.Pool, chatName string) (time.Time, error) {

	query := "SELECT MAX(date) FROM fish WHERE chat = $1"

	var latestCatchDate sql.NullTime
	err := pool.QueryRow(ctx, query, chatName).Scan(&latestCatchDate)
	if err != nil {
		return time.Time{}, err
	}

	if latestCatchDate.Valid {
		return latestCatchDate.Time, nil
	} else {
		return time.Time{}, nil
	}
}
