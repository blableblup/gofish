package leaderboards

import (
	"bufio"
	"fmt"
	"gofish/playerdata"
	"gofish/utils"
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

func RunTournaments(chatNames, leaderboard string) {

	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		os.Exit(1)
	}

	// Construct the absolute path to the config file
	configFilePath := filepath.Join(wd, "config.json")

	// Load the config from the constructed file path
	config := utils.LoadConfig(configFilePath)

	// Define point values
	pointValues := map[string]float64{"Trophy": 3, "Silver": 1, "Bronze": 0.5}

	switch chatNames {
	case "all":
		// Process all chats
		for chatName, chat := range config.Chat {
			if !chat.CheckEnabled {
				fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
				continue // Skip processing if check_enabled is false
			}

			fmt.Printf("Checking chat '%s'.\n", chatName)
			processTournaments(chatName, chat, pointValues, leaderboard)
		}
	case "":
		fmt.Println("Please specify chat names.")
	default:
		// Process specified chat names
		specifiedchatNames := strings.Split(chatNames, ",")
		for _, chatName := range specifiedchatNames {
			chat, ok := config.Chat[chatName]
			if !ok {
				fmt.Printf("Chat '%s' not found in config.\n", chatName)
				continue
			}
			if !chat.CheckEnabled {
				fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
				continue // Skip processing if check_enabled is false
			}

			fmt.Printf("Checking chat '%s'.\n", chatName)
			processTournaments(chatName, chat, pointValues, leaderboard)
		}
	}
}

func processTournaments(chatName string, chat utils.ChatInfo, pointValues map[string]float64, leaderboard string) {

	renamedChatters := playerdata.ReadRenamedChatters()
	cheaters := playerdata.ReadCheaters()

	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		os.Exit(1)
	}
	// Open and read the logs file for the chat
	logsFilePath := filepath.Join(wd, "data", chat.Logs)
	logs, err := os.Open(logsFilePath)
	if err != nil {
		panic(err)
	}
	defer logs.Close()

	fishweekLimit := chat.Fishweeklimit
	if fishweekLimit == 0 {
		fishweekLimit = 20 // Set the default fishweek limit if not specified
	}

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

			// Change to the latest name
			newPlayer := renamedChatters[player]
			for newPlayer != "" {
				player = newPlayer
				newPlayer = renamedChatters[player]
			}

			if utils.Contains(cheaters, player) {
				continue // Skip processing for ignored players
			}

			// Get the amount of fish the player caught
			fishMatch := regexp.MustCompile(`(\d+) fish: (\w+)`).FindStringSubmatch(line)
			if len(fishMatch) > 0 {
				fishCount, _ := strconv.Atoi(fishMatch[1])
				botMatch := regexp.MustCompile(`#\w+ \s?(\w+):`).FindStringSubmatch(line)
				if len(botMatch) > 0 {
					bot := botMatch[1]

					// Update the record if the current fish count is greater
					if fishCount > maxFishInWeek[player].FishCount && fishCount >= fishweekLimit {
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

	// Titles for the leaderboards
	titlefishw := fmt.Sprintf("### Most fish caught in a single week in tournaments in %s's chat\n", chatName)
	titletrophies := fmt.Sprintf("### Leaderboard for the weekly tournaments in %s's chat\n", chatName)

	// Update only the specified leaderboard if the leaderboard flag is provided
	switch leaderboard {
	case "trophy":
		// Write the leaderboard for the weekly tournaments to the file specified in the config
		fmt.Printf("Updating trophies leaderboard for chat '%s'...\n", chatName)
		err = writeTrophiesLeaderboard(chat.Trophies, playerCounts, pointValues, titletrophies)
		if err != nil {
			fmt.Println("Error writing trophies leaderboard:", err)
		} else {
			fmt.Println("Trophies leaderboard updated successfully.")
		}
	case "fishw":
		// Write the leaderboard for the most fish caught in a single week in tournaments to the file specified in the config
		fmt.Printf("Updating fishweek leaderboard for chat '%s' with fish count threshold %d...\n", chatName, fishweekLimit)
		err = writeFishWeekLeaderboard(chat.Fishweek, maxFishInWeek, titlefishw)
		if err != nil {
			fmt.Println("Error writing fishweek leaderboard:", err)
		} else {
			fmt.Println("Fishweek leaderboard updated successfully.")
		}
	default:
		// If the leaderboard flag is not provided, update both leaderboards
		fmt.Printf("Updating trophies leaderboard for chat '%s'...\n", chatName)
		err = writeTrophiesLeaderboard(chat.Trophies, playerCounts, pointValues, titletrophies)
		if err != nil {
			fmt.Println("Error writing trophies leaderboard:", err)
		} else {
			fmt.Println("Trophies leaderboard updated successfully.")
		}

		fmt.Printf("Updating fishweek leaderboard for chat '%s' with fish count threshold %d...\n", chatName, fishweekLimit)
		err = writeFishWeekLeaderboard(chat.Fishweek, maxFishInWeek, titlefishw)
		if err != nil {
			fmt.Println("Error writing fishweek leaderboard:", err)
		} else {
			fmt.Println("Fishweek leaderboard updated successfully.")
		}
	}
}

// Function to write the Trophies leaderboard with emojis indicating ranking change and the change of trophies and medals in brackets
func writeTrophiesLeaderboard(filePath string, playerCounts map[string]PlayerCounts, pointValues map[string]float64, titletrophies string) error {
	// Call ReadOldTrophyRankings to get the trophy rankings
	oldLeaderboardTrophy, err := ReadOldTrophyRankings(filePath)
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
	_, err = fmt.Fprintf(file, "%s", titletrophies)
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
	sortedPlayers := utils.SortMapByValueDesc(totalPoints)

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

		// Declare found variable in the outer scope
		var found bool

		// Getting the old rank
		oldRank := -1 // Default value if the old rank is not found
		if info, ok := oldLeaderboardTrophy[player]; ok {
			found = true
			oldRank = info.Rank
		}

		changeEmoji := ChangeEmoji(rank, oldRank, found)

		// Compare new counts to old counts and display the difference
		trophiesDifference := playerCounts[player].Trophy - oldLeaderboardTrophy[player].Trophy
		silverDifference := playerCounts[player].Silver - oldLeaderboardTrophy[player].Silver
		bronzeDifference := playerCounts[player].Bronze - oldLeaderboardTrophy[player].Bronze

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

		ranks := Ranks(rank)

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
func writeFishWeekLeaderboard(filePath string, maxFishInWeek map[string]PlayerInfo, titlefishw string) error {
	// Call ReadOldFishRankings to get the old fish rankings
	oldLeaderboardFishW, err := ReadOldFishRankings(filePath)
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
	_, err = fmt.Fprintf(file, "%s", titlefishw)
	if err != nil {
		return err
	}

	// Write the header
	_, err = fmt.Fprintln(file, "| Rank | Player | Fish Caught 🪣 |")
	_, err = fmt.Fprintln(file, "|------|--------|---------------|")
	if err != nil {
		return err
	}

	verifiedPlayers := playerdata.ReadVerifiedPlayers()

	// Calculate total fish caught for each player
	totalFishCaught := make(map[string]int)
	for player, info := range maxFishInWeek {
		totalFishCaught[player] = info.FishCount
	}

	// Sort players by total fish caught
	sortedPlayers := utils.SortMapByValueDescInt(totalFishCaught)

	// Write the leaderboard data
	rank := 1
	prevRank := 1
	prevFishCount := -1
	occupiedRanks := make(map[int]int)

	for _, player := range sortedPlayers {
		fishCount := totalFishCaught[player]

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

		// Declare found variable in the outer scope
		var found bool

		// Getting the old rank
		oldRank := -1 // Default value if the old rank is not found
		if info, ok := oldLeaderboardFishW[player]; ok {
			found = true
			oldRank = info.Rank
		}

		changeEmoji := ChangeEmoji(rank, oldRank, found)

		// Construct the string with the difference in brackets
		fishweekDifference := fishCount - oldLeaderboardFishW[player].Count
		fishWeekCount := fmt.Sprintf("%d", fishCount)
		if fishweekDifference > 0 && oldLeaderboardFishW[player].Count > 0 {
			fishWeekCount += fmt.Sprintf(" (+%d)", fishweekDifference)
		}

		// Write the leaderboard row with change indication
		botIndicator := ""
		if maxFishInWeek[player].Bot == "supibot" && !utils.Contains(verifiedPlayers, player) {
			botIndicator = "*"
		}

		ranks := Ranks(rank)

		_, err = fmt.Fprintf(file, "| %s %s| %s%s | %s |\n", ranks, changeEmoji, player, botIndicator, fishWeekCount)
		if err != nil {
			return err
		}

		prevFishCount = fishCount // Update previous fish count
		prevRank = rank           // Update previous rank

	}

	// Write the note
	_, err = fmt.Fprintln(file, "\n_* = The fish were caught on supibot and the player did not migrate their data over to gofishgame. Because of that their data was not individually verified to be accurate._")
	if err != nil {
		return err
	}

	return nil
}
