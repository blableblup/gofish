package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/playerdata"
	"gofish/utils"
	"os"
	"path/filepath"
)

func processCount(params LeaderboardParams) {
	chatName := params.ChatName
	chat := params.Chat
	pool := params.Pool
	filePath := filepath.Join("leaderboards", chatName, "count.md")

	Totalcountlimit := chat.Totalcountlimit
	if Totalcountlimit == 0 {
		Totalcountlimit = 100 // Set the default count limit if not specified
	}

	// Query the database to get the count of fish caught by each player
	rows, err := pool.Query(context.Background(), `
	  SELECT playerid, COUNT(*) AS fish_count
	  FROM fish
	  WHERE chat = $1
	  GROUP BY playerid
	  HAVING COUNT(*) >= $2`, chatName, Totalcountlimit)
	if err != nil {
		fmt.Println("Error querying database:", err)
		return
	}
	defer rows.Close()

	fishCaught := make(map[string]data.FishInfo)
	// Iterate through the query results and store fish count for each player
	for rows.Next() {
		var playerID, fishCount int
		if err := rows.Scan(&playerID, &fishCount); err != nil {
			fmt.Println("Error scanning row:", err)
			continue
		}

		// Retrieve player name from the playerdata table
		var playerName string
		err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", playerID).Scan(&playerName)
		if err != nil {
			fmt.Println("Error retrieving player name:", err)
			continue
		}

		fishInfo := data.FishInfo{
			Player: playerName,
			Count:  fishCount,
		}

		fishCaught[playerName] = fishInfo
	}

	titletotalcount := fmt.Sprintf("### Most fish caught in %s's chat\n", chatName)
	isGlobal, isType, isFishw := false, false, false

	fmt.Printf("Updating totalcount leaderboard for chat '%s' with count threshold %d...\n", chatName, Totalcountlimit)
	err = writeCount(filePath, fishCaught, titletotalcount, isGlobal, isType, isFishw)
	if err != nil {
		fmt.Println("Error writing totalcount leaderboard:", err)
	} else {
		fmt.Println("Totalcount leaderboard updated successfully.")
	}
}

func writeCount(filePath string, fishCaught map[string]data.FishInfo, titletotalcount string, isGlobal bool, isType bool, isFishw bool) error {
	oldLeaderboardCount, err := ReadTotalcountRankings(filePath)
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

	_, err = fmt.Fprintf(file, "%s", titletotalcount)
	if err != nil {
		return err
	}

	prefix := "| Rank | Player | Fish Caught |"
	if isType {
		prefix = "| Rank | Fish | Times Caught |"
	}

	_, _ = fmt.Fprintln(file, prefix+func() string {
		if isGlobal {
			return " Chat |"
		}
		return ""
	}())

	_, err = fmt.Fprintln(file, "|------|--------|-----------|"+func() string {
		if isGlobal {
			return "-------|"
		}
		return ""
	}())
	if err != nil {
		return err
	}

	verifiedPlayers := playerdata.ReadVerifiedPlayers()

	sortedPlayers := SortMapByCountDesc(fishCaught)

	rank := 1
	prevRank := 1
	prevCount := -1
	occupiedRanks := make(map[int]int)

	for _, player := range sortedPlayers {
		Count := fishCaught[player].Count
		ChatCounts := fishCaught[player].ChatCounts

		// Increment rank only if the count has changed
		if Count != prevCount {
			rank += occupiedRanks[rank]
			occupiedRanks[rank] = 1
		} else {
			rank = prevRank
			occupiedRanks[rank]++
		}

		var found bool
		oldRank := -1
		oldCount := Count
		oldFishInfo, ok := oldLeaderboardCount[player]
		oldBot := ""
		if ok {
			found = true
			oldRank = oldFishInfo.Rank
			oldCount = oldFishInfo.Count
			oldBot = oldFishInfo.Bot
		}

		changeEmoji := ChangeEmoji(rank, oldRank, found)

		var counts string

		countDifference := Count - oldCount
		if countDifference > 0 {
			counts = fmt.Sprintf("%d (+%d)", Count, countDifference)
		} else {
			counts = fmt.Sprintf("%d", Count)
		}

		botIndicator := ""
		if oldBot == "supibot" && !utils.Contains(verifiedPlayers, player) {
			botIndicator = "*"
		}

		ranks := Ranks(rank)

		_, _ = fmt.Fprintf(file, "| %s %s | %s%s | %s |", ranks, changeEmoji, player, botIndicator, counts)
		if isGlobal {
			// Print the count for each chat
			for chat, count := range ChatCounts {
				_, _ = fmt.Fprintf(file, " %s(%d) ", chat, count)
			}
			_, _ = fmt.Fprint(file, "|")
		}
		_, err = fmt.Fprintln(file)
		if err != nil {
			return err
		}

		prevCount = Count
		prevRank = rank
	}

	if !isType && !isFishw {
		_, err = fmt.Fprintln(file, "\n_* = The player caught their first fish on supibot and did not migrate their data to gofishgame. Because of that their data was not individually verified to be accurate._")
		if err != nil {
			return err
		}
	}
	if isFishw {
		_, err = fmt.Fprintln(file, "\n_* = The fish were caught on supibot and the player did not migrate their data over to gofishgame. Because of that their data was not individually verified to be accurate._")
		if err != nil {
			return err
		}
	}

	return nil
}
