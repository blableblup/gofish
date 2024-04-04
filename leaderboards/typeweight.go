package leaderboards

import (
	"fmt"
	"gofish/lists"
	"gofish/logs"
	"gofish/other"
	"os"
	"path/filepath"
	"strings"
)

func RunTypeWeight(setNames, leaderboard string, numMonths int, monthYear string, mode string) {

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

	switch setNames {
	case "all":
		// Process all sets
		for setName, urlSet := range config.URLSets {
			if !urlSet.CheckEnabled {
				fmt.Printf("Skipping set '%s' because check_enabled is false.\n", setName)
				continue // Skip processing if check_enabled is false
			}

			Weightlimit := urlSet.Weightlimit
			if Weightlimit == "" {
				Weightlimit = "200" // Set the default weight limit if not specified
			}
			fmt.Printf("Checking set '%s'.\n", setName)
			urls := other.CreateURL(setName, numMonths, monthYear)
			processTypeWeight(urls, setName, urlSet, leaderboard, Weightlimit, mode)
		}
	case "":
		fmt.Println("Please specify set names.")
	default:
		// Process specified set names
		specifiedSetNames := strings.Split(setNames, ",")
		for _, setName := range specifiedSetNames {
			urlSet, ok := config.URLSets[setName]
			if !ok {
				fmt.Printf("Set '%s' not found in config.\n", setName)
				continue
			}
			if !urlSet.CheckEnabled {
				fmt.Printf("Skipping set '%s' because check_enabled is false.\n", setName)
				continue // Skip processing if check_enabled is false
			}
			Weightlimit := urlSet.Weightlimit
			if Weightlimit == "" {
				Weightlimit = "200" // Set the default weight limit if not specified
			}
			fmt.Printf("Checking set '%s'.\n", setName)
			urls := other.CreateURL(setName, numMonths, monthYear)
			processTypeWeight(urls, setName, urlSet, leaderboard, Weightlimit, mode)
		}
	}
}

func processTypeWeight(urls []string, setName string, urlSet other.URLSet, leaderboard string, Weightlimit string, mode string) {

	oldRecordWeight, err := other.ReadWeightRankings(urlSet.Weight)
	if err != nil {
		fmt.Println("Error reading old weight leaderboard:", err)
		return
	}

	oldRecordType, err := other.ReadTypeRankings(urlSet.Type)
	if err != nil {
		fmt.Println("Error reading old type leaderboard:", err)
		return
	}

	// Define maps to hold the results
	newRecordWeight := make(map[string]other.Record)
	newRecordType := make(map[string]other.Record)

	// Define a struct to hold the results from CatchNormal
	type CatchResults struct {
		Weight map[string]other.Record
		Type   map[string]other.Record
		Err    error
	}

	// Concurrently fetch data from URLs using CatchNormal function
	catchResults := make(chan CatchResults, len(urls))
	for _, url := range urls {
		go func(url string) {
			// Call CatchNormal with the created maps
			newRecordWeight, newRecordType, err := other.CatchWeightType(url, newRecordWeight, newRecordType, Weightlimit)
			catchResults <- CatchResults{Weight: newRecordWeight, Type: newRecordType, Err: err}
		}(url)
	}

	// Aggregate results
	for i := 0; i < len(urls); i++ {
		result := <-catchResults
		if result.Err != nil {
			fmt.Println("Error fetching data:", result.Err)
			return
		}
		// Merge results into the main maps
		for player, record := range result.Weight {
			if record.Weight > newRecordWeight[player].Weight {
				newRecordWeight[player] = record
			}
		}
		for fishType, record := range result.Type {
			if record.Weight > newRecordType[fishType].Weight {
				newRecordType[fishType] = record
			}
		}
	}

	// Create maps to store updated records
	recordWeight := make(map[string]other.Record)
	recordType := make(map[string]other.Record)

	// Compare old weight records with new ones and update if necessary
	for player, oldWeightRecordInterface := range oldRecordWeight {
		oldWeightRecord, ok := oldWeightRecordInterface.(map[string]interface{})
		if !ok {
			fmt.Println("Error: Could not convert old weight record to map[string]interface{} type")
			continue
		}

		newWeightRecord, exists := newRecordWeight[player]
		if !exists {
			// If the player doesn't have a new record, add their old record to recordWeight
			recordWeight[player] = other.Record{
				Weight: oldWeightRecord["weight"].(float64),
				Type:   oldWeightRecord["type"].(string),
				Bot:    oldWeightRecord["bot"].(string),
			}
		} else if newWeightRecord.Weight > oldWeightRecord["weight"].(float64) {
			// If new record exists and its weight is greater than the old one, update the record
			recordWeight[player] = other.Record{
				Weight: newWeightRecord.Weight,
				Type:   newWeightRecord.Type,
				Bot:    newWeightRecord.Bot,
			}
			fmt.Println("Updated Record Weight for Player", player+":", newWeightRecord)
			if mode != "c" {
				record := "updated"
				// Log the updated record
				logs.WriteWeightLog(setName, record, map[string]other.Record{player: newWeightRecord})
			}
		} else {
			// If the new weight is not greater, keep the old record
			recordWeight[player] = other.Record{
				Weight: oldWeightRecord["weight"].(float64),
				Type:   oldWeightRecord["type"].(string),
				Bot:    oldWeightRecord["bot"].(string),
			}
		}
	}
	// Add players who have a new record but not an old record directly to recordWeight
	for player, newWeightRecord := range newRecordWeight {
		_, exists := oldRecordWeight[player]
		if !exists {
			recordWeight[player] = other.Record{
				Weight: newWeightRecord.Weight,
				Type:   newWeightRecord.Type,
				Bot:    newWeightRecord.Bot,
			}
			fmt.Println("New Record Weight for Player", player+":", newWeightRecord)
			if mode != "c" {
				record := "new"
				// Log the new record
				logs.WriteWeightLog(setName, record, map[string]other.Record{player: newWeightRecord})
			}
		}
	}

	// Compare old type records with new ones and update if necessary
	for fishType, oldTypeRecordInterface := range oldRecordType {
		oldTypeRecord, ok := oldTypeRecordInterface.(map[string]interface{})
		if !ok {
			fmt.Println("Error: Could not convert old type record to map[string]interface{} type")
			continue
		}

		newTypeRecord, exists := newRecordType[fishType]
		if !exists {
			// If the fish type doesn't have a new record, add their old record to recordType
			recordType[fishType] = other.Record{
				Weight: oldTypeRecord["weight"].(float64),
				Player: oldTypeRecord["player"].(string),
				Bot:    fmt.Sprintf("%v", oldTypeRecord["bot"]),
			}
		} else if newTypeRecord.Weight > oldTypeRecord["weight"].(float64) {
			// If new record exists and its weight is greater than the old one, update the record
			recordType[fishType] = other.Record{
				Weight: newTypeRecord.Weight,
				Player: newTypeRecord.Player,
				Bot:    newTypeRecord.Bot,
			}
			fmt.Println("Updated Record Type for Fish Type", fishType+":", newTypeRecord)
			if mode != "c" {
				record := "updated"
				// Log the updated record
				logs.WriteTypeLog(setName, record, map[string]other.Record{fishType: newTypeRecord})
			}
		} else {
			// If the new weight is not greater, keep the old record
			recordType[fishType] = other.Record{
				Weight: oldTypeRecord["weight"].(float64),
				Player: oldTypeRecord["player"].(string),
				Bot:    fmt.Sprintf("%v", oldTypeRecord["bot"]),
			}
		}
	}

	// Add fish types that have new records but not old records directly to recordType
	for fishType, newTypeRecord := range newRecordType {
		_, exists := oldRecordType[fishType]
		if !exists {
			recordType[fishType] = other.Record{
				Weight: newTypeRecord.Weight,
				Player: newTypeRecord.Player,
				Bot:    newTypeRecord.Bot,
			}
			fmt.Println("New Record Type for Fish Type", fishType+":", newTypeRecord)
			if mode != "c" {
				record := "new"
				// Log the new record
				logs.WriteTypeLog(setName, record, map[string]other.Record{fishType: newTypeRecord})
			}
		}
	}

	// Stops the program if it is in "just checking" mode
	if mode == "c" {
		fmt.Printf("Finished checking for new records for set '%s'.\n", setName)
		return
	}

	// Titles for the leaderboards
	titleweight := fmt.Sprintf("### Biggest fish caught per player in %s's chat\n", setName)
	titletype := fmt.Sprintf("### Biggest fish per type caught in %s's chat\n", setName)
	isGlobal := false // Since these will be leaderboards for the chats

	// Update only the specified leaderboard if the leaderboard flag is provided
	switch leaderboard {
	case "type":
		// Write the leaderboard for the biggest fish per fish type to the file specified in the config
		fmt.Printf("Updating type leaderboard for set '%s'...\n", setName)
		err = writeTypeLeaderboard(urlSet.Type, recordType, titletype, isGlobal)
		if err != nil {
			fmt.Println("Error writing type leaderboard:", err)
		} else {
			fmt.Println("Type leaderboard updated successfully.")
		}
	case "weight":
		// Write the leaderboard for the biggest fish caught per player in chat to the file specified in the config
		fmt.Printf("Updating weight leaderboard for set '%s' with weight threshold %s...\n", setName, Weightlimit)
		err = writeWeightLeaderboard(urlSet.Weight, recordWeight, titleweight, isGlobal)
		if err != nil {
			fmt.Println("Error writing weight leaderboard:", err)
		} else {
			fmt.Println("Weight leaderboard updated successfully.")
		}
	default:
		// If the leaderboard flag is not provided, update both leaderboards
		fmt.Printf("Updating type leaderboard for set '%s'...\n", setName)
		err = writeTypeLeaderboard(urlSet.Type, recordType, titletype, isGlobal)
		if err != nil {
			fmt.Println("Error writing type leaderboard:", err)
		} else {
			fmt.Println("Type leaderboard updated successfully.")
		}

		fmt.Printf("Updating weight leaderboard for set '%s' with weight threshold %s...\n", setName, Weightlimit)
		err = writeWeightLeaderboard(urlSet.Weight, recordWeight, titleweight, isGlobal)
		if err != nil {
			fmt.Println("Error writing weight leaderboard:", err)
		} else {
			fmt.Println("Weight leaderboard updated successfully.")
		}
	}
}

func writeWeightLeaderboard(filePath string, recordWeight map[string]other.Record, titleweight string, isGlobal bool) error {
	// Call ReadWeightRankings to get the weight rankings
	oldLeaderboardWeight, err := other.ReadWeightRankings(filePath)
	if err != nil {
		return err
	}

	// Ensure that the directory exists before attempting to create the file
	err = os.MkdirAll(filepath.Dir(filePath), 0755)
	if err != nil {
		return err
	}

	// Open the file for writing (or create it if it doesn't exist)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write the title
	_, err = fmt.Fprintf(file, "%s", titleweight)
	if err != nil {
		return err
	}

	// Write the header
	_, err = fmt.Fprintln(file, "| Rank | Player | Fish | Weight in lbs ⚖️ |"+func() string {
		if isGlobal {
			return " Chat |"
		}
		return ""
	}())
	_, err = fmt.Fprintln(file, "|------|--------|-----------|---------|"+func() string {
		if isGlobal {
			return "-------|"
		}
		return ""
	}())
	if err != nil {
		return err
	}

	// Import the list from lists
	verifiedPlayers := lists.ReadVerifiedPlayers()

	// Extract weights and fish types from recordWeight map
	weights := make(map[string]float64)
	fishTypes := make(map[string]string)
	for player, record := range recordWeight {
		weights[player] = record.Weight
		fishTypes[player] = record.Type
	}

	// Sort players by the weight of their fish
	sortedPlayers := other.SortMapByValueDesc(weights)

	// Write the leaderboard data
	rank := 1
	prevRank := 1
	prevWeight := -1.0
	occupiedRanks := make(map[int]int)

	for _, player := range sortedPlayers {
		weight := weights[player]     // Fetching the weight for the current player
		fishType := fishTypes[player] // Fetching the fishType for the current player

		// Increment rank only if the count has changed
		if weight != prevWeight {
			rank += occupiedRanks[rank] // Increment rank by the number of occupied ranks
			// Reset the count of occupied ranks when count changes
			occupiedRanks[rank] = 1
		} else {
			// Set the rank to the previous rank if the count hasn't changed
			rank = prevRank
			occupiedRanks[rank]++ // Increment the count of occupied ranks
		}

		// Declare found variable in the outer scope
		var found bool

		// Getting the old rank
		oldRank := -1 // Default value if the old rank is not found
		if oldPlayerData, ok := oldLeaderboardWeight[player]; ok {
			found = true
			if oldPlayerDataMap, ok := oldPlayerData.(map[string]interface{}); ok {
				if oldRankValue, rankFound := oldPlayerDataMap["rank"]; rankFound {
					oldRank, _ = oldRankValue.(int) // Assuming rank is stored as an int
				}
			}
		}

		changeEmoji := other.ChangeEmoji(rank, oldRank, found)

		// Getting the old weight
		oldWeight := weight // Default value if the old weight is not found
		if oldPlayerData, ok := oldLeaderboardWeight[player]; ok {
			found = true
			if oldPlayerDataMap, ok := oldPlayerData.(map[string]interface{}); ok {
				if oldWeightValue, weightFound := oldPlayerDataMap["weight"]; weightFound {
					oldWeight, _ = oldWeightValue.(float64)
				}
			}
		}

		// Define fishweight outside the if clause
		var fishweight string

		// Construct the string with the difference in brackets
		weightDifference := weight - oldWeight

		if weightDifference > 0 {
			fishweight = fmt.Sprintf("%.2f (+%.2f)", weight, weightDifference)
		} else {
			fishweight = fmt.Sprintf("%.2f", weight)
		}

		botIndicator := ""
		if recordWeight[player].Bot == "supibot" && !other.Contains(verifiedPlayers, player) {
			botIndicator = "*"
		}

		ranks := other.Ranks(rank)

		// Write the leaderboard row
		_, err = fmt.Fprintf(file, "| %s %s | %s%s | %s | %s |", ranks, changeEmoji, player, botIndicator, fishType, fishweight)
		if isGlobal {
			_, err = fmt.Fprintf(file, " %s |", recordWeight[player].Chat)
		}
		_, err = fmt.Fprintln(file)
		if err != nil {
			return err
		}

		prevWeight = weight // Update previous weight
		prevRank = rank     // Update previous rank
	}

	// Write the note
	_, err = fmt.Fprintln(file, "\n_* = The fish was caught on supibot and the player did not migrate their data over to gofishgame. Because of that their data was not individually verified to be accurate._")
	if err != nil {
		return err
	}

	return nil
}

func writeTypeLeaderboard(filePath string, recordType map[string]other.Record, titletype string, isGlobal bool) error {
	// Call ReadOldTypeRankings to get the type rankings
	oldLeaderboardType, err := other.ReadTypeRankings(filePath)
	if err != nil {
		return err
	}

	// Ensure that the directory exists before attempting to create the file
	err = os.MkdirAll(filepath.Dir(filePath), 0755)
	if err != nil {
		return err
	}

	// Open the file for writing (or create it if it doesn't exist)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write the title
	_, err = fmt.Fprintf(file, "%s", titletype)
	if err != nil {
		return err
	}

	// Write the header
	_, err = fmt.Fprintln(file, "| Rank | Fish | Weight in lbs | Player |"+func() string {
		if isGlobal {
			return " Chat |"
		}
		return ""
	}())
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(file, "|------|--------|-----------|---------|"+func() string {
		if isGlobal {
			return "-------|"
		}
		return ""
	}())
	if err != nil {
		return err
	}

	// Import the list from lists
	verifiedPlayers := lists.ReadVerifiedPlayers()

	// Extract weights and players from recordType map
	weights := make(map[string]float64)
	players := make(map[string]string)
	for Type, record := range recordType {
		weights[Type] = record.Weight
		players[Type] = record.Player
	}

	// Sort types by their biggest weight
	sortedTypes := other.SortMapByValueDesc(weights)

	// Write the leaderboard data
	rank := 1
	prevRank := 1
	prevWeight := -1.0
	occupiedRanks := make(map[int]int)

	for _, fishType := range sortedTypes {
		weight := weights[fishType] // Fetching the weight for the current fish type
		player := players[fishType] // Fetching the fishType for the current fish type

		// Increment rank only if the count has changed
		if weight != prevWeight {
			rank += occupiedRanks[rank] // Increment rank by the number of occupied ranks
			// Reset the count of occupied ranks when count changes
			occupiedRanks[rank] = 1
		} else {
			// Set the rank to the previous rank if the count hasn't changed
			rank = prevRank
			occupiedRanks[rank]++ // Increment the count of occupied ranks
		}

		// Declare found variable in the outer scope
		var found bool

		// Getting the old rank
		oldRank := -1 // Default value if the old rank is not found
		if oldTypeData, ok := oldLeaderboardType[fishType]; ok {
			found = true
			if oldTypeDataMap, ok := oldTypeData.(map[string]interface{}); ok {
				if oldRankValue, rankFound := oldTypeDataMap["rank"]; rankFound {
					oldRank, _ = oldRankValue.(int) // Assuming rank is stored as an int
				}
			}
		}

		changeEmoji := other.ChangeEmoji(rank, oldRank, found)

		// Getting the old weight
		oldWeight := weight // Default value if the old weight is not found
		if oldTypeData, ok := oldLeaderboardType[fishType]; ok {
			found = true
			if oldTypeDataMap, ok := oldTypeData.(map[string]interface{}); ok {
				if oldWeightValue, weightFound := oldTypeDataMap["weight"]; weightFound {
					oldWeight, _ = oldWeightValue.(float64)
				}
			}
		}

		// Define fishweight outside the if clause
		var fishweight string

		// Construct the string with the difference in brackets
		weightDifference := weight - oldWeight

		if weightDifference > 0 {
			fishweight = fmt.Sprintf("%.2f (+%.2f)", weight, weightDifference)
		} else {
			fishweight = fmt.Sprintf("%.2f", weight)
		}

		botIndicator := ""
		if recordType[fishType].Bot == "supibot" && !other.Contains(verifiedPlayers, player) {
			botIndicator = "*"
		}

		ranks := other.Ranks(rank)

		_, err = fmt.Fprintf(file, "| %s %s | %s | %s | %s%s |", ranks, changeEmoji, fishType, fishweight, player, botIndicator)
		if isGlobal {
			_, err = fmt.Fprintf(file, " %s |", recordType[fishType].Chat)
		}
		_, err = fmt.Fprintln(file)
		if err != nil {
			return err
		}

		prevWeight = weight // Update previous fish weight
		prevRank = rank     // Update previous rank
	}

	// Write the note
	_, err = fmt.Fprintln(file, "\n_* = The fish was caught on supibot and the player did not migrate their data over to gofishgame. Because of that their data was not individually verified to be accurate._")
	if err != nil {
		return err
	}

	return nil
}
