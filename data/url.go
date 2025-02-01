package data

import (
	"fmt"
	"gofish/logs"
	"gofish/utils"
	"strconv"
	"strings"
	"time"
)

// This creates the urls which get checked in data. By default it returns the url of the current month
func CreateURL(chatName string, numMonths int, monthYear string, logInstance int, config utils.Config) []string {

	now := time.Now()
	var urls []string
	var justlogInstance, justlogInstanceAdded string

	if config.Chat[chatName].LogsHost == "" {
		logs.Logs().Fatal().
			Str("Chat", chatName).
			Msg("No logs host specified for chat")
	}

	if logInstance == 99 {
		justlogInstance = config.Chat[chatName].LogsHost
		justlogInstanceAdded = config.Chat[chatName].LogsAdded
	} else {
		// Can check this api https://logs.zonian.dev/instances and search for the channel here to get the chats instances
		// Instead of adding them all manually to the config ?
		if len(config.Chat[chatName].LogsHostAlts) == 0 {
			logs.Logs().Fatal().
				Str("Chat", chatName).
				Msg("No alt logs host specified for chat in config")
		}
		if len(config.Chat[chatName].LogsHostAlts) < logInstance {
			logs.Logs().Fatal().
				Str("Chat", chatName).
				Int("Selected instance", logInstance).
				Int("Amount of instances", len(config.Chat[chatName].LogsHostAlts)).
				Interface("Available instances", config.Chat[chatName].LogsHostAlts).
				Msg("Chat does not have that many different instances")
		}
		parts := strings.Split(config.Chat[chatName].LogsHostAlts[logInstance], "$")

		justlogInstance = parts[0]
		justlogInstanceAdded = parts[1]
	}

	// Get the specified month if it was specified
	if monthYear != "" {
		parts := strings.Split(monthYear, "/")
		if len(parts) != 2 {
			logs.Logs().Fatal().
				Str("MonthYear", monthYear).
				Msg("Invalid month/year format. Use 'yyyy/mm' format.")
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

		// Get the year and month to check, from the first day of that month, because idk ?
		firstOfMonth := time.Date(now.Year(), now.Month()-time.Month(i), 1, 0, 0, 0, 0, time.UTC)
		year, month, _ := firstOfMonth.Date()

		// Check if justlog was added to the channel first
		if logsAdded, err := time.Parse("2006/1", justlogInstanceAdded); err == nil {
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
			urlSupi := fmt.Sprintf("%s/channel/%s/user/supibot/%d/%d?", justlogInstance, chatName, year, int(month))
			urlGofi := fmt.Sprintf("%s/channel/%s/user/gofishgame/%d/%d?", justlogInstance, chatName, year, int(month))
			urls = append(urls, urlSupi, urlGofi)
		} else {
			// Check supi for months before september 2023, check gofishgame for months after september 2023
			if year < 2023 || (year == 2023 && month < time.September) {

				url := fmt.Sprintf("%s/channel/%s/user/supibot/%d/%d?", justlogInstance, chatName, year, int(month))
				urls = append(urls, url)

			} else {

				url := fmt.Sprintf("%s/channel/%s/user/gofishgame/%d/%d?", justlogInstance, chatName, year, int(month))
				urls = append(urls, url)
			}
		}
	}

	return urls
}
