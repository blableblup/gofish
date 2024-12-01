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

func RunCountFishTypesGlobal(params LeaderboardParams) {
	board := params.LeaderboardType
	title := params.Title
	path := params.Path
	mode := params.Mode

	var filePath, titlerare string

	if path == "" {
		filePath = filepath.Join("leaderboards", "global", "rare.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", "global", path)
	}

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

func getRarestFish(params LeaderboardParams) (map[string]data.FishInfo, error) {
	board := params.LeaderboardType
	config := params.Config
	date2 := params.Date2
	pool := params.Pool
	date := params.Date

	globalFishTypesCount := make(map[string]data.FishInfo)

	// Process all chats
	for chatName, chat := range config.Chat {
		if !chat.CheckFData {
			if chatName != "global" && chatName != "default" {
				logs.Logs().Warn().
					Str("Board", board).
					Str("Chat", chatName).
					Msg("Skipping chat because checkfdata is false")
			}
			continue
		}

		// Query the database to get the count of each fish type caught in the chat
		rows, err := pool.Query(context.Background(), `
				SELECT fishname, COUNT(*) AS type_count
				FROM fish
				WHERE chat = $1
				AND date < $2
				AND date > $3
				GROUP BY fishname
				`, chatName, date, date2)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error querying database for rarest fish")
			return globalFishTypesCount, err
		}
		defer rows.Close()

		// Iterate through the query results and store fish type count for each chat
		for rows.Next() {
			var fishInfo data.FishInfo

			if err := rows.Scan(&fishInfo.TypeName, &fishInfo.Count); err != nil {
				logs.Logs().Error().Err(err).
					Str("Chat", chatName).
					Str("Board", board).
					Msg("Error scanning row for rarest fish")
				return globalFishTypesCount, err
			}

			err = pool.QueryRow(context.Background(), "SELECT fishtype FROM fishinfo WHERE fishname = $1", fishInfo.TypeName).Scan(&fishInfo.Type)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Fish name", fishInfo.TypeName).
					Str("Board", board).
					Str("Chat", chatName).
					Msg("Error retrieving fish type for fish name")
				return globalFishTypesCount, err
			}

			pfp := fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chatName, chatName)
			existingFishInfo, exists := globalFishTypesCount[fishInfo.TypeName]
			if exists {
				existingFishInfo.Count += fishInfo.Count

				if existingFishInfo.ChatCounts == nil {
					existingFishInfo.ChatCounts = make(map[string]int)
				}
				existingFishInfo.ChatCounts[pfp] += fishInfo.Count

				globalFishTypesCount[fishInfo.TypeName] = existingFishInfo
			} else {
				globalFishTypesCount[fishInfo.TypeName] = data.FishInfo{
					Count:      fishInfo.Count,
					TypeName:   fishInfo.TypeName,
					Type:       fishInfo.Type,
					ChatCounts: map[string]int{pfp: fishInfo.Count},
				}
			}
		}
	}

	return globalFishTypesCount, nil
}

func writeRare(filePath string, fishCaught map[string]data.FishInfo, oldCountRecord map[string]data.FishInfo, title string) error {

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

	sortedPlayers := sortFishRecords(fishCaught)

	rank := 1
	prevRank := 1
	prevCount := -1
	occupiedRanks := make(map[int]int)

	for _, FishName := range sortedPlayers {
		Count := fishCaught[FishName].Count
		ChatCounts := fishCaught[FishName].ChatCounts
		Emoji := fishCaught[FishName].Type

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

		_, err = fmt.Fprintln(file)
		if err != nil {
			return err
		}

		prevCount = Count
		prevRank = rank
	}

	_, _ = fmt.Fprintf(file, "\n_Last updated at %s_", time.Now().In(time.UTC).Format("2006-01-02 15:04:05 UTC"))

	// This has to be here, because im not getting the rank directly from the query
	err = writeRawString(filePath, fishCaught)
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
