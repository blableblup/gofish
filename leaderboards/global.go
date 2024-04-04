package leaderboards

import (
	"fmt"
	"gofish/other"
	"os"
	"path/filepath"
	"strconv"
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
	for _, urlSet := range config.URLSets {
		oldRecordType, err := other.ReadTypeRankings(urlSet.Type)
		if err != nil {
			fmt.Println("Error reading old type leaderboard:", err)
			continue
		}

		// Combine old type records into global record, keeping only the biggest record per fish type
		for fishType, oldRecord := range oldRecordType {
			if record, ok := oldRecord.(map[string]interface{}); ok {
				weight, weightOK := record["weight"].(float64)
				player, playerOK := record["player"].(string)
				bot, botOK := record["bot"].(string)

				if weightOK && playerOK && botOK {
					existingRecord, exists := globalRecordType[fishType]
					if !exists || weight > existingRecord.Weight {
						globalRecordType[fishType] = other.Record{
							Weight: weight,
							Player: player,
							Bot:    bot,
							Chat:   urlSet.Emoji,
						}
					}
				} else {
					fmt.Println("Error: Incomplete record for fish type", fishType)
				}
			} else {
				fmt.Println("Error: Could not convert old type record to map[string]interface{} type")
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
	globalWeightLimit := config.URLSets["global"].Weightlimit
	weightLimit, err := strconv.ParseFloat(globalWeightLimit, 64)
	if err != nil {
		fmt.Printf("Error parsing weight limit for 'global' URL set: %v\n", err)
		return
	}

	// Process all sets
	for setName, urlSet := range config.URLSets {
		oldRecordWeight, err := other.ReadWeightRankings(urlSet.Weight)
		if err != nil {
			fmt.Printf("Error reading old weight leaderboard for set '%s': %v\n", setName, err)
			continue
		}

		// Combine old weight records into global record, keeping only the biggest record per player
		for player, oldRecord := range oldRecordWeight {
			if record, ok := oldRecord.(map[string]interface{}); ok {
				weight, weightOK := record["weight"].(float64)
				fishType, fishTypeOK := record["type"].(string)
				bot, botOK := record["bot"].(string)

				if weightOK && fishTypeOK && botOK && weight > weightLimit {
					existingRecord, exists := globalRecordWeight[player]
					if !exists || weight > existingRecord.Weight {
						globalRecordWeight[player] = other.Record{
							Weight: weight,
							Type:   fishType,
							Bot:    bot,
							Chat:   urlSet.Emoji,
						}
					}
				} else if !weightOK || !fishTypeOK || !botOK {
					fmt.Println("Error: Incomplete record for player", player)
				}
			} else {
				fmt.Println("Error: Could not convert old weight record to map[string]interface{} type")
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
		fmt.Println("Type leaderboard updated successfully.")
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
		fmt.Println("Weight leaderboard updated successfully.")
	}
}
