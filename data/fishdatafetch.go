package data

import (
	"context"
	"database/sql"
	"fmt"
	"gofish/logs"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/valyala/fasthttp"
)

func FishData(url string, chatName string, fishData []FishInfo, pool *pgxpool.Pool, latestCatchDate time.Time, latestBagDate time.Time) ([]FishInfo, error) {
	const maxRetries = 5
	retryDelay := time.Second

	logs.Logs().Info().
		Str("URL", url).
		Str("Chat", chatName).
		Msg("Fetching fish data")

	for retry := 0; retry < maxRetries; retry++ {

		req := fasthttp.AcquireRequest()
		req.SetRequestURI(url)
		defer fasthttp.ReleaseRequest(req)

		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)

		if err := fasthttp.Do(req, resp); err != nil {
			logs.Logs().Error().Err(err).
				Str("URL", url).
				Str("Chat", chatName).
				Msg("Error fetching fish data from URL")
			time.Sleep(retryDelay)
			retryDelay *= 5
			continue
		}

		if resp.StatusCode() != fasthttp.StatusOK {
			// Since 404 can just mean that noone fished in that month for the very small chats, this doesnt have to count as an error
			if resp.StatusCode() != 404 {
				logs.Logs().Error().
					Str("URL", url).
					Str("Chat", chatName).
					Int("HTTP Code", resp.StatusCode()).
					Msg("Unexpected HTTP status code")
				time.Sleep(retryDelay)
				retryDelay *= 5
				continue
			} else {
				logs.Logs().Warn().
					Str("URL", url).
					Str("Chat", chatName).
					Int("HTTP Code", resp.StatusCode()).
					Msg("No logs for chat")
				return fishData, nil
			}
		}

		textContent := string(resp.Body())

		patterns := []*regexp.Regexp{
			MouthPattern,
			ReleasePattern,
			NormalPattern,
			JumpedPattern,
			BirdPattern,
			SquirrelPattern,
			BagPattern,
		}

		fishCatches := extractInfoFromPatterns(textContent, patterns)

		for _, fishCatch := range fishCatches {
			player := fishCatch.Player
			fishType := fishCatch.Type
			weight := fishCatch.Weight
			date := fishCatch.Date
			bot := fishCatch.Bot
			catchtype := fishCatch.CatchType

			// Skip the two players who cheated here
			if player == "cyancaesar" || player == "hansworthelias" {
				if bot == "supibot" {
					continue
				}
			}

			// This is only for old Supibot logs (Could also change regex to not get the space at the end?)
			if fishType == "SabaPing " || fishType == "HailHelix " || fishType == "Jellyfish " {
				fishType = strings.TrimSpace(fishType)
			}

			// Because Jellyfish used to be a bttv emote
			if fishType == "Jellyfish" {
				fishType = "🪼"
			}

			if catchtype != "bag" {
				if date.After(latestCatchDate) {
					FishData := FishInfo{
						Player:    player,
						Weight:    weight,
						Bot:       bot,
						Date:      date,
						CatchType: catchtype,
						Type:      fishType,
						Chat:      chatName,
						Url:       url,
					}

					fishData = append(fishData, FishData)
				}
			} else {
				if date.After(latestBagDate) {
					FishData := FishInfo{
						Player:    player,
						Bot:       bot,
						Date:      date,
						CatchType: catchtype,
						Type:      fishType,
						Chat:      chatName,
						Url:       url,
					}

					fishData = append(fishData, FishData)
				}
			}
		}

		logs.Logs().Info().
			Str("URL", url).
			Str("Chat", chatName).
			Msg("Finished storing fish")
		return fishData, nil // Return successfully fetched data
	}

	// Log the error and stop the entire program
	logs.Logs().Fatal().
		Str("URL", url).
		Str("Chat", chatName).
		Msg("Reached maximum retries, unable to fetch fish data from URL")
	return nil, nil
}

func getLatestCatchDateFromDatabase(ctx context.Context, pool *pgxpool.Pool, chatName string, tableName string) (time.Time, error) {

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
