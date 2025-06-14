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
	title := params.Title
	mode := params.Mode

	filePath := returnPath(params)

	oldTrophy, err := getJsonBoard(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error reading old leaderboard")
		return
	}

	newTrophy, err := getTrophies(params)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error getting trophies")
		return
	}

	AreMapsSame := didPlayerMapsChange(params, oldTrophy, newTrophy)

	if AreMapsSame && mode != "force" {
		logs.Logs().Warn().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Not updating board because there are no changes")
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

	err = writeTrophy(filePath, newTrophy, oldTrophy, titletrophies)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error writing leaderboard")
		return
	} else {
		logs.Logs().Info().
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Leaderboard updated successfully")
	}
}

func getTrophies(params LeaderboardParams) (map[int]BoardData, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	date2 := params.Date2
	pool := params.Pool
	date := params.Date

	playerCounts := make(map[int]BoardData)

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
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error querying database")
		return playerCounts, err
	}
	defer rows.Close()

	for rows.Next() {
		var fishInfo BoardData

		if err := rows.Scan(&fishInfo.PlayerID, &fishInfo.Trophies, &fishInfo.Silver, &fishInfo.Bronze); err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error scanning row")
			return playerCounts, err
		}

		// date and verified arent needed here
		fishInfo.Player, _, _, _, err = PlayerStuff(fishInfo.PlayerID, params, pool)
		if err != nil {
			return playerCounts, err
		}

		fishInfo.Weight = float64(fishInfo.Trophies)*3 + float64(fishInfo.Silver) + float64(fishInfo.Bronze)*0.5

		playerCounts[fishInfo.PlayerID] = fishInfo
	}

	if err := rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error iterating over query results")
		return playerCounts, err
	}

	return playerCounts, nil
}

func writeTrophy(filePath string, playerCounts map[int]BoardData, oldTrophy map[int]BoardData, title string) error {

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

	sortedPlayers := sortMapIntFishInfo(playerCounts, "weightdesc")

	rank := 1
	prevRank := 1
	prevPoints := -1.0
	occupiedRanks := make(map[int]int)

	for _, playerID := range sortedPlayers {
		trophies := playerCounts[playerID].Trophies
		silver := playerCounts[playerID].Silver
		bronze := playerCounts[playerID].Bronze
		points := playerCounts[playerID].Weight
		player := playerCounts[playerID].Player

		// Increment rank only if the count has changed
		if points != prevPoints {
			rank += occupiedRanks[rank]
			occupiedRanks[rank] = 1
		} else {
			rank = prevRank
			occupiedRanks[rank]++
		}

		// Store the rank
		if ranksksk, ok := playerCounts[playerID]; ok {

			ranksksk.Rank = rank

			playerCounts[playerID] = ranksksk
		}

		var found bool

		oldRank := -1
		oldtrophies := trophies
		oldsilver := silver
		oldbronze := bronze
		oldpoints := points
		if info, ok := oldTrophy[playerID]; ok {
			found = true
			oldRank = info.Rank
			oldtrophies = oldTrophy[playerID].Trophies
			oldsilver = oldTrophy[playerID].Silver
			oldbronze = oldTrophy[playerID].Bronze
			oldpoints = oldTrophy[playerID].Weight
		}

		changeEmoji := ChangeEmoji(rank, oldRank, found)

		trophiesDifference := trophies - oldtrophies
		silverDifference := silver - oldsilver
		bronzeDifference := bronze - oldbronze
		pointsDifference := points - oldpoints

		trophyCount := fmt.Sprintf("%d", trophies)
		if trophiesDifference > 0 {
			trophyCount += fmt.Sprintf(" (+%d)", trophiesDifference)
		}

		silverCount := fmt.Sprintf("%d", silver)
		if silverDifference > 0 {
			silverCount += fmt.Sprintf(" (+%d)", silverDifference)
		}

		bronzeCount := fmt.Sprintf("%d", bronze)
		if bronzeDifference > 0 {
			bronzeCount += fmt.Sprintf(" (+%d)", bronzeDifference)
		}

		newpoints := fmt.Sprintf("%.1f", points)
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

	// This has to be here, because im not getting the rank directly from the query
	err = writeRaw(filePath, playerCounts)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Msg("Error writing raw leaderboard")
		return nil
	} else {
		logs.Logs().Info().
			Str("Path", filePath).
			Msg("Raw leaderboard updated successfully")
	}

	return nil
}
