package data

import (
	"context"
	"database/sql"
	"fmt"
	"gofish/logs"
	"gofish/playerdata"
	"gofish/utils"
	"regexp"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/valyala/fasthttp"
)

func FishData(url string, chatName string, fishData []FishInfo, pool *pgxpool.Pool, mode string, latestCatchDate time.Time) ([]FishInfo, error) {
	const maxRetries = 5
	retryDelay := time.Second // Initial delay before first retry

	logs.Logs().Info().Str("URL", url).Str("Chat", chatName).Msg("Fetching fish data")

	for retry := 0; retry < maxRetries; retry++ {

		startTime := time.Now()

		req := fasthttp.AcquireRequest()
		req.SetRequestURI(url)
		defer fasthttp.ReleaseRequest(req)

		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)

		if err := fasthttp.Do(req, resp); err != nil {
			// Log the error and retry
			logs.Logs().Error().Err(err).Str("URL", url).Str("Chat", chatName).Msgf("Error fetching fish data from URL")
			time.Sleep(retryDelay)
			retryDelay *= 5
			continue
		}

		// Check for HTTP error status codes
		if resp.StatusCode() != fasthttp.StatusOK {
			// Log the error and retry
			// Since 404 can just mean that noone fished in that month for the very small chats, this doesnt have to count as an error
			if resp.StatusCode() != 404 {
				logs.Logs().Error().Str("URL", url).Str("Chat", chatName).Int("Code", resp.StatusCode()).Msg("Unexpected HTTP status code")
				time.Sleep(retryDelay)
				retryDelay *= 5
				continue
			}
		}

		duration := time.Since(startTime)
		logs.Logs().Debug().Dur("Duration", duration).Str("URL", url).Str("Chat", chatName).Msg("Time to load URL")

		textContent := string(resp.Body())

		cheaters := playerdata.ReadCheaters()

		patterns := []*regexp.Regexp{
			MouthPattern,
			ReleasePattern,
			NormalPattern,
			JumpedPattern,
			BirdPattern,
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

		logs.Logs().Info().Str("URL", url).Str("Chat", chatName).Msg("Finished storing fish")
		return fishData, nil // Return successfully fetched data
	}

	// Log the error and stop the entire program
	logs.Logs().Fatal().Str("URL", url).Str("Chat", chatName).Msg("Reached maximum retries, unable to fetch fish data from URL")
	return nil, nil
}

func getLatestCatchDateFromDatabase(ctx context.Context, pool *pgxpool.Pool, chatName string) (time.Time, error) {

	tableName := "fish"

	query := fmt.Sprintf("SELECT MAX(date) FROM %s WHERE chat = $1", tableName)

	var latestCatchDate sql.NullTime
	err := pool.QueryRow(ctx, query, chatName).Scan(&latestCatchDate)
	if err != nil {
		// 42P01 is when the table doesnt exist yet, if you check for the first time
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42P01" {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}

	if latestCatchDate.Valid {
		return latestCatchDate.Time, nil
	} else {
		return time.Time{}, nil
	}
}
