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
func CreateURL(chatName string, chat utils.ChatInfo, numMonths int, monthYear string, logInstance string) []string {

	// If a different date was specified, update timevar, else use time.Now in UTC, because all the instances are in UTC
	timevar := time.Now().UTC()

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

	if len(chat.LogsInstances) == 0 {
		logs.Logs().Fatal().
			Str("Chat", chatName).
			Msg("No logs host specified for chat in config !")
	}

	// Get the selected instances, default instance is "0" which is the first in the slice
	// Can be the number of the instance in the config slice, or the name of the instance; like -instance 2,ivr,potat
	// Double selecting the same instance doesnt work; that will just overwrite selectedInstances[...]...
	// But checking multiple instances for the same chat at the same time will just add the same fish multiple times
	// Even with mode "a",(if the fish was missing in the db), so this is kinda useless ?
	// Thats also why there is no -instance all, or maybe could update data to check if the fish also exists in the fish slice instead of only in the db
	// to remove duplicates from the other instances ?
	selectedInstances := make(map[string]string)
	instances := strings.Split(logInstance, ",")
	for a := range instances {
		instance, err := strconv.Atoi(instances[a])
		if err != nil {
			// Check if the name of an instance was selected instead
			thatinstanceexists := false
			for _, existinginstance := range chat.LogsInstances {
				if strings.Contains(existinginstance.URL, instances[a]) {
					selectedInstances[existinginstance.URL] = existinginstance.LogsAdded
					thatinstanceexists = true
					break
				}
			}
			if !thatinstanceexists {
				logs.Logs().Fatal().
					Str("Chat", chatName).
					Str("Instance", instances[a]).
					Msg("Selected instance does not exist")
			}
		} else {
			// If err nil, select the element of the slice as the instance
			if len(chat.LogsInstances) <= instance {
				logs.Logs().Fatal().
					Str("Chat", chatName).
					Int("Selected instance", instance).
					Interface("Available instances", chat.LogsInstances).
					Msg("Chat does not have that many different instances !")
			}
			selectedInstances[chat.LogsInstances[instance].URL] = chat.LogsInstances[instance].LogsAdded
		}
	}

	// Go over the instances and create the urls
	var urls []string

	for justlogInstance, justlogInstanceAdded := range selectedInstances {

		// Loop through the specified number of months
		for i := range numMonths {

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
				// "no logs added found" just means that gofishgame never typed in that chat (in that instance) when the instance was added
				// dont need to fatal, will just check urls which will 404 but its ok
				// If gofishgame typed and there are logs to parse, can update logs added for the instance manually
				if justlogInstanceAdded != "no logs added found" {
					logs.Logs().Fatal().Err(err).
						Str("Chat", chatName).
						Str("Instance", justlogInstance).
						Msg("Unable to parse LogsAdded for chat")
				}
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
