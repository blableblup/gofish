package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func processType(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	date2 := params.Date2
	title := params.Title
	pool := params.Pool
	date := params.Date
	path := params.Path
	mode := params.Mode

	var filePath string

	if path == "" {
		filePath = filepath.Join("leaderboards", chatName, "type.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", chatName, path)
	}

	oldType, err := ReadTypeRankings(filePath, pool)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Path", filePath).Str("Board", board).Msg("Error reading old leaderboard")
		return
	}

	recordType := make(map[string]data.FishInfo)

	// Query the database to get the biggest fish per type for the specific chat
	rows, err := pool.Query(context.Background(), `
		SELECT f.weight, f.fishname, f.bot, f.chat AS chatname, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
			SELECT fishname, MAX(weight) AS max_weight
			FROM fish 
			WHERE chat = $1
			AND date < $2
	  		AND date > $3
			GROUP BY fishname
		) AS sub
		ON f.fishname = sub.fishname AND f.weight = sub.max_weight
		WHERE f.chat = $1
		AND f.chatid = (
			SELECT MIN(chatid)
			FROM fish
			WHERE fishname = sub.fishname AND weight = sub.max_weight AND chat = $1
		)`, chatName, date, date2)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Chat", chatName).Str("Board", board).Msg("Error querying database")
		return
	}
	defer rows.Close()

	// Iterate through the query results
	for rows.Next() {
		var fishInfo data.FishInfo

		if err := rows.Scan(&fishInfo.Weight, &fishInfo.TypeName, &fishInfo.Bot,
			&fishInfo.Chat, &fishInfo.Date, &fishInfo.CatchType, &fishInfo.FishId, &fishInfo.ChatId, &fishInfo.PlayerID); err != nil {
			logs.Logs().Error().Err(err).Str("Chat", chatName).Str("Board", board).Msg("Error scanning row")
			return
		}

		// Retrieve player name from the playerdata table
		err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Player)
		if err != nil {
			logs.Logs().Error().Err(err).Int("PlayerID", fishInfo.PlayerID).Str("Chat", chatName).Str("Board", board).Msg("Error retrieving player name for id")
			return
		}

		if fishInfo.Bot == "supibot" {
			err := pool.QueryRow(context.Background(), "SELECT verified FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Verified)
			if err != nil {
				logs.Logs().Error().Err(err).Int("PlayerID", fishInfo.PlayerID).Str("Chat", chatName).Str("Board", board).Msg("Error retrieving verified status for playerid")
			}
		}

		err = pool.QueryRow(context.Background(), "SELECT fishtype FROM fishinfo WHERE fishname = $1", fishInfo.TypeName).Scan(&fishInfo.Type)
		if err != nil {
			logs.Logs().Error().Err(err).Str("FishName", fishInfo.TypeName).Str("Chat", chatName).Str("Board", board).Msg("Error retrieving fish type for fish name")
			return
		}

		recordType[fishInfo.Type] = fishInfo
	}

	if err := rows.Err(); err != nil {
		logs.Logs().Error().Str("Chat", chatName).Str("Board", board).Err(err).Msg("Error iterating over query results")
		return
	}

	// Compare old type records with new ones and update if necessary
	for fishType, newTypeRecord := range recordType {
		oldTypeRecord, exists := oldType[fishType]
		if !exists {
			logs.Logs().Info().
				Str("Date", newTypeRecord.Date.Format(time.RFC3339)).
				Str("Chat", newTypeRecord.Chat).
				Float64("Weight", newTypeRecord.Weight).
				Str("TypeName", newTypeRecord.TypeName).
				Str("CatchType", newTypeRecord.CatchType).
				Str("FishType", fishType).
				Str("Board", board).
				Str("Player", newTypeRecord.Player).
				Int("ChatID", newTypeRecord.ChatId).
				Int("FishID", newTypeRecord.FishId).
				Msg("New Record")
		} else {
			if newTypeRecord.Weight > oldTypeRecord.Weight {
				logs.Logs().Info().
					Str("Date", newTypeRecord.Date.Format(time.RFC3339)).
					Str("Chat", newTypeRecord.Chat).
					Float64("Weight", newTypeRecord.Weight).
					Float64("Old Weight", oldTypeRecord.Weight).
					Str("TypeName", newTypeRecord.TypeName).
					Str("CatchType", newTypeRecord.CatchType).
					Str("FishType", fishType).
					Str("Board", board).
					Str("Player", newTypeRecord.Player).
					Int("ChatID", newTypeRecord.ChatId).
					Int("FishID", newTypeRecord.FishId).
					Msg("Updated Record")
			}
		}
	}

	// Stops the program if it is in "just checking" mode
	if mode == "check" {
		logs.Logs().Info().Str("Chat", chatName).Str("Board", board).Msg("Finished checking for new records")
		return
	}

	var titletype string
	if title == "" {
		titletype = fmt.Sprintf("### Biggest fish per type caught in %s's chat\n", chatName)
	} else {
		titletype = fmt.Sprintf("%s\n", title)
	}

	isGlobal := false

	logs.Logs().Info().Str("Board", board).Str("Chat", chatName).Msg("Updating leaderboard")
	err = writeType(filePath, recordType, oldType, titletype, isGlobal)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Board", board).Str("Chat", chatName).Msg("Error writing leaderboard")
	} else {
		logs.Logs().Info().Str("Board", board).Str("Chat", chatName).Msg("Leaderboard updated successfully")
	}
}

func writeType(filePath string, recordType map[string]data.FishInfo, oldType map[string]LeaderboardInfo, title string, isGlobal bool) error {

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

	sortedTypes := SortMapByWeightDesc(recordType)

	rank := 1
	prevRank := 1
	prevWeight := -1.0
	occupiedRanks := make(map[int]int)

	for _, fishType := range sortedTypes {
		weight := recordType[fishType].Weight
		player := recordType[fishType].Player

		// Increment rank only if the count has changed
		if weight != prevWeight {
			rank += occupiedRanks[rank]
			occupiedRanks[rank] = 1
		} else {
			rank = prevRank
			occupiedRanks[rank]++
		}

		var found bool

		oldWeight := weight
		oldRank := -1

		if info, ok := oldType[fishType]; ok {
			found = true
			oldWeight = info.Weight
			oldRank = info.Rank
		}

		changeEmoji := ChangeEmoji(rank, oldRank, found)

		var fishweight string

		weightDifference := weight - oldWeight

		if weightDifference > 0 {
			fishweight = fmt.Sprintf("%.2f (+%.2f)", weight, weightDifference)
		} else {
			fishweight = fmt.Sprintf("%.2f", weight)
		}

		botIndicator := ""
		if recordType[fishType].Bot == "supibot" && !recordType[fishType].Verified {
			botIndicator = "*"
		}

		ranks := Ranks(rank)

		_, _ = fmt.Fprintf(file, "| %s %s | %s | %s | %s%s |", ranks, changeEmoji, fishType, fishweight, player, botIndicator)
		if isGlobal {
			_, _ = fmt.Fprintf(file, " %s |", recordType[fishType].ChatPfp)
		}
		_, err = fmt.Fprintln(file)
		if err != nil {
			return err
		}

		prevWeight = weight
		prevRank = rank
	}

	_, _ = fmt.Fprintf(file, "\n_Last updated at %s_", time.Now().In(time.UTC).Format("2006-01-02 15:04:05 UTC"))

	return nil
}
