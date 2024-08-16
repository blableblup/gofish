package utils

import (
	"fmt"
	"gofish/logs"
	"strconv"
	"strings"
	"time"
)

// This creates the urls which get checked in data. By default it returns the url of the current month
// To do: Add a way to check different justlog instances for older data / when the current one is unavailable ?

func CreateURL(chatName string, numMonths int, monthYear string, config Config) []string {

	now := time.Now()
	var urls []string

	if config.Chat[chatName].LogsHost == "" {
		logs.Logs().Fatal().
			Str("Chat", chatName).
			Msg("No logs host specified for chat")
	}

	// Start from the specified month/year or current month/year
	if monthYear != "" {
		parts := strings.Split(monthYear, "/")
		if len(parts) != 2 {
			logs.Logs().Fatal().
				Str("MonthYear", monthYear).
				Msg("Invalid month/year format. Please use 'yyyy/mm' format.")
		}
		year, err := strconv.Atoi(parts[0])
		if err != nil {
			logs.Logs().Fatal().
				Str("MonthYear", monthYear).
				Msg("Invalid year")
		}
		month, err := strconv.Atoi(parts[1])
		if err != nil || month < 1 || month > 12 {
			logs.Logs().Fatal().
				Str("MonthYear", monthYear).
				Msg("Invalid month")
		}
		now = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	}

	// Loop through the specified number of months
	for i := 0; i < numMonths; i++ {
		// Calculate the date for the first day of the current month
		firstOfMonth := time.Date(now.Year(), now.Month()-time.Month(i), 1, 0, 0, 0, 0, time.UTC)

		// Extract the year and month from the first day of the month
		year, month, _ := firstOfMonth.Date()

		// Check if justlog was added to the channel first
		if logsAdded, err := time.Parse("2006/1", config.Chat[chatName].LogsAdded); err == nil {
			if firstOfMonth.Before(logsAdded) {
				logs.Logs().Info().
					Str("Chat", chatName).
					Int("Year", year).
					Int("Month", int(month)).
					Msg("Breaking because justlog was not added yet in chat")
				break
			}
		} else {
			// Every chat should have this field in the config
			logs.Logs().Fatal().Err(err).
				Str("Chat", chatName).
				Msg("Unable to parse LogsAdded for chat")
		}

		// Check if the current month is within September 2023
		if year == 2023 && month == time.September {

			// Check supi and gofishgame
			urlSupi := fmt.Sprintf("%s/channel/%s/user/supibot/%d/%d?", config.Chat[chatName].LogsHost, chatName, year, int(month))
			urlGofi := fmt.Sprintf("%s/channel/%s/user/gofishgame/%d/%d?", config.Chat[chatName].LogsHost, chatName, year, int(month))
			urls = append(urls, urlSupi, urlGofi)
		} else {
			// Check supi for months before september 2023, check gofishgame for months after september 2023
			if year < 2023 || (year == 2023 && month < time.September) {

				url := fmt.Sprintf("%s/channel/%s/user/supibot/%d/%d?", config.Chat[chatName].LogsHost, chatName, year, int(month))
				urls = append(urls, url)

			} else {

				url := fmt.Sprintf("%s/channel/%s/user/gofishgame/%d/%d?", config.Chat[chatName].LogsHost, chatName, year, int(month))
				urls = append(urls, url)
			}
		}
	}

	return urls
}
