package leaderboards

import (
	"fmt"
	"gofish/lists"
	"gofish/other"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

func RunTotalcount(setNames, leaderboard string, numMonths int, monthYear string) {

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
		for setName := range config.URLSets {
			Totalcountlimit := config.URLSets[setName].Totalcountlimit
			if Totalcountlimit == "" {
				Totalcountlimit = "100" // Set the default count limit if not specified
			}
			urls := other.CreateURL(setName, numMonths, monthYear)
			processTotalcount(urls, setName, config.URLSets[setName], leaderboard, Totalcountlimit)
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
			Totalcountlimit := urlSet.Totalcountlimit
			if Totalcountlimit == "" {
				Totalcountlimit = "100" // Set the default count limit if not specified
			}
			urls := other.CreateURL(setName, numMonths, monthYear)
			processTotalcount(urls, setName, urlSet, leaderboard, Totalcountlimit)
		}
	}
}

func processTotalcount(urls []string, setName string, urlSet other.URLSet, leaderboard string, Totalcountlimit string) {

	// Convert Totalcountlimit to an integer
	totalCountLimitInt, err := strconv.Atoi(Totalcountlimit)
	if err != nil {
		fmt.Println("Error converting Totalcountlimit to integer:", err)
		return
	}

	// Define maps to hold the results
	fishCaught := make(map[string]int)
	var fishCaughtMutex sync.Mutex
	var wg sync.WaitGroup

	// Concurrently fetch data from URLs using CountFishCaught function
	for _, url := range urls {
		// Create a copy of fishCaught for each goroutine
		fishCaughtCopy := make(map[string]int)
		for player, count := range fishCaught {
			fishCaughtCopy[player] = count
		}

		wg.Add(1)
		go func(url string, fishCaughtCopy map[string]int) {
			defer wg.Done()
			fishCounts, err := other.CountFishCaught(url, fishCaughtCopy)
			if err != nil {
				fmt.Println("Error counting fish caught:", err)
				return
			}
			// Lock the mutex before updating the shared map
			fishCaughtMutex.Lock()
			defer fishCaughtMutex.Unlock()
			// Aggregate results
			for player, count := range fishCounts {
				fishCaught[player] += count
			}
		}(url, fishCaughtCopy)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Filter out players with counts less than or equal to Totalcountlimit
	for player, count := range fishCaught {
		if count <= totalCountLimitInt {
			delete(fishCaught, player)
		}
	}

	// Update only the specified leaderboard if the leaderboard flag is provided
	switch leaderboard {
	case "count":
		// Write the leaderboard for the total fish caught to the file specified in the config
		fmt.Printf("Updating totalcount leaderboard for set '%s' with count threshold %s...\n", setName, Totalcountlimit)
		err = writeTotalcount(urlSet.Totalcount, setName, fishCaught)
		if err != nil {
			fmt.Println("Error writing totalcount leaderboard:", err)
		} else {
			fmt.Println("Totalcount leaderboard updated successfully.")
		}
	default:
		fmt.Println("This does nothing.") // Add more cases
	}
}

// Function to write the Totalcount leaderboard with emojis indicating ranking change and the count change in brackets
func writeTotalcount(filePath string, setName string, fishCaught map[string]int) error {
	// Call ReadTotalcountRankings to get the totalcount rankings
	oldLeaderboardCount, err := other.ReadTotalcountRankings(filePath)
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
	_, err = fmt.Fprintf(file, "### Most fish caught in %s's chat (since gofish was added)\n", setName)
	if err != nil {
		return err
	}

	// Write the header
	_, err = fmt.Fprintln(file, "| Rank | Player | Fish Caught |")
	_, err = fmt.Fprintln(file, "|------|--------|-----------|")
	if err != nil {
		return err
	}

	// Import the list from lists
	verifiedPlayers := lists.ReadVerifiedPlayers()

	// Extract count from fishCount map
	fishCount := make(map[string]int)
	for player, count := range fishCaught {
		fishCount[player] = count
	}

	// Sort players by their fish count
	sortedPlayers := other.SortMapByValueDescInt(fishCaught)

	// Write the leaderboard data
	rank := 1
	prevRank := 1
	prevCount := -1
	occupiedRanks := make(map[int]int)

	for _, player := range sortedPlayers {
		count := fishCaught[player] // Fetching the count for the current player

		// Increment rank only if the count has changed
		if count != prevCount {
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
		if oldPlayerData, ok := oldLeaderboardCount[player]; ok {
			found = true
			if oldPlayerDataMap, ok := oldPlayerData.(map[string]interface{}); ok {
				if oldRankValue, rankFound := oldPlayerDataMap["rank"]; rankFound {
					oldRank, _ = oldRankValue.(int) // Assuming rank is stored as an int
				}
			}
		}

		var changeEmoji string
		if found {
			if rank < oldRank {
				changeEmoji = "⬆" // Emoji indicating rank increase
			} else if rank > oldRank {
				changeEmoji = "⬇" // Emoji indicating rank decrease
			} else {
				changeEmoji = "" // Emoji indicating no change in rank
			}
		} else {
			changeEmoji = "🆕" // Emoji indicating new player
		}

		// Getting the old count
		oldCount := 0 // Default value if the old count is not found
		if oldPlayerData, ok := oldLeaderboardCount[player]; ok {
			found = true
			if oldPlayerDataMap, ok := oldPlayerData.(map[string]interface{}); ok {
				if oldCountValue, countFound := oldPlayerDataMap["count"]; countFound {
					if count, ok := oldCountValue.(int); ok {
						oldCount = count
					}
				}
			}
		}

		// Define counts outside the if clause
		var counts string

		// Construct the string with the difference in brackets
		countDifference := count - oldCount

		if countDifference > 0 {
			counts = fmt.Sprintf("%d (+%d)", count, countDifference)
		} else {
			counts = fmt.Sprintf("%d ", count)
		}

		// Getting the old bot value
		oldBot := ""
		if oldPlayerData, ok := oldLeaderboardCount[player].(map[string]interface{}); ok {
			if botValue, botFound := oldPlayerData["bot"]; botFound {
				if botString, ok := botValue.(string); ok {
					oldBot = botString
				}
			}
		}

		botIndicator := ""
		if oldBot == "supibot" && !other.Contains(verifiedPlayers, player) {
			botIndicator = "*"
		}

		var ranks string

		switch rank {
		case 1:
			ranks = fmt.Sprintf("%d 🥇", rank)
		case 2:
			ranks = fmt.Sprintf("%d 🥈", rank)
		case 3:
			ranks = fmt.Sprintf("%d 🥉", rank)
		default:
			ranks = fmt.Sprintf("%d", rank)
		}

		// Write the leaderboard row
		_, err := fmt.Fprintf(file, "| %s %s | %s%s | %s |\n", ranks, changeEmoji, player, botIndicator, counts)
		if err != nil {
			return err
		}

		prevCount = count // Update previous count
		prevRank = rank   // Update previous rank

	}

	// Write the note
	_, err = fmt.Fprintln(file, "\n_* = The player caught their first fish on supibot and did not migrate their data to gofishgame. Because of that their data was not individually verified to be accurate._")
	if err != nil {
		return err
	}

	return nil
}
