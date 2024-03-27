package other

import (
	"bufio"
	"gofish/lists"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// PlayerInfo stores information about a player's rank and medals for the trophy leaderboard
type PlayerInfo struct {
	Rank   int
	Trophy int
	Silver int
	Bronze int
}

// Function to read and extract the old fish per week leaderboard from the leaderboard file
func ReadOldFishRankings(filePath string) (map[string]int, map[string]int, error) {
	oldFishRankings := make(map[string]int)
	oldFishCountWeek := make(map[string]int)
	renamedChatters := lists.ReadRenamedChatters()
	cheaters := lists.ReadCheaters()

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		// If the file doesn't exist, return empty rankings and counts
		return oldFishRankings, oldFishCountWeek, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	skipHeader := 0
	for scanner.Scan() {
		line := scanner.Text()
		if skipHeader < 3 {
			skipHeader++
			continue
		}
		if strings.HasPrefix(line, "|") {
			parts := strings.Split(line, "|")
			rankStr := strings.TrimSpace(parts[1])
			rank, err := strconv.Atoi(strings.Split(rankStr, " ")[0])
			if err != nil {
				continue
			}
			player := strings.TrimSpace(parts[2])
			if strings.Contains(player, "*") {
				player = strings.TrimRight(player, "*")
			}
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
			weekcountStr := strings.TrimSpace(parts[3])
			weekcount, err := strconv.Atoi(strings.Split(weekcountStr, " ")[0])
			oldFishCountWeek[player] = weekcount
			oldFishRankings[player] = rank
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return oldFishRankings, oldFishCountWeek, nil
}

// Function to read and extract the old trophies leaderboard from the leaderboard file
func ReadOldTrophyRankings(filePath string) (map[string]int, map[string]PlayerInfo, error) {
	oldTrophyRankings := make(map[string]int)
	oldPlayerCounts := make(map[string]PlayerInfo)
	renamedChatters := lists.ReadRenamedChatters()
	cheaters := lists.ReadCheaters()

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		// If the file doesn't exist, return empty rankings and counts
		return oldTrophyRankings, oldPlayerCounts, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	skipHeader := 0
	for scanner.Scan() {
		line := scanner.Text()
		if skipHeader < 3 {
			skipHeader++
			continue
		}
		if strings.HasPrefix(line, "|") {
			parts := strings.Split(line, "|")
			rankStr := strings.TrimSpace(parts[1])
			rank, err := strconv.Atoi(strings.Split(rankStr, " ")[0])
			if err != nil {
				continue
			}
			player := strings.TrimSpace(parts[2])
			if strings.Contains(player, "*") {
				player = strings.TrimRight(player, "*")
			}
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
			oldTrophyRankings[player] = rank

			trohpyStr := strings.TrimSpace(parts[3])
			trophies, err := strconv.Atoi(strings.Split(trohpyStr, " ")[0])
			silverMedalsStr := strings.TrimSpace(parts[4])
			silverMedals, err := strconv.Atoi(strings.Split(silverMedalsStr, " ")[0])
			bronzeMedalsStr := strings.TrimSpace(parts[5])
			bronzeMedals, err := strconv.Atoi(strings.Split(bronzeMedalsStr, " ")[0])

			oldPlayerCounts[player] = PlayerInfo{Trophy: trophies, Silver: silverMedals, Bronze: bronzeMedals}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return oldTrophyRankings, oldPlayerCounts, nil
}

// Function to read and extract the old weight leaderboard from the leaderboard file
func ReadWeightRankings(filePath string) (map[string]interface{}, error) {
	oldLeaderboardWeight := make(map[string]interface{})
	renamedChatters := lists.ReadRenamedChatters()
	cheaters := lists.ReadCheaters()

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		// If the file doesn't exist, return empty rankings and counts
		return oldLeaderboardWeight, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	skipHeader := 0
	for scanner.Scan() {
		line := scanner.Text()
		if skipHeader < 3 {
			skipHeader++
			continue
		}
		if strings.HasPrefix(line, "|") {
			parts := strings.Split(line, "|")
			rankStr := strings.TrimSpace(parts[1])
			rank, err := strconv.Atoi(strings.Split(rankStr, " ")[0])
			if err != nil {
				continue
			}
			player := strings.TrimSpace(parts[2])
			var bot string
			if strings.Contains(player, "*") {
				player = strings.TrimRight(player, "*")
				bot = "supibot"
			}
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
			fishType := strings.TrimSpace(parts[3])
			// Update fish type if it has an equivalent
			if equivalent := EquivalentFishType(fishType); equivalent != "" {
				fishType = equivalent
			}
			oldWeightStr := strings.TrimSpace(parts[4])
			re := regexp.MustCompile(`([0-9.]+)`) // Regular expression to match floating-point numbers
			matches := re.FindStringSubmatch(oldWeightStr)
			var oldweight float64 // Declare oldweight outside the if block
			if len(matches) >= 2 {
				var err error
				oldweight, err = strconv.ParseFloat(matches[1], 64)
				if err != nil {
					continue // Skip if unable to parse weight
				}
			} else {
				continue // Skip if unable to extract weight
			}

			oldLeaderboardWeight[player] = map[string]interface{}{
				"rank":   rank,
				"weight": oldweight,
				"type":   fishType,
				"bot":    bot,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return oldLeaderboardWeight, nil
}

// Function to read and extract the old type leaderboard from the leaderboard file
func ReadTypeRankings(filePath string) (map[string]interface{}, error) {
	oldLeaderboardType := make(map[string]interface{})
	renamedChatters := lists.ReadRenamedChatters()
	cheaters := lists.ReadCheaters()

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		// If the file doesn't exist, return empty rankings and counts
		return oldLeaderboardType, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	skipHeader := 0
	for scanner.Scan() {
		line := scanner.Text()
		if skipHeader < 3 {
			skipHeader++
			continue
		}
		if strings.HasPrefix(line, "|") {
			parts := strings.Split(line, "|")
			rankStr := strings.TrimSpace(parts[1])
			rank, err := strconv.Atoi(strings.Split(rankStr, " ")[0])
			if err != nil {
				continue
			}
			fishType := strings.TrimSpace(parts[2])
			// Update fish type if it has an equivalent
			if equivalent := EquivalentFishType(fishType); equivalent != "" {
				fishType = equivalent
			}
			player := strings.TrimSpace(parts[4])
			var bot string
			if strings.Contains(player, "*") {
				player = strings.TrimRight(player, "*")
				bot = "supibot"
			}
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
			oldWeightStr := strings.TrimSpace(parts[3])
			re := regexp.MustCompile(`([0-9.]+)`) // Regular expression to match floating-point numbers
			matches := re.FindStringSubmatch(oldWeightStr)
			var oldweight float64 // Declare oldweight outside the if block
			if len(matches) >= 2 {
				var err error
				oldweight, err = strconv.ParseFloat(matches[1], 64)
				if err != nil {
					continue // Skip if unable to parse weight
				}
			} else {
				continue // Skip if unable to extract weight
			}

			oldLeaderboardType[fishType] = map[string]interface{}{
				"weight": oldweight,
				"player": player,
				"bot":    bot,
				"rank":   rank,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return oldLeaderboardType, nil
}

// Function to read and extract the old totalcount leaderboard from the leaderboard file
func ReadTotalcountRankings(filePath string) (map[string]interface{}, error) {
	oldLeaderboardCount := make(map[string]interface{})
	renamedChatters := lists.ReadRenamedChatters()
	cheaters := lists.ReadCheaters()

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		// If the file doesn't exist, return empty rankings and counts
		return oldLeaderboardCount, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	skipHeader := 0
	for scanner.Scan() {
		line := scanner.Text()
		if skipHeader < 3 {
			skipHeader++
			continue
		}
		if strings.HasPrefix(line, "|") {
			parts := strings.Split(line, "|")
			rankStr := strings.TrimSpace(parts[1])
			rank, err := strconv.Atoi(strings.Split(rankStr, " ")[0])
			if err != nil {
				continue
			}
			player := strings.TrimSpace(parts[2])
			var bot string
			if strings.Contains(player, "*") {
				player = strings.TrimRight(player, "*")
				bot = "supibot"
			}
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
			countStr := strings.TrimSpace(parts[3])
			count, err := strconv.Atoi(strings.Split(countStr, " ")[0])

			oldLeaderboardCount[player] = map[string]interface{}{
				"count": count,
				"bot":   bot,
				"rank":  rank,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return oldLeaderboardCount, nil
}
