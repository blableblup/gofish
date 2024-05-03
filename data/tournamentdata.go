package data

import (
	"bufio"
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

	"github.com/jackc/pgx/v4/pgxpool"
)

func GetTournamentData(config utils.Config, pool *pgxpool.Pool, chatNames string, numMonths int, monthYear string, mode string) {

	switch chatNames {
	case "all":
		for chatName, chat := range config.Chat {
			if !chat.CheckEnabled {
				if chatName != "global" && chatName != "default" {
					logs.Logs().Info().Msgf("Skipping chat '%s' because check_enabled is false", chatName)
				}
				continue
			}

			logs.Logs().Info().Msgf("Checking tournament results for chat '%s'", chatName)
			urls := utils.CreateURL(chatName, numMonths, monthYear)
			fetchMatchingLines(chatName, pool, urls, mode)
		}
	case "":
		logs.Logs().Warn().Msgf("Please specify chat names.")
	default:
		specifiedchatNames := strings.Split(chatNames, ",")
		for _, chatName := range specifiedchatNames {
			chat, ok := config.Chat[chatName]
			if !ok {
				logs.Logs().Warn().Msgf("Chat '%s' not found in config", chatName)
				continue
			}
			if !chat.CheckEnabled {
				if chatName != "global" && chatName != "default" {
					logs.Logs().Info().Msgf("Skipping chat '%s' because check_enabled is false", chatName)
				}
				continue
			}

			logs.Logs().Info().Msgf("Checking tournament results for chat '%s'", chatName)
			urls := utils.CreateURL(chatName, numMonths, monthYear)
			fetchMatchingLines(chatName, pool, urls, mode)
		}
	}
}

func fetchMatchingLines(chatName string, pool *pgxpool.Pool, urls []string, mode string) {

	// Fetch matching lines from each URL
	matchingLines := make([]string, 0)
	if mode != "insertall" {
		for _, url := range urls {
			response, err := http.Get(url)
			if err != nil {
				logs.Logs().Fatal().Err(err).Msg("Error fetching URL")
				continue
			}

			if response.StatusCode != http.StatusOK {
				logs.Logs().Fatal().Msgf("Unexpected HTTP status code %d for URL: %s", response.StatusCode, url)
				response.Body.Close()
				continue
			}

			defer response.Body.Close()

			body, err := io.ReadAll(response.Body)
			if err != nil {
				logs.Logs().Fatal().Err(err).Msg("Error reading response body")
				continue
			}

			lines := strings.Split(string(body), "\n")
			for _, line := range lines {
				if strings.Contains(line, "The results are in") ||
					strings.Contains(line, "The results for last week are in") ||
					strings.Contains(line, "Last week...") {
					matchingLines = append(matchingLines, strings.TrimSpace(line))
				}
			}
			logs.Logs().Info().Msgf("Finished checking for matching lines in %s", url)
		}
	}

	// Ensure directory exists
	logFilePath := filepath.Join("data", chatName, "tournamentlogs.txt")
	err := os.MkdirAll(filepath.Dir(logFilePath), 0755)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error creating directory")
		return
	}

	file, err := os.OpenFile(logFilePath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error opening log file")
		return
	}
	defer file.Close()

	existingLines := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "You caught") {
			parts := strings.SplitN(line, "You caught", 2)
			if mode == "insertall" {
				existingLines[line] = struct{}{}
			} else if len(parts) > 1 {
				existingLines[strings.TrimSpace(parts[1])] = struct{}{}
			}
		}
	}

	newResults := make([]string, 0)

	if mode == "insertall" {
		for line := range existingLines {
			newResults = append(newResults, line)
		}
	} else {
		for _, line := range matchingLines {
			if strings.Contains(line, "You caught") {
				parts := strings.SplitN(line, "You caught", 2)
				if len(parts) > 1 {
					newLine := strings.TrimSpace(parts[1])
					if _, exists := existingLines[newLine]; !exists {
						newResults = append(newResults, line)
						existingLines[newLine] = struct{}{} // Update existing lines with new ones
					}
				}
			}
		}
	}

	if len(newResults) > 0 {

		if err := insertTDataIntoDB(newResults, chatName, mode, pool); err != nil {
			logs.Logs().Error().Err(err).Msg("Error inserting tournament data into database")
			return
		}

		if mode == "insertall" {
			logs.Logs().Info().Msg("Returning because program is in mode 'insertall'")
			return
		}

		file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			logs.Logs().Error().Err(err).Msg("Error opening log file for appending")
			return
		}
		defer file.Close()

		for _, line := range newResults {
			if _, err := file.WriteString(line + "\n"); err != nil {
				logs.Logs().Error().Err(err).Msg("Error appending to log file")
				return
			}
		}
		logs.Logs().Info().Msgf("New results appended to %s", logFilePath)
	} else {
		logs.Logs().Info().Msgf("No new results to append to %s", logFilePath)
	}
}

func insertTDataIntoDB(newResults []string, chatName string, mode string, pool *pgxpool.Pool) error {

	Results, err := TData(chatName, newResults, pool)
	if err != nil {
		return err
	}

	tx, err := pool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())

	sort.SliceStable(Results, func(i, j int) bool {
		return Results[i].Date.Before(Results[j].Date)
	})

	newResultCounts := 0

	for _, result := range Results {

		tableName := "tournaments" + chatName
		if err := utils.EnsureTableExists(pool, tableName); err != nil {
			return err
		}

		if mode == "insertall" {
			var count int
			err := tx.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM `+tableName+`
			WHERE EXTRACT(year FROM date) = EXTRACT(year FROM $1::timestamp)
			AND EXTRACT(month FROM date) = EXTRACT(month FROM $2::timestamp)
			AND player = $3 AND fishcaught = $4 AND placement1 = $5 AND totalweight = $6 AND placement2 = $7 AND biggestfish = $8 AND placement3 = $9
		`, result.Date, result.Date, result.Player, result.FishCaught, result.FishPlacement, result.TotalWeight, result.WeightPlacement,
				result.BiggestFish, result.BiggestFishPlacement).Scan(&count)
			if err != nil {
				return err
			}
			if count > 0 {
				continue
			}
		}

		playerID, err := playerdata.GetPlayerID(pool, result.Player, result.Date, result.Chat)
		if err != nil {
			return err
		}

		query := fmt.Sprintf("INSERT INTO %s ( player, playerid, fishcaught, placement1, totalweight, placement2, biggestfish, placement3, date, bot, chat) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)", tableName)
		_, err = tx.Exec(context.Background(), query, result.Player, playerID, result.FishCaught, result.FishPlacement, result.TotalWeight,
			result.WeightPlacement, result.BiggestFish, result.BiggestFishPlacement, result.Date, result.Bot, result.Chat)
		if err != nil {
			return err
		}

		newResultCounts++
	}

	logs.Logs().Info().Msgf("Successfully inserted %d new results into the database for chat '%s'", newResultCounts, chatName)

	if err := tx.Commit(context.Background()); err != nil {
		return err
	}

	return nil
}
