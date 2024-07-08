package leaderboards

import (
	"context"
	"fmt"
	"gofish/logs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func processTrophy(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	date2 := params.Date2
	title := params.Title
	pool := params.Pool
	date := params.Date
	path := params.Path

	var filePath string

	if path == "" {
		filePath = filepath.Join("leaderboards", chatName, "trophy.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", chatName, path)
	}

	oldTrophy, err := ReadOldTrophyRankings(filePath, pool)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Chat", chatName).Str("Board", board).Msg("Error reading old leaderboard")
		return
	}

	playerCounts := make(map[string]LeaderboardInfo)

	// I get "ERROR: column \"date\" does not exist" with the old query ? This works but it is the exact same query (?) or I am stupid
	query := fmt.Sprintf(`
	SELECT 
		playerid,
		SUM(CASE WHEN placement IN (1) THEN 1 ELSE 0 END) AS trophycount,
		SUM(CASE WHEN placement IN (2) THEN 1 ELSE 0 END) AS silvercount,
		SUM(CASE WHEN placement IN (3) THEN 1 ELSE 0 END) AS bronzecount
	FROM (
		SELECT playerid, placement1 AS placement, date FROM tournaments%s
		UNION ALL
		SELECT playerid, placement2 AS placement, date FROM tournaments%s
		UNION ALL
		SELECT playerid, placement3 AS placement, date FROM tournaments%s
	) AS all_placements
	WHERE date < $1
	AND date > $2
	GROUP BY playerid
	HAVING (SUM(CASE WHEN placement IN (1) THEN 1 ELSE 0 END) + 
			SUM(CASE WHEN placement IN (2) THEN 1 ELSE 0 END) +
			SUM(CASE WHEN placement IN (3) THEN 1 ELSE 0 END)) > 0`, chatName, chatName, chatName)

	rows, err := pool.Query(context.Background(), query, date, date2)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Chat", chatName).Str("Board", board).Msg("Error querying database")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var playerid, trophycount, silvercount, bronzecount int

		if err := rows.Scan(&playerid, &trophycount, &silvercount, &bronzecount); err != nil {
			logs.Logs().Error().Err(err).Str("Chat", chatName).Str("Board", board).Msg("Error scanning row")
			return
		}

		var playerName string
		err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", playerid).Scan(&playerName)
		if err != nil {
			logs.Logs().Error().Err(err).Int("PlayerID", playerid).Str("Chat", chatName).Str("Board", board).Msg("Error retrieving player name for id")
			return
		}

		playerCounts[playerName] = LeaderboardInfo{
			Trophy: trophycount,
			Silver: silvercount,
			Bronze: bronzecount,
		}
	}

	if err := rows.Err(); err != nil {
		logs.Logs().Error().Str("Chat", chatName).Str("Board", board).Err(err).Msg("Error iterating over query results")
		return
	}

	var titletrophies string
	if title == "" {
		if strings.HasSuffix(chatName, "s") {
			titletrophies = fmt.Sprintf("### Leaderboard for the weekly tournaments in %s' chat\n", chatName)
		} else {
			titletrophies = fmt.Sprintf("### Leaderboard for the weekly tournaments in %s's chat\n", chatName)
		}
	} else {
		titletrophies = fmt.Sprintf("%s\n", title)
	}

	logs.Logs().Info().Str("Chat", chatName).Str("Board", board).Msg("Updating leaderboard")
	err = writeTrophy(filePath, playerCounts, oldTrophy, titletrophies)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Chat", chatName).Str("Board", board).Msg("Error writing leaderboard")
	} else {
		logs.Logs().Info().Str("Chat", chatName).Str("Board", board).Msg("Leaderboard updated successfully")
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
		oldtrophies := playerCounts[player].Trophy
		oldsilver := playerCounts[player].Silver
		oldbronze := playerCounts[player].Bronze
		oldpoints := totalPoints[player]
		if info, ok := oldTrophy[player]; ok {
			found = true
			oldRank = info.Rank
			oldtrophies = oldTrophy[player].Trophy
			oldsilver = oldTrophy[player].Silver
			oldbronze = oldTrophy[player].Bronze
			oldpoints = oldTrophy[player].Points
		}

		changeEmoji := ChangeEmoji(rank, oldRank, found)

		trophiesDifference := playerCounts[player].Trophy - oldtrophies
		silverDifference := playerCounts[player].Silver - oldsilver
		bronzeDifference := playerCounts[player].Bronze - oldbronze
		pointsDifference := totalPoints[player] - oldpoints

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

		newpoints := fmt.Sprintf("%.1f", totalPoints[player])
		if pointsDifference > 0 {
			newpoints += fmt.Sprintf(" (+%.1f)", pointsDifference)
		}

		ranks := Ranks(rank)

		_, err = fmt.Fprintf(file, "| %s %s| %s | %s | %s | %s | %s |\n", ranks, changeEmoji, player, trophyCount, silverCount, bronzeCount, newpoints)
		if err != nil {
			return err
		}

		prevPoints = points
		prevRank = rank
	}

	_, _ = fmt.Fprintf(file, "\n_Last updated at %s_", time.Now().In(time.UTC).Format("2006-01-02 15:04:05 UTC"))

	return nil
}
