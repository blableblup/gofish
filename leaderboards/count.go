package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func processCount(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	global := params.Global
	config := params.Config
	date2 := params.Date2
	title := params.Title
	chat := params.Chat
	pool := params.Pool
	date := params.Date
	path := params.Path

	fishCaught := make(map[string]data.FishInfo)
	var filePath, titletotalcount string

	if path == "" {
		filePath = filepath.Join("leaderboards", chatName, "count.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", chatName, path)
	}

	isFish := false
	oldCountRecord, err := ReadTotalcountRankings(filePath, pool, isFish)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error reading old leaderboard")
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
	  AND date < $2
	  AND date > $3
	  GROUP BY playerid
	  HAVING COUNT(*) >= $4`, chatName, date, date2, Totalcountlimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error querying database")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var fishInfo data.FishInfo

		if err := rows.Scan(&fishInfo.PlayerID, &fishInfo.Count); err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error scanning row for fish count")
			return
		}

		err := pool.QueryRow(context.Background(), "SELECT name, firstfishdate FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Player, &fishInfo.Date)
		if err != nil {
			logs.Logs().Error().Err(err).
				Int("PlayerID", fishInfo.PlayerID).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error retrieving player name for id")
			return
		}
		if fishInfo.Date.Before(time.Date(2023, time.September, 14, 0, 0, 0, 0, time.UTC)) {
			fishInfo.Bot = "supibot"
			err := pool.QueryRow(context.Background(), "SELECT verified FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Verified)
			if err != nil {
				logs.Logs().Error().Err(err).
					Int("PlayerID", fishInfo.PlayerID).
					Str("Chat", chatName).
					Str("Board", board).
					Msg("Error retrieving verified status for playerid")
				return
			}
		}

		fishCaught[fishInfo.Player] = fishInfo
	}

	if title == "" {
		if strings.HasSuffix(chatName, "s") {
			titletotalcount = fmt.Sprintf("### Most fish caught in %s' chat\n", chatName)
		} else {
			titletotalcount = fmt.Sprintf("### Most fish caught in %s's chat\n", chatName)
		}
	} else {
		titletotalcount = fmt.Sprintf("%s\n", title)
	}

	logs.Logs().Info().
		Str("Board", board).
		Str("Chat", chatName).
		Msg("Updating leaderboard")

	err = writeCount(filePath, fishCaught, oldCountRecord, titletotalcount, global, board, Totalcountlimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error writing leaderboard")
	} else {
		logs.Logs().Info().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Leaderboard updated successfully")
	}
}

func writeCount(filePath string, fishCaught map[string]data.FishInfo, oldCountRecord map[string]LeaderboardInfo, title string, global bool, board string, countlimit int) error {

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
	if board == "rare" {
		prefix = "| Rank | Fish | Times Caught |"
	}
	if board == "unique" || board == "uniqueglobal" {
		prefix = "| Rank | Fish | Fish Seen |"
	}

	_, _ = fmt.Fprintln(file, prefix+func() string {
		if global {
			return " Chat |"
		}
		return ""
	}())

	_, err = fmt.Fprintln(file, "|------|--------|-----------|"+func() string {
		if global {
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
		FishName := fishCaught[player].TypeName // For rarest fish leaderboard. For the count boards, this will print nothing

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

		_, _ = fmt.Fprintf(file, "| %s %s | %s%s %s | %s |", ranks, changeEmoji, player, botIndicator, FishName, counts)
		if global {
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
				_, _ = fmt.Fprintf(file, " %s %d ", count.chat, count.count)
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

	if board != "rare" {
		if board == "unique" || board == "uniqueglobal" {
			_, _ = fmt.Fprintf(file, "\n_Only showing fishers who have seen >= %d fish_\n", countlimit)
		} else {
			_, _ = fmt.Fprintf(file, "\n_Only showing fishers who caught >= %d fish_\n", countlimit)

		}
	}
	_, _ = fmt.Fprintf(file, "\n_Last updated at %s_", time.Now().In(time.UTC).Format("2006-01-02 15:04:05 UTC"))

	return nil
}
