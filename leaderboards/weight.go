package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/playerdata"
	"gofish/utils"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

func processWeight(chatName string, chat utils.ChatInfo, pool *pgxpool.Pool, mode string) {
	filePath := filepath.Join("leaderboards", chatName, "weight.md")

	oldRecordWeight, err := ReadWeightRankings(filePath)
	if err != nil {
		fmt.Println("Error reading old weight leaderboard:", err)
		return
	}

	Weightlimit := chat.Weightlimit
	if Weightlimit == 0 {
		Weightlimit = 200 // Set the default weight limit if not specified
	}

	// Create maps to store updated records
	recordWeight := make(map[string]data.FishInfo)
	newRecordWeight := make(map[string]data.FishInfo)

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
		fmt.Println("Error querying database:", err)
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
			fmt.Printf("Error retrieving player name for id '%d':\n", playerid)
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
		fmt.Println("Error iterating over query results:", err)
		return
	}

	// Compare old weight records with new ones and update if necessary
	for playerName, newWeightRecord := range newRecordWeight {
		// Check if player exists in the old weight leaderboard
		oldWeightRecord, exists := oldRecordWeight[playerName]
		if !exists {
			// If player doesn't exist in the old leaderboard, add the new record
			recordWeight[playerName] = newWeightRecord
			fmt.Println("New Record Weight for Player", playerName+":", newWeightRecord)
		} else {
			// If player exists in the old leaderboard, compare weights
			if newWeightRecord.Weight > oldWeightRecord.Weight {
				// If new weight is greater, update the leaderboard record
				recordWeight[playerName] = newWeightRecord
				fmt.Println("Updated Record Weight for Player", playerName+":", newWeightRecord)
			} else {
				// If new weight is not greater, keep the old record
				recordWeight[playerName] = ConvertToFishInfo(oldWeightRecord)
			}
		}
	}

	// Stops the program if it is in "just checking" mode
	if mode == "c" {
		fmt.Printf("Finished checking for new weight records for chat '%s'.\n", chatName)
		return
	}

	titleweight := fmt.Sprintf("### Biggest fish caught per player in %s's chat\n", chatName)
	isGlobal := false

	fmt.Printf("Updating weight leaderboard for chat '%s' with weight threshold %f...\n", chatName, Weightlimit)
	err = writeWeight(filePath, recordWeight, titleweight, isGlobal)
	if err != nil {
		fmt.Printf("Error writing weight leaderboard for chat '%s': %v\n", chatName, err)
	} else {
		fmt.Printf("Weight leaderboard updated successfully for chat '%s'\n", chatName)
	}
}

func writeWeight(filePath string, recordWeight map[string]data.FishInfo, titleweight string, isGlobal bool) error {

	oldLeaderboardWeight, err := ReadWeightRankings(filePath)
	if err != nil {
		return err
	}

	// Ensure that the directory exists before attempting to create the file
	err = os.MkdirAll(filepath.Dir(filePath), 0755)
	if err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "%s", titleweight)
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

		if info, ok := oldLeaderboardWeight[player]; ok {
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
