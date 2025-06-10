package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

func processCount(params LeaderboardParams) {
	board := params.LeaderboardType
	boardInfo := params.BoardInfo
	chatName := params.ChatName
	global := params.Global
	config := params.Config
	limit := params.Limit
	chat := params.Chat
	mode := params.Mode

	filePath := returnPath(params)

	oldCountRecord, err := getJsonBoard(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error reading old leaderboard")
		return
	}

	var countlimit int

	if limit == "" {
		// idk how else to access the different limits per board through the config? whatever
		switch board {
		default:
			logs.Logs().Warn().
				Str("Board", board).
				Msg("NO LIMIT for board defined!")

		case "count":
			countlimit = chat.Totalcountlimit
			if countlimit == 0 {
				countlimit = config.Chat["default"].Totalcountlimit
			}

		case "fishweek":
			countlimit = chat.Fishweeklimit
			if countlimit == 0 {
				countlimit = config.Chat["default"].Fishweeklimit
			}

		case "uniquefish":
			countlimit = chat.Uniquelimit
			if countlimit == 0 {
				countlimit = config.Chat["default"].Uniquelimit
			}
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

	fishCaught, err := boardInfo.GetFunctionInt(params, countlimit)
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

	var title string

	if params.Title == "" {
		title = boardInfo.GetTitleFunction(params)
	} else {
		title = fmt.Sprintf("%s\n", params.Title)
	}

	logs.Logs().Info().
		Str("Board", board).
		Str("Chat", chatName).
		Msg("Updating leaderboard")

	err = writeCount(filePath, fishCaught, oldCountRecord, title, global, board, countlimit)
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
	boardInfo := params.BoardInfo
	chatName := params.ChatName
	global := params.Global
	date2 := params.Date2
	pool := params.Pool
	date := params.Date

	fishCaught := make(map[int]data.FishInfo)
	var rows pgx.Rows
	var err error

	queries := boardInfo.GetQueryFunctionMap(params)

	if !global {
		rows, err = pool.Query(context.Background(), queries["1"], chatName, date, date2, countlimit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error querying database")
			return fishCaught, err
		}
	} else {
		rows, err = pool.Query(context.Background(), queries["1"], date, date2, countlimit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error querying database")
			return fishCaught, err
		}
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[data.FishInfo])
	if err != nil && err != pgx.ErrNoRows {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error collecting rows")
		return fishCaught, err
	}

	var players []int

	for _, result := range results {

		result.Player, result.Date, result.Verified, _, err = PlayerStuff(result.PlayerID, params, pool)
		if err != nil {
			return fishCaught, err
		}

		// because fishweek gets bot from the query
		// to see when that tournemant week result happened
		if board != "fishweek" {
			if result.Date.Before(time.Date(2023, time.September, 14, 0, 0, 0, 0, time.UTC)) {
				result.Bot = "supibot"
			}
		}

		players = append(players, result.PlayerID)

		fishCaught[result.PlayerID] = result
	}

	if global {
		// Get the fish caught per chat for the chatters above the countlimit
		rows, err = pool.Query(context.Background(), queries["2"], players, date, date2)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error querying database")
			return fishCaught, err
		}

		results, err = pgx.CollectRows(rows, pgx.RowToStructByNameLax[data.FishInfo])
		if err != nil && err != pgx.ErrNoRows {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error collecting rows")
			return fishCaught, err
		}

		for _, result := range results {

			pfp := fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", result.Chat, result.Chat)

			existingFishInfo, exists := fishCaught[result.PlayerID]
			if exists {

				if existingFishInfo.ChatCounts == nil {
					existingFishInfo.ChatCounts = make(map[string]int)
				}
				existingFishInfo.ChatCounts[pfp] += result.Count

				fishCaught[result.PlayerID] = existingFishInfo
			}
		}

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

	if board == "uniquefish" {
		prefix = "| Rank | Player | Fish Seen |"
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

	sortedPlayers := sortMapIntFishInfo(fishCaught, "countdesc")

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

		// update the rank
		if copy, ok := fishCaught[playerID]; ok {

			copy.Rank = rank

			fishCaught[playerID] = copy
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
			sortedChatCounts := sortMapStringInt(ChatCounts, "nameasc")

			for _, chat := range sortedChatCounts {
				_, _ = fmt.Fprintf(file, " %s %d ", chat, ChatCounts[chat])
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

	if board == "uniquefish" {
		_, _ = fmt.Fprint(file, "\n_This does not include fish seen through 🎁 gifts or through releasing to another player during the winter events!_\n")
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
