package data

import (
	"context"
	"database/sql"
	"fmt"
	"gofish/logs"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valyala/fasthttp"
)

func GetFishDataFromURL(url string, chatName string, data string, pool *pgxpool.Pool, latestCatchDate time.Time, latestBagDate time.Time, latestTournamentDate time.Time) ([]FishInfo, error) {
	var fishData []FishInfo

	// retry 3 times after 20 seconds
	const maxRetries = 3
	retryDelay := time.Second * 20

	logs.Logs().Info().
		Str("URL", url).
		Str("Chat", chatName).
		Msg("Fetching data")

	for range maxRetries {

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
			continue
		}

		if resp.StatusCode() != fasthttp.StatusOK {
			// Since 404 can just mean that noone fished in that month for the very small chats, this doesnt have to count as an error
			// Just make sure that the chat actually doesnt have logs
			// The chat could also be banned or might have been renamed or might have been removed from the justlog instance
			if resp.StatusCode() != 404 {
				logs.Logs().Error().
					Str("URL", url).
					Str("Chat", chatName).
					Int("HTTP Code", resp.StatusCode()).
					Msg("Unexpected HTTP status code")
				time.Sleep(retryDelay)
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

		// Dont check every pattern depending on "data"
		var patterns []*regexp.Regexp
		switch data {
		case "all":
			patterns = []*regexp.Regexp{
				MouthPattern,
				ReleasePattern,
				NormalPattern,
				JumpedPattern,
				BirdPattern,
				SquirrelPattern,
				BagPattern,
				TournamentPattern,
			}
		case "f":
			patterns = []*regexp.Regexp{
				MouthPattern,
				ReleasePattern,
				NormalPattern,
				JumpedPattern,
				BirdPattern,
				SquirrelPattern,
				BagPattern,
			}
		case "t":
			patterns = []*regexp.Regexp{
				TournamentPattern,
			}
		}

		fishCatches := extractFishDataFromPatterns(textContent, patterns)

		for _, fish := range fishCatches {
			// Update the url and the name of the chat here
			fish.Chat = chatName
			fish.Url = url

			// Skip the two players who cheated here
			if fish.Player == "cyancaesar" || fish.Player == "hansworthelias" {
				if fish.Bot == "supibot" {
					continue
				}
			}

			switch fish.CatchType {
			default:

				if fish.Date.After(latestCatchDate) {
					// Because Jellyfish used to be a bttv emote
					if fish.Type == "Jellyfish" {
						fish.Type = "🪼"
					}

					fishData = append(fishData, fish)
				}
			case "bag":
				if fish.Date.After(latestBagDate) {

					fishData = append(fishData, fish)
				}
			case "result":
				if fish.Date.After(latestTournamentDate) {

					fishData = append(fishData, fish)
				}
			}
		}

		logs.Logs().Info().
			Str("URL", url).
			Str("Chat", chatName).
			Msg("Finished parsing data")

		return fishData, nil
	}

	// dont really need to fatal here, not getting data from an instance isnt a problem
	// fish will just not be ordered by fishid if they get inserted later or are added from a different instance but its ok
	logs.Logs().Error().
		Str("URL", url).
		Str("Chat", chatName).
		Msg("Reached maximum retries, unable to fetch data from URL")
	return fishData, nil
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
