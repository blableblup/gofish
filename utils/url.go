package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// createURL generates URLs based on the given arguments and returns them
func CreateURL(chatName string, numMonths int, monthYear string) []string {

	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		os.Exit(1)
	}

	// Construct the absolute path to the config file
	configFilePath := filepath.Join(wd, "config.json")

	// Load the config from the constructed file path
	config := LoadConfig(configFilePath)

	now := time.Now()
	var urls []string

	// Start from the specified month/year or current month/year
	if monthYear != "" {
		parts := strings.Split(monthYear, "/")
		if len(parts) != 2 {
			fmt.Println("Invalid month/year format. Please use 'yyyy/mm' format.")
			os.Exit(1)
		}
		year, err := strconv.Atoi(parts[0])
		if err != nil {
			fmt.Println("Invalid year:", err)
			os.Exit(1)
		}
		month, err := strconv.Atoi(parts[1])
		if err != nil || month < 1 || month > 12 {
			fmt.Println("Invalid month:", err)
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
				fmt.Printf("Breaking at %d/%d because gofish was not added yet in chat '%s'\n", year, month, chatName)
				break
			}
		} else {
			fmt.Println("Error parsing LogsAdded:", err)
		}

		// Check if the current month is within September 2023
		if year == 2023 && month == time.September && config.Chat[chatName].LogsHostOld != "" {
			// Use both the old and new logs hosts
			urlOld := fmt.Sprintf("%s%d/%d?", config.Chat[chatName].LogsHostOld, year, int(month))
			urlNew := fmt.Sprintf("%s%d/%d?", config.Chat[chatName].LogsHost, year, int(month))
			fmt.Println("Fetching data from supibot:", urlOld)    // Print the URL being used for old logs host
			fmt.Println("Fetching data from gofishgame:", urlNew) // Print the URL being used for new logs host
			urls = append(urls, urlOld, urlNew)
		} else {
			// Check if the current month is before the logs host change
			if year < 2023 || (year == 2023 && month < time.September) {
				// Use the old logs host if it's not empty
				if config.Chat[chatName].LogsHostOld != "" {
					url := fmt.Sprintf("%s%d/%d?", config.Chat[chatName].LogsHostOld, year, int(month))
					fmt.Println("Fetching data from supibot:", url) // Print the URL being used
					urls = append(urls, url)
				} else {
					fmt.Println("There is no old logs host specified. Skipping...")
				}
			} else {
				// Use the current logs host
				url := fmt.Sprintf("%s%d/%d?", config.Chat[chatName].LogsHost, year, int(month))
				fmt.Println("Fetching data from gofishgame:", url) // Print the URL being used
				urls = append(urls, url)
			}
		}
	}

	return urls
}
