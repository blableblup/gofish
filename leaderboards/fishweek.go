package leaderboards

import (
	"bufio"
	"fmt"
	"gofish/data"
	"gofish/playerdata"
	"gofish/utils"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

func processFishweek(params LeaderboardParams) {
	chatName := params.ChatName
	chat := params.Chat

	renamedChatters := playerdata.ReadRenamedChatters()
	cheaters := playerdata.ReadCheaters()

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

	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		line := scanner.Text()

		playerMatch := regexp.MustCompile(`[@👥]\s?(\w+)`).FindStringSubmatch(line)
		if len(playerMatch) > 0 {
			player := playerMatch[1]

			// Change to the latest name
			newPlayer := renamedChatters[player]
			for newPlayer != "" {
				player = newPlayer
				newPlayer = renamedChatters[player]
			}

			if utils.Contains(cheaters, player) {
				continue // Skip processing for ignored players
			}

			// Get the amount of fish the player caught
			fishMatch := regexp.MustCompile(`(\d+) fish: (\w+)`).FindStringSubmatch(line)
			if len(fishMatch) > 0 {
				fishCount, _ := strconv.Atoi(fishMatch[1])
				botMatch := regexp.MustCompile(`#\w+ \s?(\w+):`).FindStringSubmatch(line)
				if len(botMatch) > 0 {
					bot := botMatch[1]

					// Update the record if the current fish count is greater
					if fishCount > maxFishInWeek[player].Count && fishCount >= fishweekLimit {
						maxFishInWeek[player] = data.FishInfo{Count: fishCount, Bot: bot}
					}
				}
			}
		}
	}

	titlefishw := fmt.Sprintf("### Most fish caught in a single week in tournaments in %s's chat\n", chatName)
	filePath := filepath.Join("leaderboards", chatName, "fishweek.md")
	isGlobal, isType := false, false
	isFishw := true

	fmt.Printf("Updating fishweek leaderboard for chat '%s' with fish count threshold %d...\n", chatName, fishweekLimit)
	err = writeCount(filePath, maxFishInWeek, titlefishw, isGlobal, isType, isFishw)
	if err != nil {
		fmt.Println("Error writing fishweek leaderboard:", err)
	} else {
		fmt.Println("Fishweek leaderboard updated successfully.")
	}
}
