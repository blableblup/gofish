package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"gofish/playerdata"
	"gofish/utils"
	"os"
	"path/filepath"
	"time"
)

func processWeight(params LeaderboardParams) {
	chatName := params.ChatName
	config := params.Config
	mode := params.Mode
	pool := params.Pool
	chat := params.Chat

	filePath := filepath.Join("leaderboards", chatName, "weight.md")

	oldRecordWeight, err := ReadWeightRankings(filePath, pool)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error reading old weight leaderboard")
		return
	}

	Weightlimit := chat.Weightlimit
	if Weightlimit == 0 {
		Weightlimit = config.Chat["default"].Weightlimit
	}

	// Create maps to store updated records
	recordWeight, newRecordWeight := make(map[string]data.FishInfo), make(map[string]data.FishInfo)

	// Query the database to get the biggest fish per player for the specific chat
	rows, err := pool.Query(context.Background(), `
		SELECT f.playerid, f.weight, f.type AS fish_type, f.typename, f.bot, f.chat AS chatname, f.date, f.catchtype, f.fishid, f.chatid
		FROM fish f
		JOIN (
			SELECT playerid, MAX(weight) AS max_weight
			FROM fish 
			WHERE chat = $1
			GROUP BY playerid
		) max_fish ON f.playerid = max_fish.playerid AND f.weight = max_fish.max_weight
		WHERE f.chat = $1 AND f.weight >= $2`, chatName, Weightlimit)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error querying database")
		return
	}
	defer rows.Close()

	// Iterate through the query results
	for rows.Next() {
		var fishType, typeName, bot, catchtype, chatname string
		var date time.Time
		var playerid, fishid, chatid int
		var weight float64

		if err := rows.Scan(&playerid, &weight, &fishType, &typeName, &bot, &chatname, &date, &catchtype, &fishid, &chatid); err != nil {
			fmt.Println("Error scanning row:", err)
			continue
		}

		// Retrieve player name from the playerdata table
		var playerName string
		err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", playerid).Scan(&playerName)
		if err != nil {
			logs.Logs().Error().Err(err).Msgf("Error retrieving player name for id '%d'", playerid)
			continue
		}

		newRecordWeight[playerName] = data.FishInfo{
			Weight:    weight,
			Type:      fishType,
			TypeName:  typeName,
			Bot:       bot,
			Date:      date,
			CatchType: catchtype,
			Chat:      chatname,
			FishId:    fishid,
			ChatId:    chatid,
		}
	}

	if err := rows.Err(); err != nil {
		logs.Logs().Error().Err(err).Msg("Error iterating over query results")
		return
	}

	// Compare old weight records with new ones and update if necessary
	for playerName, newWeightRecord := range newRecordWeight {
		oldWeightRecord, exists := oldRecordWeight[playerName]
		if !exists {
			recordWeight[playerName] = newWeightRecord
			logs.Logs().Info().
				Str("Date", newWeightRecord.Date.Format(time.RFC3339)).
				Str("Chat", newWeightRecord.Chat).
				Float64("Weight", newWeightRecord.Weight).
				Str("TypeName", newWeightRecord.TypeName).
				Str("FishType", newWeightRecord.Type).
				Str("Player", playerName).
				Int("ChatID", newWeightRecord.ChatId).
				Int("FishID", newWeightRecord.FishId).
				Msg("New Record Weight for Player")
		} else {
			if newWeightRecord.Weight > oldWeightRecord.Weight {
				recordWeight[playerName] = newWeightRecord
				logs.Logs().Info().
					Str("Date", newWeightRecord.Date.Format(time.RFC3339)).
					Str("Chat", newWeightRecord.Chat).
					Float64("Weight", newWeightRecord.Weight).
					Str("TypeName", newWeightRecord.TypeName).
					Str("FishType", newWeightRecord.Type).
					Str("Player", playerName).
					Int("ChatID", newWeightRecord.ChatId).
					Int("FishID", newWeightRecord.FishId).
					Msg("Updated Record Weight for Player")
			} else {
				recordWeight[playerName] = ConvertToFishInfo(oldWeightRecord)
			}
		}
	}

	// Stops the program if it is in "just checking" mode
	if mode == "check" {
		logs.Logs().Info().Msgf("Finished checking for new weight records for chat '%s'", chatName)
		return
	}

	titleweight := fmt.Sprintf("### Biggest fish caught per player in %s's chat\n", chatName)
	isGlobal := false

	logs.Logs().Info().Msgf("Updating weight leaderboard for chat '%s' with weight threshold %f...", chatName, Weightlimit)
	err = writeWeight(filePath, recordWeight, oldRecordWeight, titleweight, isGlobal)
	if err != nil {
		logs.Logs().Error().Err(err).Msgf("Error writing weight leaderboard for chat '%s'", chatName)
	} else {
		logs.Logs().Info().Msgf("Weight leaderboard updated successfully for chat '%s'\n", chatName)
	}
}

func writeWeight(filePath string, recordWeight map[string]data.FishInfo, oldRecordWeight map[string]LeaderboardInfo, title string, isGlobal bool) error {

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

	_, _ = fmt.Fprintln(file, "| Rank | Player | Fish | Weight in lbs ⚖️ |"+func() string {
		if isGlobal {
			return " Chat |"
		}
		return ""
	}())
	_, err = fmt.Fprintln(file, "|------|--------|-----------|---------|"+func() string {
		if isGlobal {
			return "-------|"
		}
		return ""
	}())
	if err != nil {
		return err
	}

	verifiedPlayers := playerdata.ReadVerifiedPlayers()

	sortedPlayers := SortMapByWeightDesc(recordWeight)

	rank := 1
	prevRank := 1
	prevWeight := -1.0
	occupiedRanks := make(map[int]int)

	for _, player := range sortedPlayers {
		weight := recordWeight[player].Weight
		fishType := recordWeight[player].Type

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

		if info, ok := oldRecordWeight[player]; ok {
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
		if recordWeight[player].Bot == "supibot" && !utils.Contains(verifiedPlayers, player) {
			botIndicator = "*"
		}

		ranks := Ranks(rank)

		// Write the leaderboard row
		_, _ = fmt.Fprintf(file, "| %s %s | %s%s | %s | %s |", ranks, changeEmoji, player, botIndicator, fishType, fishweight)
		if isGlobal {
			_, _ = fmt.Fprintf(file, " %s |", recordWeight[player].Chat)
		}
		_, err = fmt.Fprintln(file)
		if err != nil {
			return err
		}

		prevWeight = weight
		prevRank = rank
	}

	_, err = fmt.Fprintln(file, "\n_* = The fish was caught on supibot and the player did not migrate their data over to gofishgame. Because of that their data was not individually verified to be accurate._")
	if err != nil {
		return err
	}

	return nil
}
