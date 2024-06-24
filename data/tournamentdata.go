package data

import (
	"context"
	"fmt"
	"gofish/logs"
	"gofish/playerdata"
	"gofish/utils"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

func GetTournamentData(config utils.Config, pool *pgxpool.Pool, chatNames string, numMonths int, monthYear string, mode string) {

	var wg sync.WaitGroup

	switch chatNames {
	case "all":
		for chatName, chat := range config.Chat {
			if !chat.CheckTData {
				if chatName != "global" && chatName != "default" {
					logs.Logs().Warn().Str("Chat", chatName).Msg("Skipping chat because checktdata is false")
				}
				continue
			}

			wg.Add(1)
			go func(chatName string, chat utils.ChatInfo) {
				defer wg.Done()

				urls := utils.CreateURL(chatName, numMonths, monthYear, config)
				matchingLines, err := fetchMatchingLines(chatName, urls)
				if err != nil {
					logs.Logs().Error().Err(err).Str("Chat", chatName).Msg("Error fetching tournament data")
					return
				}
				processTData(matchingLines, chatName, pool)
			}(chatName, chat)
		}
	case "":
		logs.Logs().Warn().Msgf("Please specify chat names.")
	default:
		specifiedchatNames := strings.Split(chatNames, ",")
		for _, chatName := range specifiedchatNames {
			chat, ok := config.Chat[chatName]
			if !ok {
				logs.Logs().Warn().Str("Chat", chatName).Msg("Chat not found in config")
				continue
			}
			if !chat.CheckTData {
				if chatName != "global" && chatName != "default" {
					logs.Logs().Warn().Str("Chat", chatName).Msg("Skipping chat because checktdata is false")
				}
				continue
			}

			wg.Add(1)
			go func(chatName string, chat utils.ChatInfo) {
				defer wg.Done()

				urls := utils.CreateURL(chatName, numMonths, monthYear, config)
				matchingLines, err := fetchMatchingLines(chatName, urls)
				if err != nil {
					logs.Logs().Error().Err(err).Str("Chat", chatName).Msg("Error fetching tournament data")
					return
				}
				processTData(matchingLines, chatName, pool)
			}(chatName, chat)
		}
	}

	wg.Wait()
}

func fetchMatchingLines(chatName string, urls []string) ([]string, error) {
	matchingLines := make([]string, 0)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errCh := make(chan error, len(urls)) // Channel to receive errors from goroutines

	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			const maxRetries = 5
			retryDelay := time.Second

			logs.Logs().Info().Str("URL", url).Str("Chat", chatName).Msg("Fetching tournament results")

			for retry := 0; retry < maxRetries; retry++ {
				startTime := time.Now()

				response, err := http.Get(url)
				if err != nil {
					logs.Logs().Error().Err(err).Str("URL", url).Str("Chat", chatName).Msg("Error fetching URL")
					time.Sleep(retryDelay)
					retryDelay *= 5
					continue
				}

				if response.StatusCode != http.StatusOK {
					// Since 404 can just mean that noone fished in that month for the very small chats, this doesnt have to count as an error
					if response.StatusCode != 404 {
						logs.Logs().Error().Str("URL", url).Str("Chat", chatName).Int("Code", response.StatusCode).Msg("Unexpected HTTP status code")
						time.Sleep(retryDelay)
						retryDelay *= 5
						continue
					}
				}

				body, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil {
					logs.Logs().Error().Err(err).Str("URL", url).Str("Chat", chatName).Msg("Error reading response body")
					time.Sleep(retryDelay)
					retryDelay *= 5
					continue
				}

				lines := strings.Split(string(body), "\n")
				for _, line := range lines {
					if strings.Contains(line, "The results are in") ||
						strings.Contains(line, "Last week...") {
						mu.Lock()
						matchingLines = append(matchingLines, strings.TrimSpace(line))
						mu.Unlock()
					}
				}

				duration := time.Since(startTime)
				logs.Logs().Debug().Dur("Duration", duration).Str("URL", url).Str("Chat", chatName).Msg("Time to load URL")
				logs.Logs().Info().Str("URL", url).Str("Chat", chatName).Msg("Finished checking for tournament results")
				return
			}

			logs.Logs().Error().Str("URL", url).Str("Chat", chatName).Msg("Reached maximum retries, unable to fetch tournament data from URL")
			errCh <- fmt.Errorf("reached maximum retries for URL %s", url)

		}(url)
	}

	go func() {
		wg.Wait()
		close(errCh)
	}()

	// Collect errors from errCh if any goroutine failed
	for err := range errCh {
		if err != nil {
			logs.Logs().Error().Err(err).Msg("Error fetching tournament data")
			return matchingLines, err
		}
	}

	return matchingLines, nil
}

func processTData(matchingLines []string, chatName string, pool *pgxpool.Pool) {

	// Ensure directory exists
	logFilePath := filepath.Join("data", chatName, "tournamentlogs.txt")
	err := os.MkdirAll(filepath.Dir(logFilePath), 0755)
	if err != nil {
		logs.Logs().Error().Err(err).Str("File", logFilePath).Msg("Error creating directory")
		return
	}

	file, err := os.OpenFile(logFilePath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		logs.Logs().Error().Err(err).Str("File", logFilePath).Msg("Error opening log file")
		return
	}
	defer file.Close()

	newResults, err := insertTDataIntoDB(matchingLines, chatName, pool)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Chat", chatName).Msg("Error inserting tournament data into database")
		return
	}

	if len(newResults) > 0 {

		file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			logs.Logs().Error().Err(err).Str("File", logFilePath).Str("Chat", chatName).Msg("Error opening log file for appending")
			return
		}
		defer file.Close()

		for _, line := range newResults {
			if _, err := file.WriteString(line + "\n"); err != nil {
				logs.Logs().Error().Err(err).Str("File", logFilePath).Str("Chat", chatName).Msg("Error appending to log file")
				return
			}
		}

		logs.Logs().Info().Str("File", logFilePath).Str("Chat", chatName).Msgf("New results appended")

	} else {
		logs.Logs().Info().Str("File", logFilePath).Str("Chat", chatName).Msgf("No new results to append")
	}
}

func insertTDataIntoDB(matchingLines []string, chatName string, pool *pgxpool.Pool) ([]string, error) {

	newResults := make([]string, 0)

	Results, err := TData(chatName, matchingLines, pool)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Chat", chatName).Msg("Error parsing results")
		return newResults, err
	}

	tx, err := pool.Begin(context.Background())
	if err != nil {
		logs.Logs().Error().Err(err).Str("Chat", chatName).Msg("Error starting transaction")
		return newResults, err
	}
	defer tx.Rollback(context.Background())

	sort.SliceStable(Results, func(i, j int) bool {
		return Results[i].Date.Before(Results[j].Date)
	})

	newResultCounts := 0

	for _, result := range Results {

		tableName := "tournaments" + chatName
		if err := utils.EnsureTableExists(pool, tableName); err != nil {
			logs.Logs().Error().Err(err).Str("Table", tableName).Str("Chat", chatName).Msg("Error ensuring table exists")
			return newResults, err
		}

		var count int
		err := tx.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM `+tableName+`
			WHERE (EXTRACT(year FROM date) = EXTRACT(year FROM $1::timestamp)
				   AND EXTRACT(month FROM date) = EXTRACT(month FROM $2::timestamp))
			   OR (EXTRACT(year FROM date) = EXTRACT(year FROM $3::timestamp)
				   AND EXTRACT(month FROM date) = EXTRACT(month FROM $4::timestamp))
			   AND player = $5 AND fishcaught = $6 AND placement1 = $7 AND totalweight = $8 AND placement2 = $9 AND biggestfish = $10 AND placement3 = $11
		`, result.Date, result.Date, result.Date.AddDate(0, -1, 0), result.Date.AddDate(0, -1, 0), result.Player, result.FishCaught, result.FishPlacement, result.TotalWeight, result.WeightPlacement,
			result.BiggestFish, result.BiggestFishPlacement).Scan(&count)
		if err != nil {
			logs.Logs().Error().Err(err).Str("Table", tableName).Str("Chat", chatName).Msg("Error counting existing results")
			return newResults, err
		}
		if count > 0 {
			continue
		}

		// This adds the players checkin result date and chat as their first fish!
		playerID, err := playerdata.GetPlayerID(pool, result.Player, result.Date, result.Chat)
		if err != nil {
			logs.Logs().Error().Err(err).Str("Player", result.Player).Str("Chat", result.Chat).Msg("Error getting player ID")
			return newResults, err
		}

		query := fmt.Sprintf("INSERT INTO %s ( player, playerid, fishcaught, placement1, totalweight, placement2, biggestfish, placement3, date, bot, chat) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)", tableName)
		_, err = tx.Exec(context.Background(), query, result.Player, playerID, result.FishCaught, result.FishPlacement, result.TotalWeight,
			result.WeightPlacement, result.BiggestFish, result.BiggestFishPlacement, result.Date, result.Bot, result.Chat)
		if err != nil {
			logs.Logs().Error().Err(err).Str("Chat", chatName).Str("Query", query).Msg("Error inserting tournament data")
			return newResults, err
		}

		newResultCounts++
		newResults = append(newResults, result.Line)
	}

	if err := tx.Commit(context.Background()); err != nil {
		logs.Logs().Error().Err(err).Str("Chat", chatName).Msg("Error committing transaction")
		return newResults, err
	}

	if newResultCounts > 0 {
		logs.Logs().Info().Int("Count", newResultCounts).Str("Chat", chatName).Msg("New results added into the database for chat")
	} else {
		logs.Logs().Info().Int("Count", newResultCounts).Str("Chat", chatName).Msg("No new results found for chat")
	}

	return newResults, nil
}
