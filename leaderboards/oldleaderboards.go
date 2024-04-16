package leaderboards

import (
	"bufio"
	"gofish/data"
	"gofish/playerdata"
	"gofish/utils"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Storing data from the old leaderboards
type LeaderboardInfo struct {
	Trophy int
	Silver int
	Bronze int
	Rank   int
	Count  int
	Weight float64
	Type   string
	Bot    string
	Player string
}

// Function to read and extract the old fish per week leaderboard from the leaderboard file
func ReadOldFishRankings(filePath string) (map[string]LeaderboardInfo, error) {
	oldLeaderboardFishW := make(map[string]LeaderboardInfo)
	renamedChatters := playerdata.ReadRenamedChatters()
	cheaters := playerdata.ReadCheaters()

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		// If the file doesn't exist, return empty rankings and counts
		return oldLeaderboardFishW, nil
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

			// Change to the latest name
			newPlayer := renamedChatters[player]
			for newPlayer != "" {
				player = newPlayer
				newPlayer = renamedChatters[player]
			}

			if utils.Contains(cheaters, player) {
				continue // Skip processing for ignored players
			}

			weekcountStr := strings.TrimSpace(parts[3])
			weekcount, err := strconv.Atoi(strings.Split(weekcountStr, " ")[0])

			oldLeaderboardFishW[player] = LeaderboardInfo{
				Rank:  rank,
				Count: weekcount,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return oldLeaderboardFishW, nil
}

// Function to read and extract the old trophies leaderboard from the leaderboard file
func ReadOldTrophyRankings(filePath string) (map[string]LeaderboardInfo, error) {
	oldLeaderboardTrophy := make(map[string]LeaderboardInfo)
	renamedChatters := playerdata.ReadRenamedChatters()
	cheaters := playerdata.ReadCheaters()

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		// If the file doesn't exist, return empty rankings and counts
		return oldLeaderboardTrophy, nil
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

			// Change to the latest name
			newPlayer := renamedChatters[player]
			for newPlayer != "" {
				player = newPlayer
				newPlayer = renamedChatters[player]
			}

			if utils.Contains(cheaters, player) {
				continue // Skip processing for ignored players
			}

			trohpyStr := strings.TrimSpace(parts[3])
			trophies, err := strconv.Atoi(strings.Split(trohpyStr, " ")[0])
			silverMedalsStr := strings.TrimSpace(parts[4])
			silverMedals, err := strconv.Atoi(strings.Split(silverMedalsStr, " ")[0])
			bronzeMedalsStr := strings.TrimSpace(parts[5])
			bronzeMedals, err := strconv.Atoi(strings.Split(bronzeMedalsStr, " ")[0])

			oldLeaderboardTrophy[player] = LeaderboardInfo{
				Rank:   rank,
				Trophy: trophies,
				Silver: silverMedals,
				Bronze: bronzeMedals,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return oldLeaderboardTrophy, nil
}

// Function to read and extract the old weight leaderboard from the leaderboard file
func ReadWeightRankings(filePath string) (map[string]LeaderboardInfo, error) {
	oldLeaderboardWeight := make(map[string]LeaderboardInfo)
	renamedChatters := playerdata.ReadRenamedChatters()
	cheaters := playerdata.ReadCheaters()

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

			// Change to the latest name
			newPlayer := renamedChatters[player]
			for newPlayer != "" {
				player = newPlayer
				newPlayer = renamedChatters[player]
			}

			if utils.Contains(cheaters, player) {
				continue // Skip processing for ignored players
			}

			fishType := strings.TrimSpace(parts[3])
			// Update fish type if it has an equivalent
			if equivalent := data.EquivalentFishType(fishType); equivalent != "" {
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

			oldLeaderboardWeight[player] = LeaderboardInfo{
				Rank:   rank,
				Weight: oldweight,
				Type:   fishType,
				Bot:    bot,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return oldLeaderboardWeight, nil
}

// Function to read and extract the old type leaderboard from the leaderboard file
func ReadTypeRankings(filePath string) (map[string]LeaderboardInfo, error) {
	oldLeaderboardType := make(map[string]LeaderboardInfo)
	renamedChatters := playerdata.ReadRenamedChatters()
	cheaters := playerdata.ReadCheaters()

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
			if equivalent := data.EquivalentFishType(fishType); equivalent != "" {
				fishType = equivalent
			}
			player := strings.TrimSpace(parts[4])
			var bot string
			if strings.Contains(player, "*") {
				player = strings.TrimRight(player, "*")
				bot = "supibot"
			}

			// Change to the latest name
			newPlayer := renamedChatters[player]
			for newPlayer != "" {
				player = newPlayer
				newPlayer = renamedChatters[player]
			}

			if utils.Contains(cheaters, player) {
				continue // Skip processing for ignored players
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

			oldLeaderboardType[fishType] = LeaderboardInfo{
				Rank:   rank,
				Weight: oldweight,
				Player: player,
				Bot:    bot,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return oldLeaderboardType, nil
}

// Function to read and extract the old totalcount leaderboard from the leaderboard file
func ReadTotalcountRankings(filePath string) (map[string]LeaderboardInfo, error) {
	oldLeaderboardCount := make(map[string]LeaderboardInfo)
	renamedChatters := playerdata.ReadRenamedChatters()
	cheaters := playerdata.ReadCheaters()

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

			// Change to the latest name
			newPlayer := renamedChatters[player]
			for newPlayer != "" {
				player = newPlayer
				newPlayer = renamedChatters[player]
			}

			if utils.Contains(cheaters, player) {
				continue // Skip processing for ignored players
			}

			countStr := strings.TrimSpace(parts[3])
			count, err := strconv.Atoi(strings.Split(countStr, " ")[0])

			oldLeaderboardCount[player] = LeaderboardInfo{
				Rank:  rank,
				Count: count,
				Bot:   bot,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return oldLeaderboardCount, nil
}
