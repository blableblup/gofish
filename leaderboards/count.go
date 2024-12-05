package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func processCount(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	global := params.Global
	config := params.Config
	title := params.Title
	limit := params.Limit
	chat := params.Chat
	path := params.Path
	mode := params.Mode

	var filePath, titletotalcount string
	var countlimit int

	if path == "" {
		filePath = filepath.Join("leaderboards", chatName, "count.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", chatName, path)
	}

	oldCountRecord, err := getJsonBoard(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error reading old leaderboard")
		return
	}

	if limit == "" {
		countlimit = chat.Totalcountlimit
		if countlimit == 0 {
			countlimit = config.Chat["default"].Totalcountlimit
		}
	} else {
		countlimit, err = strconv.Atoi(limit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Limit", limit).
				Str("Board", board).
				Msg("Error converting custom limit to int")
			return
		}
	}

	fishCaught, err := getCount(params, countlimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error getting leaderboard")
		return
	}

	AreMapsSame := didPlayerMapsChange(params, oldCountRecord, fishCaught)

	if AreMapsSame && mode != "force" {
		logs.Logs().Warn().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Not updating board because there are no changes")
		return
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

	err = writeCount(filePath, fishCaught, oldCountRecord, titletotalcount, global, board, countlimit)
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

func getCount(params LeaderboardParams, countlimit int) (map[int]data.FishInfo, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	date2 := params.Date2
	pool := params.Pool
	date := params.Date

	fishCaught := make(map[int]data.FishInfo)

	// Query the database to get the count of fish caught by each player
	rows, err := pool.Query(context.Background(), `
		SELECT playerid, COUNT(*) AS fish_count
		FROM fish
		WHERE chat = $1
		AND date < $2
		AND date > $3
		GROUP BY playerid
		HAVING COUNT(*) >= $4`, chatName, date, date2, countlimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error querying database")
		return fishCaught, err
	}
	defer rows.Close()

	for rows.Next() {
		var fishInfo data.FishInfo

		if err := rows.Scan(&fishInfo.PlayerID, &fishInfo.Count); err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error scanning row for fish count")
			return fishCaught, err
		}

		err := pool.QueryRow(context.Background(), "SELECT name, firstfishdate FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Player, &fishInfo.Date)
		if err != nil {
			logs.Logs().Error().Err(err).
				Int("PlayerID", fishInfo.PlayerID).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error retrieving player name for id")
			return fishCaught, err
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
				return fishCaught, err
			}
		}

		fishCaught[fishInfo.PlayerID] = fishInfo
	}

	return fishCaught, nil
}

func writeCount(filePath string, fishCaught map[int]data.FishInfo, oldCountRecord map[int]data.FishInfo, title string, global bool, board string, countlimit int) error {

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

	sortedPlayers := sortPlayerRecords(fishCaught)

	rank := 1
	prevRank := 1
	prevCount := -1
	occupiedRanks := make(map[int]int)

	for _, playerID := range sortedPlayers {
		Player := fishCaught[playerID].Player
		Count := fishCaught[playerID].Count
		ChatCounts := fishCaught[playerID].ChatCounts
		FishName := fishCaught[playerID].TypeName

		// Increment rank only if the count has changed
		if Count != prevCount {
			rank += occupiedRanks[rank]
			occupiedRanks[rank] = 1
		} else {
			rank = prevRank
			occupiedRanks[rank]++
		}

		// Store the rank
		if ranksksk, ok := fishCaught[playerID]; ok {

			ranksksk.Rank = rank

			fishCaught[playerID] = ranksksk
		}

		var found bool
		oldRank := -1
		oldCount := Count
		oldFishInfo, ok := oldCountRecord[playerID]
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
		if fishCaught[playerID].Bot == "supibot" && !fishCaught[playerID].Verified {
			botIndicator = "*"
		}

		ranks := Ranks(rank)

		_, _ = fmt.Fprintf(file, "| %s %s | %s%s %s | %s |", ranks, changeEmoji, Player, botIndicator, FishName, counts)
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

	if board == "unique" || board == "uniqueglobal" {
		_, _ = fmt.Fprintf(file, "\n_Only showing fishers who have seen >= %d fish_\n", countlimit)
	} else {
		_, _ = fmt.Fprintf(file, "\n_Only showing fishers who caught >= %d fish_\n", countlimit)

	}

	_, _ = fmt.Fprintf(file, "\n_Last updated at %s_", time.Now().In(time.UTC).Format("2006-01-02 15:04:05 UTC"))

	// This has to be here, because im not getting the rank directly from the query
	err = writeRaw(filePath, fishCaught)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Msg("Error writing raw leaderboard")
		return nil
	} else {
		logs.Logs().Info().
			Str("Path", filePath).
			Msg("Raw leaderboard updated successfully")
	}

	return nil
}
