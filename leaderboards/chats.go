package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"os"
	"path/filepath"
	"time"
)

func RunChatStatsGlobal(params LeaderboardParams) {
	pool := params.Pool
	config := params.Config

	chatStats := make(map[string]data.FishInfo)

	filePath := filepath.Join("leaderboards", "global", "chats.md")
	oldChatStats, err := ReadOldChatStats(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).Str("Path", filePath).Msg("Error reading old chatStats leaderboard")
		return
	}

	for chatName, chat := range config.Chat {
		var chatInfo data.FishInfo

		if !chat.CheckFData {
			if chatName != "global" && chatName != "default" {
				logs.Logs().Warn().Str("Chat", chatName).Msg("Skipping chat because checkfdata is false")
			}
			continue
		}

		// Get the amount of fish caught per chat
		err := pool.QueryRow(context.Background(), `
				SELECT COUNT(*) AS fish_count
				FROM fish
				WHERE chat = $1
				`, chatName).Scan(&chatInfo.Count)
		if err != nil {
			logs.Logs().Error().Err(err).Str("Chat", chatName).Msg("Error querying fish database for fish count")
			return
		}

		// Skip chats with zero fish caught
		if chatInfo.Count == 0 {
			logs.Logs().Debug().Str("Chat", chatName).Msg("Skipping chat with zero fish caught")
			continue
		}

		// Get the active fishers
		err = pool.QueryRow(context.Background(), `
				SELECT COUNT(*) AS active_fishers_count
				FROM (
					SELECT DISTINCT playerid
					FROM fish
					WHERE chat = $1
					AND date >= CURRENT_DATE - INTERVAL '7 days'
					GROUP BY playerid
					HAVING COUNT(*) > 10
				) AS subquery
				`, chatName).Scan(&chatInfo.MaxCount)
		if err != nil {
			logs.Logs().Error().Err(err).Str("Chat", chatName).Msg("Error querying fish database for active fishers")
			return
		}

		// Get the unique fishers
		err = pool.QueryRow(context.Background(), `
				SELECT COUNT(*) AS unique_fishers_count
				FROM (
					SELECT DISTINCT playerid
					FROM fish
					WHERE chat = $1
					GROUP BY playerid
				) AS subquery
				`, chatName).Scan(&chatInfo.FishId)
		if err != nil {
			logs.Logs().Error().Err(err).Str("Chat", chatName).Msg("Error querying fish database for unique fishers")
			return
		}

		// Get the channel record
		err = pool.QueryRow(context.Background(), `
				SELECT f.playerid, f.weight, f.fishname
				FROM fish f
				JOIN (
					SELECT MAX(weight) AS max_weight
					FROM fish
					WHERE chat = $1
				) max_weight_chat ON f.weight = max_weight_chat.max_weight
				WHERE f.chat = $1;
				`, chatName).Scan(&chatInfo.PlayerID, &chatInfo.Weight, &chatInfo.TypeName)
		if err != nil {
			logs.Logs().Error().Err(err).Str("Chat", chatName).Msgf("Error querying fish database for channel record")
			return
		}

		err = pool.QueryRow(context.Background(), "SELECT fishtype FROM fishinfo WHERE fishname = $1", chatInfo.TypeName).Scan(&chatInfo.Type)
		if err != nil {
			logs.Logs().Error().Err(err).Str("Chat", chatName).Str("Fishname", chatInfo.TypeName).Msg("Error retrieving fish type for fish name")
			return
		}

		err = pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", chatInfo.PlayerID).Scan(&chatInfo.Player)
		if err != nil {
			logs.Logs().Error().Err(err).Str("Chat", chatName).Int("PlayerID", chatInfo.PlayerID).Msg("Error retrieving player name for id")
			return
		}

		chatInfo.ChatPfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chatName, chatName)
		chatInfo.Chat = chatName

		// Update chatStats
		chatStats[chatName] = chatInfo
	}

	updateChatStats(chatStats, oldChatStats, filePath)

}

func updateChatStats(chatStats map[string]data.FishInfo, oldChatStats map[string]LeaderboardInfo, filepath string) {
	logs.Logs().Info().Msg("Updating global chatStats leaderboard...")
	title := "### Chat leaderboard\n"
	err := writeChatStats(filepath, chatStats, oldChatStats, title)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error writing global chatStats leaderboard")
	} else {
		logs.Logs().Info().Msg("Global chatStats leaderboard updated successfully")
	}
}

func writeChatStats(filePath string, chatStats map[string]data.FishInfo, oldChatStats map[string]LeaderboardInfo, title string) error {

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

	_, _ = fmt.Fprintln(file, "| Rank | Chat | Fish Caught | Active Players | Unique Players | Channel Record 🎊 |")
	_, _ = fmt.Fprintln(file, "|------|------|-------------|----------------|----------------|-------------------|")

	sortedChats := SortMapByCountDesc(chatStats)

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
		oldCount := count
		oldWeight := weight
		oldActive := activefishers
		oldUnique := uniquefishers
		oldChatInfo, ok := oldChatStats[chatname]
		if ok {
			found = true
			oldRank = oldChatInfo.Rank
			oldCount = oldChatInfo.Count
			oldWeight = oldChatInfo.Weight
			oldActive = oldChatInfo.Silver
			oldUnique = oldChatInfo.Bronze
		}

		changeEmoji := ChangeEmoji(rank, oldRank, found)

		var counts, fishweight, activepl, uniquepl string

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

		ranks := Ranks(rank)

		_, _ = fmt.Fprintf(file, "| %s %s | %s %s | %s | %s | %s | %s %s lbs, %s |", ranks, changeEmoji, chatname, pfp, counts, activepl, uniquepl, fishtype, fishweight, player)
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

	return nil
}
