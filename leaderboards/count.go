package leaderboards

import (
	"context"
	"fmt"
	"gofish/playerdata"
	"gofish/utils"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v4/pgxpool"
)

func processCount(chatName string, chat utils.ChatInfo, pool *pgxpool.Pool) {
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

	fishCaught := make(map[string]int)
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

		fishCaught[playerName] = fishCount
	}

	titletotalcount := fmt.Sprintf("### Most fish caught in %s's chat\n", chatName)

	fmt.Printf("Updating totalcount leaderboard for chat '%s' with count threshold %d...\n", chatName, Totalcountlimit)
	err = writeCount(filePath, fishCaught, titletotalcount)
	if err != nil {
		fmt.Println("Error writing totalcount leaderboard:", err)
	} else {
		fmt.Println("Totalcount leaderboard updated successfully.")
	}
}

// Function to write the Totalcount leaderboard with emojis indicating ranking change and the count change in brackets
func writeCount(filePath string, fishCaught map[string]int, titletotalcount string) error {

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

	_, _ = fmt.Fprintln(file, "| Rank | Player | Fish Caught |")
	_, err = fmt.Fprintln(file, "|------|--------|-----------|")
	if err != nil {
		return err
	}

	verifiedPlayers := playerdata.ReadVerifiedPlayers()

	// Extract count from fishCount map
	fishCount := make(map[string]int)
	for player, count := range fishCaught {
		fishCount[player] = count
	}

	sortedPlayers := utils.SortMapByValueDescInt(fishCaught)

	rank := 1
	prevRank := 1
	prevCount := -1
	occupiedRanks := make(map[int]int)

	for _, player := range sortedPlayers {
		count := fishCaught[player]

		// Increment rank only if the count has changed
		if count != prevCount {
			rank += occupiedRanks[rank]
			occupiedRanks[rank] = 1
		} else {
			rank = prevRank
			occupiedRanks[rank]++
		}

		var found bool

		oldRank := -1
		if info, ok := oldLeaderboardCount[player]; ok {
			found = true
			oldRank = info.Rank
		}

		changeEmoji := utils.ChangeEmoji(rank, oldRank, found)

		oldCount := count
		if info, ok := oldLeaderboardCount[player]; ok {
			found = true
			oldCount = info.Count
		}

		var counts string

		countDifference := count - oldCount

		if countDifference > 0 {
			counts = fmt.Sprintf("%d (+%d)", count, countDifference)
		} else {
			counts = fmt.Sprintf("%d ", count)
		}

		oldBot := ""
		if info, ok := oldLeaderboardCount[player]; ok {
			found = true
			oldBot = info.Bot
		}

		botIndicator := ""
		if oldBot == "supibot" && !utils.Contains(verifiedPlayers, player) {
			botIndicator = "*"
		}

		ranks := utils.Ranks(rank)

		// Write the leaderboard row
		_, err := fmt.Fprintf(file, "| %s %s | %s%s | %s |\n", ranks, changeEmoji, player, botIndicator, counts)
		if err != nil {
			return err
		}

		prevCount = count
		prevRank = rank

	}

	_, err = fmt.Fprintln(file, "\n_* = The player caught their first fish on supibot and did not migrate their data to gofishgame. Because of that their data was not individually verified to be accurate._")
	if err != nil {
		return err
	}

	return nil
}
