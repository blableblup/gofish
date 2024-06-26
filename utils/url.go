package utils

import (
	"fmt"
	"gofish/logs"
	"strconv"
	"strings"
	"time"
)

// This creates the urls which get checked in data. By default it returns the url of the current month

func CreateURL(chatName string, numMonths int, monthYear string, config Config) []string {

	now := time.Now()
	var urls []string

	if config.Chat[chatName].LogsHost == "" {
		logs.Logs().Fatal().Str("Chat", chatName).Msg("No logs host specified for chat")
	}

	// Start from the specified month/year or current month/year
	if monthYear != "" {
		parts := strings.Split(monthYear, "/")
		if len(parts) != 2 {
			logs.Logs().Fatal().Str("MonthYear", monthYear).Msg("Invalid month/year format. Please use 'yyyy/mm' format.")
		}
		year, err := strconv.Atoi(parts[0])
		if err != nil {
			logs.Logs().Fatal().Str("MonthYear", monthYear).Msg("Invalid year")
		}
		month, err := strconv.Atoi(parts[1])
		if err != nil || month < 1 || month > 12 {
			logs.Logs().Fatal().Str("MonthYear", monthYear).Msg("Invalid month")
		}
		now = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	}

	// Loop through the specified number of months
	for i := 0; i < numMonths; i++ {
		// Calculate the date for the first day of the current month
		firstOfMonth := time.Date(now.Year(), now.Month()-time.Month(i), 1, 0, 0, 0, 0, time.UTC)

		// Extract the year and month from the first day of the month
		year, month, _ := firstOfMonth.Date()

		// Check if gofish was added to the channel first
		if logsAdded, err := time.Parse("2006/1", config.Chat[chatName].LogsAdded); err == nil {
			if firstOfMonth.Before(logsAdded) {
				logs.Logs().Info().Str("Chat", chatName).Int("Year", year).Int("Month", int(month)).Msg("Breaking because gofish was not added yet in chat")
				break
			}
		} else {
			// Every chat should have this field in the config
			logs.Logs().Fatal().Err(err).Str("Chat", chatName).Msg("Unable to parse LogsAdded for chat")
		}

		// Check if the current month is within September 2023
		if year == 2023 && month == time.September {
			if config.Chat[chatName].LogsHostOld == "" {
				logs.Logs().Fatal().Str("Chat", chatName).Msg("There is no old logs host specified for chat")
			}

			// Use both the old and new logs hosts
			urlOld := fmt.Sprintf("%s%d/%d?", config.Chat[chatName].LogsHostOld, year, int(month))
			urlNew := fmt.Sprintf("%s%d/%d?", config.Chat[chatName].LogsHost, year, int(month))
			urls = append(urls, urlOld, urlNew)
		} else {
			// Check if the current month is before the logs host change
			if year < 2023 || (year == 2023 && month < time.September) {
				// Use the old logs host if it's not empty
				if config.Chat[chatName].LogsHostOld != "" {
					url := fmt.Sprintf("%s%d/%d?", config.Chat[chatName].LogsHostOld, year, int(month))
					urls = append(urls, url)
				} else {
					logs.Logs().Fatal().Str("Chat", chatName).Msg("There is no old logs host specified for chat")
				}
			} else {
				// Use the current logs host
				url := fmt.Sprintf("%s%d/%d?", config.Chat[chatName].LogsHost, year, int(month))
				urls = append(urls, url)
			}
		}
	}

	return urls
}
