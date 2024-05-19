package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func processCount(params LeaderboardParams) {
	chatName := params.ChatName
	config := params.Config
	chat := params.Chat
	pool := params.Pool

	filePath := filepath.Join("leaderboards", chatName, "count.md")
	isFish := false
	oldCountRecord, err := ReadTotalcountRankings(filePath, pool, isFish)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error reading old count leaderboard")
		return
	}

	Totalcountlimit := chat.Totalcountlimit
	if Totalcountlimit == 0 {
		Totalcountlimit = config.Chat["default"].Totalcountlimit
	}

	// Query the database to get the count of fish caught by each player
	rows, err := pool.Query(context.Background(), `
	  SELECT playerid, COUNT(*) AS fish_count
	  FROM fish
	  WHERE chat = $1
	  GROUP BY playerid
	  HAVING COUNT(*) >= $2`, chatName, Totalcountlimit)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error querying database")
		return
	}
	defer rows.Close()

	fishCaught := make(map[string]data.FishInfo)

	for rows.Next() {
		var fishInfo data.FishInfo
		if err := rows.Scan(&fishInfo.PlayerID, &fishInfo.Count); err != nil {
			logs.Logs().Error().Err(err).Msg("Error scanning row")
			continue
		}

		err := pool.QueryRow(context.Background(), "SELECT name, firstfishdate FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Player, &fishInfo.Date)
		if err != nil {
			logs.Logs().Error().Err(err).Msgf("Error retrieving player name for id '%d'", fishInfo.PlayerID)
		}
		if fishInfo.Date.Before(time.Date(2023, time.September, 14, 0, 0, 0, 0, time.UTC)) {
			fishInfo.Bot = "supibot"
			err := pool.QueryRow(context.Background(), "SELECT verified FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Verified)
			if err != nil {
				logs.Logs().Error().Err(err).Msgf("Error retrieving verified status for playerid '%d'", fishInfo.PlayerID)
			}
		}

		fishCaught[fishInfo.Player] = fishInfo
	}

	titletotalcount := fmt.Sprintf("### Most fish caught in %s's chat\n", chatName)
	isGlobal, isType := false, false

	logs.Logs().Info().Msgf("Updating totalcount leaderboard for chat '%s' with count threshold %d...", chatName, Totalcountlimit)
	err = writeCount(filePath, fishCaught, oldCountRecord, titletotalcount, isGlobal, isType)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error writing totalcount leaderboard")
	} else {
		logs.Logs().Info().Msg("Totalcount leaderboard updated successfully.")
	}
}

func writeCount(filePath string, fishCaught map[string]data.FishInfo, oldCountRecord map[string]LeaderboardInfo, title string, isGlobal bool, isType bool) error {

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
		oldFishInfo, ok := oldCountRecord[player]
		if ok {
			found = true
			oldRank = oldFishInfo.Rank
			oldCount = oldFishInfo.Count
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
		if fishCaught[player].Bot == "supibot" && !fishCaught[player].Verified {
			botIndicator = "*"
		}

		ranks := Ranks(rank)

		_, _ = fmt.Fprintf(file, "| %s %s | %s%s | %s |", ranks, changeEmoji, player, botIndicator, counts)
		if isGlobal {
			// Turn the map to a slice
			ChatCountsSlice := make([]struct {
				chat  string
				count int
			}, 0, 2)

			for k, v := range ChatCounts {
				ChatCountsSlice = append(ChatCountsSlice, struct {
					chat  string
					count int
				}{k, v})
			}

			// Sort per-channel counts by channel
			sort.Slice(ChatCountsSlice, func(i, j int) bool {
				return ChatCountsSlice[i].chat < ChatCountsSlice[j].chat
			})

			// Print the count for each chat
			for _, count := range ChatCountsSlice {
				_, _ = fmt.Fprintf(file, " %s(%d) ", count.chat, count.count)
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

	_, _ = fmt.Fprintf(file, "\n_Last updated at %s_", time.Now().In(time.UTC).Format("2006-01-02 15:04:05 UTC"))

	return nil
}
