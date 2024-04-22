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
)

func processType(params LeaderboardParams) {
	chatName := params.ChatName
	mode := params.Mode
	pool := params.Pool

	filePath := filepath.Join("leaderboards", chatName, "type.md")

	oldRecordType, err := ReadTypeRankings(filePath)
	if err != nil {
		fmt.Println("Error reading old type leaderboard:", err)
		return
	}

	// Create maps to store updated records
	recordType, newRecordType := make(map[string]data.FishInfo), make(map[string]data.FishInfo)

	// Query the database to get the biggest fish per type for the specific chat
	rows, err := pool.Query(context.Background(), `
		SELECT f.type AS fish_type, f.weight, f.typename, f.bot, f.chat AS chatname, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
			SELECT type, MAX(weight) AS max_weight
			FROM fish 
			WHERE chat = $1
			GROUP BY type
		) AS sub
		ON f.type = sub.type AND f.weight = sub.max_weight
		WHERE f.chat = $1
		AND f.chatid = (
			SELECT MIN(chatid)
			FROM fish
			WHERE type = sub.type AND weight = sub.max_weight AND chat = $1
		)`, chatName)
	if err != nil {
		fmt.Println("Error querying database:", err)
		return
	}
	defer rows.Close()

	// Iterate through the query results
	for rows.Next() {
		var fishType, typeName, bot, catchtype, chatname string
		var date time.Time
		var fishid, chatid, playerid int
		var weight float64

		if err := rows.Scan(&fishType, &weight, &typeName, &bot, &chatname, &date, &catchtype, &fishid, &chatid, &playerid); err != nil {
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

		newRecordType[fishType] = data.FishInfo{
			Weight:    weight,
			Player:    playerName,
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

	// Compare old type records with new ones and update if necessary
	for fishType, newTypeRecord := range newRecordType {
		oldTypeRecord, exists := oldRecordType[fishType]
		if !exists {
			recordType[fishType] = newTypeRecord
			fmt.Println("New Record Record Type for Fish Type", fishType+":", newTypeRecord)
		} else {
			if newTypeRecord.Weight > oldTypeRecord.Weight {
				recordType[fishType] = newTypeRecord
				fmt.Println("Updated Record Type for Fish Type", fishType+":", newTypeRecord)
			} else {
				recordType[fishType] = ConvertToFishInfo(oldTypeRecord)
			}
		}
	}

	// Stops the program if it is in "just checking" mode
	if mode == "check" {
		fmt.Printf("Finished checking for new type records for chat '%s'.\n", chatName)
		return
	}

	titletype := fmt.Sprintf("### Biggest fish per type caught in %s's chat\n", chatName)
	isGlobal := false

	fmt.Printf("Updating type leaderboard for chat '%s'...\n", chatName)
	err = writeType(filePath, recordType, titletype, isGlobal)
	if err != nil {
		fmt.Println("Error writing type leaderboard:", err)
	} else {
		fmt.Println("Type leaderboard updated successfully.")
	}
}

func writeType(filePath string, recordType map[string]data.FishInfo, titletype string, isGlobal bool) error {

	oldLeaderboardType, err := ReadTypeRankings(filePath)
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

	_, err = fmt.Fprintf(file, "%s", titletype)
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

	verifiedPlayers := playerdata.ReadVerifiedPlayers()

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

		if info, ok := oldLeaderboardType[fishType]; ok {
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
		if recordType[fishType].Bot == "supibot" && !utils.Contains(verifiedPlayers, player) {
			botIndicator = "*"
		}

		ranks := Ranks(rank)

		_, _ = fmt.Fprintf(file, "| %s %s | %s | %s | %s%s |", ranks, changeEmoji, fishType, fishweight, player, botIndicator)
		if isGlobal {
			_, _ = fmt.Fprintf(file, " %s |", recordType[fishType].Chat)
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
