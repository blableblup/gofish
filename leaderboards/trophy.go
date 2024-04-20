package leaderboards

import (
	"bufio"
	"fmt"
	"gofish/playerdata"
	"gofish/utils"
	"os"
	"path/filepath"
	"regexp"
)

func processTrophy(chatName string) {

	renamedChatters := playerdata.ReadRenamedChatters()
	cheaters := playerdata.ReadCheaters()

	wd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		os.Exit(1)
	}

	logsFilePath := filepath.Join(wd, "data", chatName, "tournamentlogs.txt")
	logs, err := os.Open(logsFilePath)
	if err != nil {
		panic(err)
	}
	defer logs.Close()

	playerCounts := make(map[string]LeaderboardInfo)

	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		line := scanner.Text()

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

			// Find all their medals and trophies
			achievements := regexp.MustCompile(`(Victory|champion|runner-up|third)`).FindAllString(line, -1)
			for _, achievement := range achievements {
				switch achievement {
				case "Victory", "champion":
					counts := playerCounts[player]
					counts.Trophy++

					playerCounts[player] = counts
				case "runner-up":
					counts := playerCounts[player]
					counts.Silver++

					playerCounts[player] = counts
				case "third":
					counts := playerCounts[player]
					counts.Bronze++

					playerCounts[player] = counts
				}
			}
		}
	}

	titletrophies := fmt.Sprintf("### Leaderboard for the weekly tournaments in %s's chat\n", chatName)
	filePath := filepath.Join("leaderboards", chatName, "trophy.md")

	fmt.Printf("Updating trophies leaderboard for chat '%s'...\n", chatName)
	err = writeTrophy(filePath, playerCounts, titletrophies)
	if err != nil {
		fmt.Println("Error writing trophies leaderboard:", err)
	} else {
		fmt.Println("Trophies leaderboard updated successfully.")
	}
}

func writeTrophy(filePath string, playerCounts map[string]LeaderboardInfo, titletrophies string) error {

	oldLeaderboardTrophy, err := ReadOldTrophyRankings(filePath)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(filePath), 0755)
	if err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "%s", titletrophies)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(file, "| Rank | Player | Trophies 🏆 | Silver Medals 🥈 | Bronze Medals 🥉 | Points |")
	_, _ = fmt.Fprintln(file, "|------|--------|-------------|------------------|------------------|--------|")

	totalPoints := make(map[string]float64)
	for player, counts := range playerCounts {
		totalPoints[player] = float64(counts.Trophy)*3 + float64(counts.Silver) + float64(counts.Bronze)*0.5
	}

	sortedPlayers := utils.SortMapByValueDesc(totalPoints)

	rank := 1
	prevRank := 1
	prevPoints := -1.0
	occupiedRanks := make(map[int]int)

	for _, player := range sortedPlayers {
		points := totalPoints[player]

		// Increment rank only if the count has changed
		if points != prevPoints {
			rank += occupiedRanks[rank]
			occupiedRanks[rank] = 1
		} else {
			rank = prevRank
			occupiedRanks[rank]++
		}

		var found bool

		oldRank := -1
		if info, ok := oldLeaderboardTrophy[player]; ok {
			found = true
			oldRank = info.Rank
		}

		changeEmoji := ChangeEmoji(rank, oldRank, found)

		trophiesDifference := playerCounts[player].Trophy - oldLeaderboardTrophy[player].Trophy
		silverDifference := playerCounts[player].Silver - oldLeaderboardTrophy[player].Silver
		bronzeDifference := playerCounts[player].Bronze - oldLeaderboardTrophy[player].Bronze

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

		_, err = fmt.Fprintf(file, "| %s %s| %s | %s | %s | %s | %.1f |\n", ranks, changeEmoji, player, trophyCount, silverCount, bronzeCount, totalPoints[player])
		if err != nil {
			return err
		}

		prevPoints = points
		prevRank = rank
	}

	return nil
}
