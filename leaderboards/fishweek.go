package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"os"
	"path/filepath"
	"time"
)

func processFishweek(params LeaderboardParams) {
	chatName := params.ChatName
	chat := params.Chat
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

	fishweekLimit := chat.Fishweeklimit
	if fishweekLimit == 0 {
		fishweekLimit = 20 // Set the default fishweek limit if not specified
	}

	maxFishInWeek := make(map[string]data.FishInfo)

	rows, err := pool.Query(context.Background(), `
	SELECT t.playerid, t.fishcaught, t.bot, t.date
	FROM tournaments`+chatName+` t
	JOIN (
		SELECT playerid, MAX(fishcaught) AS max_count
		FROM tournaments`+chatName+`
		GROUP BY playerid
	) max_t ON t.playerid = max_t.playerid AND t.fishcaught = max_t.max_count
	WHERE t.chat = $1 AND max_count >= $2`, chatName, fishweekLimit)
	if err != nil {
		fmt.Println("Error querying database:", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var bot string
		var date time.Time
		var playerid, count int

		if err := rows.Scan(&playerid, &count, &bot, &date); err != nil {
			fmt.Println("Error scanning row:", err)
			continue
		}

		var playerName string
		err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", playerid).Scan(&playerName)
		if err != nil {
			fmt.Printf("Error retrieving player name for id '%d':\n", playerid)
			continue
		}

		maxFishInWeek[playerName] = data.FishInfo{
			Count: count,
			Bot:   bot,
			Date:  date,
		}
	}

	if err := rows.Err(); err != nil {
		fmt.Println("Error iterating over query results:", err)
		return
	}

	titlefishw := fmt.Sprintf("### Most fish caught in a single week in tournaments in %s's chat\n", chatName)
	filePath := filepath.Join("leaderboards", chatName, "fishweek.md")
	isGlobal, isType := false, false
	isFishw := true

	oldFishw, err := ReadTotalcountRankings(filePath, pool)
	if err != nil {
		fmt.Println("Error reading old fishweek leaderboard:", err)
		return
	}

	fmt.Printf("Updating fishweek leaderboard for chat '%s' with fish count threshold %d...\n", chatName, fishweekLimit)
	err = writeCount(filePath, maxFishInWeek, oldFishw, titlefishw, isGlobal, isType, isFishw)
	if err != nil {
		fmt.Println("Error writing fishweek leaderboard:", err)
	} else {
		fmt.Println("Fishweek leaderboard updated successfully.")
	}
}
