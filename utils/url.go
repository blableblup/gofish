package utils

import (
	"fmt"
	"gofish/logs"
	"os"
	"strconv"
	"strings"
	"time"
)

// One problem here: Whenever it is past midnight for me, it checks the previous month instead of the current one ?
// This creates the urls which get checked in data. By default it returns the url of the current month

func CreateURL(chatName string, numMonths int, monthYear string, config Config) []string {

	now := time.Now()
	var urls []string
	var err error

	// Start from the specified month/year or current month/year
	if monthYear != "" {
		parts := strings.Split(monthYear, "/")
		if len(parts) != 2 {
			logs.Logs().Error().Err(err).Msg("Invalid month/year format. Please use 'yyyy/mm' format.")
			os.Exit(1)
		}
		year, err := strconv.Atoi(parts[0])
		if err != nil {
			logs.Logs().Error().Err(err).Msg("Invalid year")
			os.Exit(1)
		}
		month, err := strconv.Atoi(parts[1])
		if err != nil || month < 1 || month > 12 {
			logs.Logs().Error().Err(err).Msg("Invalid month")
			os.Exit(1)
		}
		now = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	}

	// Loop through the specified number of months
	for i := 0; i < numMonths; i++ {
		// Calculate the date for the first day of the current month
		firstOfMonth := now.AddDate(0, -i, -now.Day()+1).UTC().Truncate(24 * time.Hour)

		// Extract the year and month from the first day of the month
		year, month, _ := firstOfMonth.Date()

		// Check if gofish was added to the channel first
		if logsAdded, err := time.Parse("2006/1", config.Chat[chatName].LogsAdded); err == nil {
			if firstOfMonth.Before(logsAdded) {
				logs.Logs().Info().Msgf("Breaking at %d/%d because gofish was not added yet in chat '%s'", year, month, chatName)
				break
			}
		} else {
			logs.Logs().Error().Err(err).Msg("Error parsing LogsAdded")
		}

		// Check if the current month is within September 2023
		if year == 2023 && month == time.September && config.Chat[chatName].LogsHostOld != "" {
			// Use both the old and new logs hosts
			urlOld := fmt.Sprintf("%s%d/%d?", config.Chat[chatName].LogsHostOld, year, int(month))
			urlNew := fmt.Sprintf("%s%d/%d?", config.Chat[chatName].LogsHost, year, int(month))
			logs.Logs().Info().Msgf("Fetching data from supibot: %s", urlOld)
			logs.Logs().Info().Msgf("Fetching data from gofishgame: %s", urlNew)
			urls = append(urls, urlOld, urlNew)
		} else {
			// Check if the current month is before the logs host change
			if year < 2023 || (year == 2023 && month < time.September) {
				// Use the old logs host if it's not empty
				if config.Chat[chatName].LogsHostOld != "" {
					url := fmt.Sprintf("%s%d/%d?", config.Chat[chatName].LogsHostOld, year, int(month))
					logs.Logs().Info().Msgf("Fetching data from supibot: %s", url)
					urls = append(urls, url)
				} else {
					logs.Logs().Warn().Msg("There is no old logs host specified. Skipping...")
				}
			} else {
				// Use the current logs host
				url := fmt.Sprintf("%s%d/%d?", config.Chat[chatName].LogsHost, year, int(month))
				logs.Logs().Info().Msgf("Fetching data from gofishgame: %s", url)
				urls = append(urls, url)
			}
		}
	}

	return urls
}
