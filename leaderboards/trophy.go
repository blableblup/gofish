package leaderboards

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func processTrophy(params LeaderboardParams) {
	chatName := params.ChatName
	pool := params.Pool

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

	rows, err := pool.Query(context.Background(), `
		SELECT 
		playerid,
		SUM(CASE WHEN placement IN (1) THEN 1 ELSE 0 END) AS trophycount,
		SUM(CASE WHEN placement IN (2) THEN 1 ELSE 0 END) AS silvercount,
		SUM(CASE WHEN placement IN (3) THEN 1 ELSE 0 END) AS bronzecount
	FROM (
		SELECT playerid, placement1 AS placement FROM tournaments`+chatName+` UNION ALL
		SELECT playerid, placement2 AS placement FROM tournaments`+chatName+` UNION ALL
		SELECT playerid, placement3 AS placement FROM tournaments`+chatName+`
	) AS all_placements
	GROUP BY playerid
	HAVING (SUM(CASE WHEN placement IN (1) THEN 1 ELSE 0 END) + 
            SUM(CASE WHEN placement IN (2) THEN 1 ELSE 0 END) +
            SUM(CASE WHEN placement IN (3) THEN 1 ELSE 0 END)) > 0`)
	if err != nil {
		fmt.Println("Error querying database:", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var playerid, trophycount, silvercount, bronzecount int

		if err := rows.Scan(&playerid, &trophycount, &silvercount, &bronzecount); err != nil {
			fmt.Println("Error scanning row:", err)
			continue
		}

		var playerName string
		err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", playerid).Scan(&playerName)
		if err != nil {
			fmt.Printf("Error retrieving player name for id '%d':\n", playerid)
			continue
		}

		playerCounts[playerName] = LeaderboardInfo{
			Trophy: trophycount,
			Silver: silvercount,
			Bronze: bronzecount,
		}
	}

	if err := rows.Err(); err != nil {
		fmt.Println("Error iterating over query results:", err)
		return
	}

	titletrophies := fmt.Sprintf("### Leaderboard for the weekly tournaments in %s's chat\n", chatName)
	filePath := filepath.Join("leaderboards", chatName, "trophy.md")

	oldTrophy, err := ReadOldTrophyRankings(filePath, pool)
	if err != nil {
		fmt.Println("Error reading old trophy leaderboard:", err)
		return
	}

	fmt.Printf("Updating trophies leaderboard for chat '%s'...\n", chatName)
	err = writeTrophy(filePath, playerCounts, oldTrophy, titletrophies)
	if err != nil {
		fmt.Println("Error writing trophies leaderboard:", err)
	} else {
		fmt.Println("Trophies leaderboard updated successfully.")
	}
}

func writeTrophy(filePath string, playerCounts map[string]LeaderboardInfo, oldTrophy map[string]LeaderboardInfo, title string) error {

	// Ensure that the directory exists before attempting to create the file
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "%s", title)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(file, "| Rank | Player | Trophies 🏆 | Silver Medals 🥈 | Bronze Medals 🥉 | Points |")
	_, _ = fmt.Fprintln(file, "|------|--------|-------------|------------------|------------------|--------|")

	totalPoints := make(map[string]float64)
	for player, counts := range playerCounts {
		totalPoints[player] = float64(counts.Trophy)*3 + float64(counts.Silver) + float64(counts.Bronze)*0.5
	}

	sortedPlayers := SortMapByValueDesc(totalPoints)

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
		if info, ok := oldTrophy[player]; ok {
			found = true
			oldRank = info.Rank
		}

		changeEmoji := ChangeEmoji(rank, oldRank, found)

		trophiesDifference := playerCounts[player].Trophy - oldTrophy[player].Trophy
		silverDifference := playerCounts[player].Silver - oldTrophy[player].Silver
		bronzeDifference := playerCounts[player].Bronze - oldTrophy[player].Bronze

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
