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
// If multiple instances are selected and mode isnt "a", you will add every new fish multiple times!!!!!!!!!!
func CreateURL(chatName string, numMonths int, monthYear string, logInstance string, config utils.Config) []string {

	var urls []string
	var justlogInstance, justlogInstanceAdded string

	// If a different date was specified, update timevar, else use time.Now
	timevar := time.Now()

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
		timevar = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	}

	if config.Chat[chatName].LogsHost == "" {
		logs.Logs().Fatal().
			Str("Chat", chatName).
			Msg("No logs host specified for chat")
	}

	// Get the selected instances, like:  -instance 99,2
	// 0 is the first extra instance!!!
	var selectedInstances []int
	instances := strings.Split(logInstance, ",")
	for a := range instances {
		instance, err := strconv.Atoi(instances[a])
		if err != nil {
			logs.Logs().Fatal().
				Str("Chat", chatName).
				Interface("Selected instances", logInstance).
				Msg("Can't convert instance to int")
		}
		selectedInstances = append(selectedInstances, instance)
	}

	for _, instance := range selectedInstances {

		// 99 is the default value of the "-instance" flag, no chat will ever have 99 instances
		if instance == 99 {
			justlogInstance = config.Chat[chatName].LogsHost
			justlogInstanceAdded = config.Chat[chatName].LogsAdded
		} else {
			// Can check this api https://logs.zonian.dev/instances and search for the channel here to get the chats instances
			// Instead of adding them all manually to the config ? also, just add the "default" instance to the slice as well, dont need two things or?
			if len(config.Chat[chatName].LogsHostAlts) == 0 {
				logs.Logs().Fatal().
					Str("Chat", chatName).
					Msg("No alt logs host specified for chat in config")
			}
			if len(config.Chat[chatName].LogsHostAlts) <= instance {
				logs.Logs().Fatal().
					Str("Chat", chatName).
					Int("Selected instance", instance).
					Interface("Available instances", config.Chat[chatName].LogsHostAlts).
					Msg("Chat does not have that many different instances")
			}
			// Because the extra instances are stored like this in the config: "https://logs.ivr.fi$2024/6",
			parts := strings.Split(config.Chat[chatName].LogsHostAlts[instance], "$")

			justlogInstance = parts[0]
			justlogInstanceAdded = parts[1]
		}

		// Loop through the specified number of months
		for i := 0; i < numMonths; i++ {

			// Get the year and month to check, from the first day of that month, because idk ?
			firstOfMonth := time.Date(timevar.Year(), timevar.Month()-time.Month(i), 1, 0, 0, 0, 0, time.UTC)
			year, month, _ := firstOfMonth.Date()

			// Check if justlog was added to the channel first
			if logsAdded, err := time.Parse("2006/1", justlogInstanceAdded); err == nil {
				if firstOfMonth.Before(logsAdded) {
					logs.Logs().Info().
						Int("Year", year).
						Str("Chat", chatName).
						Int("Month", int(month)).
						Str("Instance", justlogInstance).
						Msg("Breaking because justlog was not added yet in chat")
					break
				}
			} else {
				// Every chat should have this field in the config
				logs.Logs().Fatal().Err(err).
					Str("Chat", chatName).
					Str("Instance", justlogInstance).
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
	}

	return urls
}
