package leaderboards

import (
	"bufio"
	"fmt"
	"gofish/lists"
	"gofish/other"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type PlayerInfo struct {
	FishCount int
	Bot       string
}

type PlayerCounts struct {
	Trophy int
	Silver int
	Bronze int
}

func RunTournaments(setNames, leaderboard string) {

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

	// Define point values
	pointValues := map[string]float64{"Trophy": 3, "Silver": 1, "Bronze": 0.5}

	switch setNames {
	case "all":
		// Process all sets
		for setName, urlSet := range config.URLSets {
			if !urlSet.CheckEnabled {
				fmt.Printf("Skipping set '%s' because check_enabled is false.\n", setName)
				continue // Skip processing if check_enabled is false
			}
			fishweekLimit := urlSet.Fishweeklimit
			if fishweekLimit == "" {
				fishweekLimit = "20" // Set the default fishweek limit if not specified
			}
			fmt.Printf("Checking set '%s'.\n", setName)
			processTournaments(setName, urlSet, pointValues, leaderboard, fishweekLimit)
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
			fishweekLimit := urlSet.Fishweeklimit
			if fishweekLimit == "" {
				fishweekLimit = "20" // Set the default fishweek limit if not specified
			}
			fmt.Printf("Checking set '%s'.\n", setName)
			processTournaments(setName, urlSet, pointValues, leaderboard, fishweekLimit)
		}
	}
}

func processTournaments(setName string, urlSet other.URLSet, pointValues map[string]float64, leaderboard string, fishweekLimit string) {

	// Import the lists from lists
	cheaters := lists.ReadCheaters()
	renamedChatters := lists.ReadRenamedChatters()

	// Check if fish count is greater than or equal to the fishweek limit
	fishCountThreshold, err := strconv.Atoi(fishweekLimit)
	if err != nil {
		fmt.Printf("Invalid fishweek limit for set '%s': %v\n", setName, err)
		return
	}
	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		os.Exit(1)
	}
	// Open and read the logs file for the set
	logsFilePath := filepath.Join(wd, "logs", urlSet.Logs)
	logs, err := os.Open(logsFilePath)
	if err != nil {
		panic(err)
	}
	defer logs.Close()

	// Define a map to store the maximum fish caught in a week for each player and the bot name
	maxFishInWeek := make(map[string]PlayerInfo)

	// Define a map to store the counts for each player's trophies and medals
	playerCounts := make(map[string]PlayerCounts)

	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		line := scanner.Text()

		// Find the player
		playerMatch := regexp.MustCompile(`[@👥]\s?(\w+)`).FindStringSubmatch(line)
		if len(playerMatch) > 0 {
			player := playerMatch[1]

			// Skip processing for ignored players
			found := false
			for _, c := range cheaters {
				if c == player {
					found = true
					break
				}
			}
			if found {
				continue // Skip processing for ignored players
			}

			// Change to the latest name
			newPlayer := renamedChatters[player]
			for newPlayer != "" {
				player = newPlayer
				newPlayer = renamedChatters[player]
			}

			// Get the amount of fish the player caught
			fishMatch := regexp.MustCompile(`(\d+) fish: (\w+)`).FindStringSubmatch(line)
			if len(fishMatch) > 0 {
				fishCount, _ := strconv.Atoi(fishMatch[1])
				botMatch := regexp.MustCompile(`#\w+ \s?(\w+):`).FindStringSubmatch(line)
				if len(botMatch) > 0 {
					bot := botMatch[1]

					// Update the record if the current fish count is greater
					if fishCount > maxFishInWeek[player].FishCount {
						maxFishInWeek[player] = PlayerInfo{FishCount: fishCount, Bot: bot}
					}
				}
			}

			// Find all their medals and trophies
			achievements := regexp.MustCompile(`(Victory|champion|runner-up|third)`).FindAllString(line, -1)
			for _, achievement := range achievements {
				switch achievement {
				case "Victory", "champion":
					// Get the player's counts
					counts := playerCounts[player]

					// Update the Trophy count
					counts.Trophy++

					// Assign the updated counts back to the map
					playerCounts[player] = counts
				case "runner-up":
					// Get the player's counts
					counts := playerCounts[player]

					// Update the Silver count
					counts.Silver++

					// Assign the updated counts back to the map
					playerCounts[player] = counts
				case "third":
					// Get the player's counts
					counts := playerCounts[player]

					// Update the Bronze count
					counts.Bronze++

					// Assign the updated counts back to the map
					playerCounts[player] = counts
				}
			}
		}
	}
	// Update only the specified leaderboard if the leaderboard flag is provided
	switch leaderboard {
	case "trophy":
		// Write the leaderboard for the weekly tournaments to the file specified in the config
		fmt.Printf("Updating trophies leaderboard for set '%s'...\n", setName)
		err = writeTrophiesLeaderboard(urlSet.Trophies, setName, playerCounts, pointValues)
		if err != nil {
			fmt.Println("Error writing trophies leaderboard:", err)
		} else {
			fmt.Println("Trophies leaderboard updated successfully.")
		}
	case "fishw":
		// Write the leaderboard for the most fish caught in a single week in tournaments to the file specified in the config
		fmt.Printf("Updating fishweek leaderboard for set '%s' with fish count threshold %d...\n", setName, fishCountThreshold)
		err = writeFishWeekLeaderboard(urlSet.Fishweek, setName, maxFishInWeek, fishCountThreshold)
		if err != nil {
			fmt.Println("Error writing fishweek leaderboard:", err)
		} else {
			fmt.Println("Fishweek leaderboard updated successfully.")
		}
	default:
		// If the leaderboard flag is not provided, update both leaderboards
		fmt.Printf("Updating trophies leaderboard for set '%s'...\n", setName)
		err = writeTrophiesLeaderboard(urlSet.Trophies, setName, playerCounts, pointValues)
		if err != nil {
			fmt.Println("Error writing trophies leaderboard:", err)
		} else {
			fmt.Println("Trophies leaderboard updated successfully.")
		}

		fmt.Printf("Updating fishweek leaderboard for set '%s' with fish count threshold %d...\n", setName, fishCountThreshold)
		err = writeFishWeekLeaderboard(urlSet.Fishweek, setName, maxFishInWeek, fishCountThreshold)
		if err != nil {
			fmt.Println("Error writing fishweek leaderboard:", err)
		} else {
			fmt.Println("Fishweek leaderboard updated successfully.")
		}
	}
}

// Function to write the Trophies leaderboard with emojis indicating ranking change and the change of trophies and medals in brackets
func writeTrophiesLeaderboard(filePath string, setName string, playerCounts map[string]PlayerCounts, pointValues map[string]float64) error {
	// Call ReadOldTrophyRankings to get the old trophy rankings and player counts
	oldTrophyRankings, oldPlayerCounts, err := other.ReadOldTrophyRankings(filePath)
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
	_, err = fmt.Fprintf(file, "### Leaderboard for the weekly tournaments in %s's chat\n", setName)
	if err != nil {
		return err
	}

	// Write the header
	_, err = fmt.Fprintln(file, "| Rank | Player | Trophies 🏆 | Silver Medals 🥈 | Bronze Medals 🥉 | Points |")
	_, err = fmt.Fprintln(file, "|------|--------|-------------|------------------|------------------|--------|")
	if err != nil {
		return err
	}

	// Calculate total points for each player
	totalPoints := make(map[string]float64)
	for player, counts := range playerCounts {
		totalPoints[player] = float64(counts.Trophy)*pointValues["Trophy"] + float64(counts.Silver)*pointValues["Silver"] + float64(counts.Bronze)*pointValues["Bronze"]
	}

	// Sort players by total points
	sortedPlayers := other.SortMapByValueDesc(totalPoints)

	// Write the leaderboard data
	rank := 1
	prevRank := 1
	prevPoints := -1.0
	occupiedRanks := make(map[int]int)

	for _, player := range sortedPlayers {
		points := totalPoints[player]

		// Increment rank only if the count has changed
		if points != prevPoints {
			rank += occupiedRanks[rank] // Increment rank by the number of occupied ranks
			// Reset the count of occupied ranks when count changes
			occupiedRanks[rank] = 1
		} else {
			// Set the rank to the previous rank if the count hasn't changed
			rank = prevRank
			occupiedRanks[rank]++ // Increment the count of occupied ranks
		}

		oldRank, found := oldTrophyRankings[player]
		changeEmoji := other.ChangeEmoji(rank, oldRank, found)

		// Compare new counts to old counts and display the difference
		trophiesDifference := playerCounts[player].Trophy - oldPlayerCounts[player].Trophy
		silverDifference := playerCounts[player].Silver - oldPlayerCounts[player].Silver
		bronzeDifference := playerCounts[player].Bronze - oldPlayerCounts[player].Bronze

		// Construct the string with the difference in brackets
		trophyCount := fmt.Sprintf("%d", playerCounts[player].Trophy)
		if trophiesDifference > 0 {
			trophyCount += fmt.Sprintf(" (+%d)", trophiesDifference)
		}

		silverCount := fmt.Sprintf("%d", playerCounts[player].Silver)
		if silverDifference > 0 {
			silverCount += fmt.Sprintf(" (+%d)", silverDifference)
		}

		bronzeCount := fmt.Sprintf("%d", playerCounts[player].Bronze)
		if bronzeDifference > 0 {
			bronzeCount += fmt.Sprintf(" (+%d)", bronzeDifference)
		}

		ranks := other.Ranks(rank)

		// Write the leaderboard row
		_, err = fmt.Fprintf(file, "| %s %s| %s | %s | %s | %s | %.1f |\n", ranks, changeEmoji, player, trophyCount, silverCount, bronzeCount, totalPoints[player])
		if err != nil {
			return err
		}

		prevPoints = points // Update previous points
		prevRank = rank     // Update previous rank
	}

	return nil
}

// Function to write the Fish Week leaderboard with emojis indicating ranking change
func writeFishWeekLeaderboard(filePath string, setName string, maxFishInWeek map[string]PlayerInfo, fishCountThreshold int) error {
	// Call ReadOldFishRankings to get the old fish rankings
	oldFishRankings, oldFishCountWeek, err := other.ReadOldFishRankings(filePath)
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
	_, err = fmt.Fprintf(file, "### Most fish caught in a single week in tournaments in %s's chat\n", setName)
	if err != nil {
		return err
	}

	// Write the header
	_, err = fmt.Fprintln(file, "| Rank | Player | Fish Caught 🪣 |")
	_, err = fmt.Fprintln(file, "|------|--------|---------------|")
	if err != nil {
		return err
	}

	// Import the list from lists
	verifiedPlayers := lists.ReadVerifiedPlayers()

	// Calculate total fish caught for each player
	totalFishCaught := make(map[string]int)
	for player, info := range maxFishInWeek {
		totalFishCaught[player] = info.FishCount
	}

	// Sort players by total fish caught
	sortedPlayers := other.SortMapByValueDescInt(totalFishCaught)

	// Write the leaderboard data
	rank := 1
	prevRank := 1
	prevFishCount := -1
	occupiedRanks := make(map[int]int)

	for _, player := range sortedPlayers {
		fishCount := totalFishCaught[player]

		// Check if fish count is greater than or equal to 20
		if fishCount >= fishCountThreshold {

			// Increment rank only if the count has changed
			if fishCount != prevFishCount {
				rank += occupiedRanks[rank] // Increment rank by the number of occupied ranks
				// Reset the count of occupied ranks when count changes
				occupiedRanks[rank] = 1
			} else {
				// Set the rank to the previous rank if the count hasn't changed
				rank = prevRank
				occupiedRanks[rank]++ // Increment the count of occupied ranks
			}

			oldRank, found := oldFishRankings[player]
			changeEmoji := other.ChangeEmoji(rank, oldRank, found)

			// Construct the string with the difference in brackets
			fishweekDifference := fishCount - oldFishCountWeek[player]
			fishWeekCount := fmt.Sprintf("%d", fishCount)
			if fishweekDifference > 0 && fishweekDifference > fishCount {
				fishWeekCount += fmt.Sprintf(" (+%d)", fishweekDifference)
			}

			// Write the leaderboard row with change indication
			botIndicator := ""
			if maxFishInWeek[player].Bot == "supibot" && !other.Contains(verifiedPlayers, player) {
				botIndicator = "*"
			}

			ranks := other.Ranks(rank)

			_, err = fmt.Fprintf(file, "| %s %s| %s%s | %s |\n", ranks, changeEmoji, player, botIndicator, fishWeekCount)
			if err != nil {
				return err
			}

			prevFishCount = fishCount // Update previous fish count
			prevRank = rank           // Update previous rank
		}
	}

	// Write the note
	_, err = fmt.Fprintln(file, "\n_* = The fish were caught on supibot and the player did not migrate their data over to gofishgame. Because of that their data was not individually verified to be accurate._")
	if err != nil {
		return err
	}

	return nil
}
