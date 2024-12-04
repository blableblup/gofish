package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"gofish/utils"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func RunChatStatsGlobal(params LeaderboardParams) {
	board := params.LeaderboardType
	title := params.Title
	path := params.Path

	var filePath, titlestats string

	if path == "" {
		filePath = filepath.Join("leaderboards", "global", "chats.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", "global", path)
	}

	oldChatStats, err := getJsonBoardString(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error reading old chatStats leaderboard")
		return
	}

	chatStats, err := getChatStats(params)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error getting leaderboard")
		return
	}

	// Not checking if maps changed because they should always have changes here

	logs.Logs().Info().
		Str("Board", board).
		Msg("Updating leaderboard")

	if title == "" {
		titlestats = "### Chat leaderboard\n"
	} else {
		titlestats = fmt.Sprintf("%s\n", title)
	}

	err = writeChatStats(filePath, chatStats, oldChatStats, titlestats)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Msg("Error writing leaderboard")
	} else {
		logs.Logs().Info().
			Str("Board", board).
			Msg("Leaderboard updated successfully")
	}
}

func getChatStats(params LeaderboardParams) (map[string]data.FishInfo, error) {
	board := params.LeaderboardType
	config := params.Config
	date2 := params.Date2
	pool := params.Pool
	date := params.Date

	chatStats := make(map[string]data.FishInfo)

	for chatName, chat := range config.Chat {
		var chatInfo data.FishInfo

		if !chat.CheckFData {
			if chatName != "global" && chatName != "default" {
				logs.Logs().Warn().
					Str("Board", board).
					Str("Chat", chatName).
					Msg("Skipping chat because checkfdata is false")
			}
			continue
		}

		// Get the amount of fish caught per chat
		err := pool.QueryRow(context.Background(), `
				SELECT COUNT(*) AS fish_count
				FROM fish
				WHERE chat = $1
				AND date < $2
	  			AND date > $3
				`, chatName, date, date2).Scan(&chatInfo.Count)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error querying fish database for fish count")
			return chatStats, err
		}

		// Skip chats with zero fish caught
		if chatInfo.Count == 0 {
			logs.Logs().Debug().
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Skipping chat with zero fish caught")
			continue
		}

		// Get the active fishers for the last seven days (defined by date)
		// For pastdate, the query should have >= else it will only be 6 days
		datetime, err := utils.ParseDate(date)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Date", date).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error parsing date into time.Time for active fishers")
			return chatStats, err
		}
		pastDate := datetime.AddDate(0, 0, -7)

		err = pool.QueryRow(context.Background(), `
				SELECT COUNT(*) AS active_fishers_count
				FROM (
					SELECT DISTINCT playerid
					FROM fish
					WHERE chat = $1
					AND date >= $2
					AND date < $3
					GROUP BY playerid
					HAVING COUNT(*) > 10
				) AS subquery
				`, chatName, pastDate, datetime).Scan(&chatInfo.MaxCount)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error querying fish database for active fishers")
			return chatStats, err
		}

		// Get the unique fishers
		err = pool.QueryRow(context.Background(), `
				SELECT COUNT(*) AS unique_fishers_count
				FROM (
					SELECT DISTINCT playerid
					FROM fish
					WHERE chat = $1
					AND date < $2
	  				AND date > $3
					GROUP BY playerid
				) AS subquery
				`, chatName, date, date2).Scan(&chatInfo.FishId)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error querying fish database for unique fishers")
			return chatStats, err
		}

		// Get the unique fish caught
		err = pool.QueryRow(context.Background(), `
				SELECT COUNT(*) AS unique_fish_count
				FROM (
					SELECT DISTINCT fishname
					FROM fish
					WHERE chat = $1
					AND date < $2
	  				AND date > $3
					GROUP BY fishname
				) AS subquery
				`, chatName, date, date2).Scan(&chatInfo.ChatId)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error querying fish database for unique fish caught")
			return chatStats, err
		}

		// Get the channel record
		err = pool.QueryRow(context.Background(), `
				SELECT f.playerid, f.weight, f.fishname
				FROM fish f
				JOIN (
					SELECT MAX(weight) AS max_weight
					FROM fish
					WHERE chat = $1
					AND date < $2
	  				AND date > $3
				) max_weight_chat ON f.weight = max_weight_chat.max_weight
				WHERE f.chat = $1;
				`, chatName, date, date2).Scan(&chatInfo.PlayerID, &chatInfo.Weight, &chatInfo.TypeName)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error querying fish database for channel record")
			return chatStats, err
		}

		err = pool.QueryRow(context.Background(), "SELECT fishtype FROM fishinfo WHERE fishname = $1", chatInfo.TypeName).Scan(&chatInfo.Type)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Fishname", chatInfo.TypeName).
				Str("Board", board).
				Msg("Error retrieving fish type for fish name")
			return chatStats, err
		}

		err = pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", chatInfo.PlayerID).Scan(&chatInfo.Player)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Int("PlayerID", chatInfo.PlayerID).
				Str("Board", board).
				Msg("Error retrieving player name for id")
			return chatStats, err
		}

		chatInfo.ChatPfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chatName, chatName)
		chatInfo.Chat = chatName

		// Update chatStats
		chatStats[chatName] = chatInfo
	}

	return chatStats, nil
}

func writeChatStats(filePath string, chatStats map[string]data.FishInfo, oldChatStats map[string]data.FishInfo, title string) error {

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

	_, _ = fmt.Fprintln(file, "| Rank | Chat | Fish Caught | Active Players | Unique Players | Unique Fish | Channel Record 🎊 |")
	_, _ = fmt.Fprintln(file, "|------|------|-------------|----------------|----------------|-------------|-------------------|")

	sortedChats := sortFishRecords(chatStats)

	rank := 1
	prevRank := 1
	prevCount := -1
	occupiedRanks := make(map[int]int)

	for _, chat := range sortedChats {
		count := chatStats[chat].Count
		pfp := chatStats[chat].ChatPfp
		weight := chatStats[chat].Weight
		fishtype := chatStats[chat].Type
		player := chatStats[chat].Player
		chatname := chatStats[chat].Chat
		activefishers := chatStats[chat].MaxCount
		uniquefishers := chatStats[chat].FishId
		uniquefish := chatStats[chat].ChatId

		// Increment rank only if the count has changed
		if count != prevCount {
			rank += occupiedRanks[rank]
			occupiedRanks[rank] = 1
		} else {
			rank = prevRank
			occupiedRanks[rank]++
		}

		// Store the rank
		if ranksksk, ok := chatStats[chat]; ok {

			ranksksk.Rank = rank

			chatStats[chat] = ranksksk
		}

		var found bool
		oldRank := -1
		oldCount := count
		oldWeight := weight
		oldActive := activefishers
		oldUnique := uniquefishers
		oldUniquef := uniquefish
		oldChatInfo, ok := oldChatStats[chatname]
		if ok {
			found = true
			oldRank = oldChatInfo.Rank
			oldCount = oldChatInfo.Count
			oldWeight = oldChatInfo.Weight
			oldActive = oldChatInfo.MaxCount
			oldUnique = oldChatInfo.FishId
			oldUniquef = oldChatInfo.ChatId
		}

		changeEmoji := ChangeEmoji(rank, oldRank, found)

		var counts, fishweight, activepl, uniquepl, uniquef string

		countDifference := count - oldCount
		if countDifference > 0 {
			counts = fmt.Sprintf("%d (+%d)", count, countDifference)
		} else {
			counts = fmt.Sprintf("%d", count)
		}

		weightDifference := weight - oldWeight
		if weightDifference > 0 {
			fishweight = fmt.Sprintf("%.2f (+%.2f)", weight, weightDifference)
		} else {
			fishweight = fmt.Sprintf("%.2f", weight)
		}

		activediff := activefishers - oldActive
		if activediff > 0 {
			activepl = fmt.Sprintf("%d (+%d)", activefishers, activediff)
		} else if activediff < 0 {
			activepl = fmt.Sprintf("%d (%d)", activefishers, activediff)
		} else {
			activepl = fmt.Sprintf("%d", activefishers)
		}

		uniquediff := uniquefishers - oldUnique
		if uniquediff > 0 {
			uniquepl = fmt.Sprintf("%d (+%d)", uniquefishers, uniquediff)
		} else if uniquediff < 0 {
			uniquepl = fmt.Sprintf("%d (%d)", uniquefishers, uniquediff)
		} else {
			uniquepl = fmt.Sprintf("%d", uniquefishers)
		}

		uniquefdiff := uniquefish - oldUniquef
		if uniquefdiff > 0 {
			uniquef = fmt.Sprintf("%d (+%d)", uniquefish, uniquefdiff)
		} else if uniquediff < 0 {
			uniquef = fmt.Sprintf("%d (%d)", uniquefish, uniquefdiff)
		} else {
			uniquef = fmt.Sprintf("%d", uniquefish)
		}

		ranks := Ranks(rank)

		_, _ = fmt.Fprintf(file, "| %s %s | %s %s | %s | %s | %s | %s | %s %s lbs, %s |", ranks, changeEmoji, chatname, pfp, counts, activepl, uniquepl, uniquef, fishtype, fishweight, player)
		_, err = fmt.Fprintln(file)
		if err != nil {
			return err
		}

		prevCount = count
		prevRank = rank
	}

	_, _ = fmt.Fprint(file, "\n_Active players means that they caught more than 10 fish in the last seven days_\n")
	_, _ = fmt.Fprint(file, "\n_Unique players is how many different players caught a fish in that chat_\n")
	_, _ = fmt.Fprintf(file, "\n_Last updated at %s_", time.Now().In(time.UTC).Format("2006-01-02 15:04:05 UTC"))

	// This has to be here, because im not getting the rank directly from the query
	err = writeRawString(filePath, chatStats)
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
