package leaderboards

import (
	"context"
	"fmt"
	"gofish/logs"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
)

func RunCountFishTypesGlobal(params LeaderboardParams) {
	board := params.LeaderboardType
	title := params.Title
	mode := params.Mode

	filePath := returnPath(params)

	oldCount, err := getJsonBoardString(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error reading old leaderboard")
		return
	}

	globalFishTypesCount, err := getRarestFish(params)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error getting leaderboard")
		return
	}

	// This board should always have changes unless game doid
	AreMapsSame := didFishMapChange(params, oldCount, globalFishTypesCount)

	if AreMapsSame && mode != "force" {
		logs.Logs().Warn().
			Str("Board", board).
			Msg("Not updating board because there are no changes")
		return
	}

	var titlerare string

	if title == "" {
		titlerare = "### How many times a fish has been caught\n"
	} else {
		titlerare = fmt.Sprintf("%s\n", title)
	}

	logs.Logs().Info().
		Str("Board", board).
		Msg("Updating leaderboard")

	err = writeRare(filePath, globalFishTypesCount, oldCount, titlerare)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error writing leaderboard")
	} else {
		logs.Logs().Info().
			Str("Board", board).
			Msg("Leaderboard updated successfully")
	}
}

func getRarestFish(params LeaderboardParams) (map[string]BoardData, error) {
	board := params.LeaderboardType
	date2 := params.Date2
	pool := params.Pool
	date := params.Date

	globalFishTypesCount := make(map[string]BoardData)

	// Query the database to get the count of each fish type caught in the chat
	rows, err := pool.Query(context.Background(), `
				SELECT fishname, COUNT(*), chat
				FROM fish
				WHERE date < $1
				AND date > $2
				GROUP BY fishname, chat
				`, date, date2)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Msg("Error querying database for rarest fish")
		return globalFishTypesCount, err
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[BoardData])
	if err != nil && err != pgx.ErrNoRows {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Msg("Error collecting rows")
		return globalFishTypesCount, err
	}

	for _, result := range results {

		result.FishType, err = FishStuff(result.FishName, params)
		if err != nil {
			return globalFishTypesCount, err
		}

		pfp := fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", result.Chat, result.Chat)
		existingFishInfo, exists := globalFishTypesCount[result.FishName]
		if exists {
			existingFishInfo.Count += result.Count

			if existingFishInfo.ChatCounts == nil {
				existingFishInfo.ChatCounts = make(map[string]int)
			}
			existingFishInfo.ChatCounts[pfp] += result.Count

			globalFishTypesCount[result.FishName] = existingFishInfo
		} else {
			globalFishTypesCount[result.FishName] = BoardData{
				Count:      result.Count,
				FishName:   result.FishName,
				FishType:   result.FishType,
				ChatCounts: map[string]int{pfp: result.Count},
			}
		}
	}

	return globalFishTypesCount, nil
}

func writeRare(filePath string, fishCaught map[string]BoardData, oldCountRecord map[string]BoardData, title string) error {

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

	prefix := "| Rank | Fish | Times Caught | Chat |"

	_, _ = fmt.Fprintln(file, prefix)

	_, err = fmt.Fprintln(file, "|------|--------|-----------|-------|")
	if err != nil {
		return err
	}

	sortedPlayers := sortMapStringFishInfo(fishCaught, "countdesc")

	rank := 1
	prevRank := 1
	prevCount := -1
	occupiedRanks := make(map[int]int)

	for _, FishName := range sortedPlayers {
		Count := fishCaught[FishName].Count
		ChatCounts := fishCaught[FishName].ChatCounts
		Emoji := fishCaught[FishName].FishType

		if Count != prevCount {
			rank += occupiedRanks[rank]
			occupiedRanks[rank] = 1
		} else {
			rank = prevRank
			occupiedRanks[rank]++
		}

		// Store the rank
		if ranksksk, ok := fishCaught[FishName]; ok {

			ranksksk.Rank = rank

			fishCaught[FishName] = ranksksk
		}

		var found bool
		oldRank := -1
		oldCount := Count
		oldFishInfo, ok := oldCountRecord[FishName]
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

		ranks := Ranks(rank)

		_, _ = fmt.Fprintf(file, "| %s %s | %s %s | %s |", ranks, changeEmoji, Emoji, FishName, counts)

		sortedChatCounts := sortMapStringInt(ChatCounts, "nameasc")

		for _, chat := range sortedChatCounts {
			_, _ = fmt.Fprintf(file, " %s %d ", chat, ChatCounts[chat])
		}
		_, _ = fmt.Fprint(file, "|")

		_, err = fmt.Fprintln(file)
		if err != nil {
			return err
		}

		prevCount = Count
		prevRank = rank
	}

	_, _ = fmt.Fprintf(file, "\n_Last updated at %s_", time.Now().In(time.UTC).Format("2006-01-02 15:04:05 UTC"))

	// This has to be here, because im not getting the rank directly from the query
	err = writeRaw(filePath, fishCaught)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Msg("Error writing raw leaderboard")
		return err
	} else {
		logs.Logs().Info().
			Str("Path", filePath).
			Msg("Raw leaderboard updated successfully")
	}

	return nil
}
