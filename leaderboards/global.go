package leaderboards

import (
	"fmt"
	"gofish/other"
	"os"
	"path/filepath"
)

func RunGlobal(leaderboard string) {
	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		os.Exit(1)
	}

	// Construct the absolute path to the config file
	configFilePath := filepath.Join(wd, "config.json")

	// Load the config from the constructed file path
	config := other.LoadConfig(configFilePath)

	// Titles for the leaderboards
	titleweight := "### Biggest fish caught per player globally\n"
	titletype := "### Biggest fish per type caught globally\n"

	switch leaderboard {
	case "type":
		RunTypeGlobal(config, titletype)
	case "weight":
		RunWeightGlobal(config, titleweight)
	case "all":
		RunWeightGlobal(config, titleweight)
		RunTypeGlobal(config, titletype)
	default:
		fmt.Println("Please specify a valid leaderboard type.")
	}
}

func RunTypeGlobal(config other.Config, title string) {
	// Create a map to store combined records
	globalRecordType := make(map[string]other.Record)

	// Process all sets
	for setName, urlSet := range config.URLSets {
		if !urlSet.CheckEnabled {
			fmt.Printf("Skipping set '%s' because check_enabled is false.\n", setName)
			continue // Skip processing if check_enabled is false
		}

		oldRecordType, err := other.ReadTypeRankings(urlSet.Type)
		if err != nil {
			fmt.Printf("Error reading old type leaderboard for set '%s': %v\n", setName, err)
			continue
		}

		// Combine old type records into global record, keeping only the biggest record per fish type
		for fishType, oldRecord := range oldRecordType {
			convertedRecord := other.ConvertToRecord(oldRecord)

			existingRecord, exists := globalRecordType[fishType]
			if !exists || convertedRecord.Weight > existingRecord.Weight {
				globalRecordType[fishType] = convertedRecord
			}
		}
	}

	// Write the global type leaderboard
	updateTypeLeaderboard(config, globalRecordType, title)
}

func RunWeightGlobal(config other.Config, title string) {
	// Create a map to store combined records
	globalRecordWeight := make(map[string]other.Record)

	// Get the weight limit from the "global" URL set configuration
	WeightLimit := config.URLSets["global"].Weightlimit

	// Process all sets
	for setName, urlSet := range config.URLSets {
		if !urlSet.CheckEnabled {
			fmt.Printf("Skipping set '%s' because check_enabled is false.\n", setName)
			continue // Skip processing if check_enabled is false
		}

		oldRecordWeight, err := other.ReadWeightRankings(urlSet.Weight)
		if err != nil {
			fmt.Printf("Error reading old weight leaderboard for set '%s': %v\n", setName, err)
			continue
		}

		// Combine old weight records into global record, keeping only the biggest record per player
		for player, oldRecord := range oldRecordWeight {
			convertedRecord := other.ConvertToRecord(oldRecord)

			if convertedRecord.Weight > WeightLimit {
				existingRecord, exists := globalRecordWeight[player]
				if !exists || convertedRecord.Weight > existingRecord.Weight {
					globalRecordWeight[player] = convertedRecord
				}
			}
		}
	}

	// Write the global weight leaderboard
	updateWeightLeaderboard(config, globalRecordWeight, title)
}

// Update the type leaderboard
func updateTypeLeaderboard(config other.Config, recordType map[string]other.Record, title string) {
	fmt.Println("Updating global type leaderboard...")
	isGlobal := true
	err := writeTypeLeaderboard(config.URLSets["global"].Type, recordType, title, isGlobal)
	if err != nil {
		fmt.Println("Error writing type leaderboard:", err)
	} else {
		fmt.Println("Global type leaderboard updated successfully.")
	}
}

// Update the weight leaderboard
func updateWeightLeaderboard(config other.Config, recordWeight map[string]other.Record, title string) {
	fmt.Println("Updating global weight leaderboard...")
	isGlobal := true
	err := writeWeightLeaderboard(config.URLSets["global"].Weight, recordWeight, title, isGlobal)
	if err != nil {
		fmt.Println("Error writing weight leaderboard:", err)
	} else {
		fmt.Println("Global weight leaderboard updated successfully.")
	}
}
