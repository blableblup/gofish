package leaderboards

import (
	"fmt"
	"gofish/data"
	"gofish/logs"
	"gofish/utils"
	"os"
	"path/filepath"
	"time"
)

func RunChatStatsGlobal(params LeaderboardParams) {
	board := params.LeaderboardType
	title := params.Title

	filePath := returnPath(params)

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

	var titlestats string

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

	chatStats := make(map[string]data.FishInfo)
	chatStatsIDK := make(map[string]*data.FishInfo)

	// add every chat in the config to the map first
	for chatName := range config.Chat {

		if chatName == "global" || chatName == "default" {
			continue
		}

		chatStatsIDK[chatName] = &data.FishInfo{
			Count:    0,
			MaxCount: 0,
			FishId:   0,
			ChatId:   0,
			Player:   "",
			PlayerID: 0,
			Type:     "",
			TypeName: "",
			Weight:   0.0,
			Chat:     chatName,
			ChatPfp:  fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chatName, chatName),
		}
	}

	// total fish caught
	queryFishPerChat := `
		select count(*), chat
		from fish
		where date < $1
		and date > $2
		group by chat`

	results, err := ReturnFishSliceQuery(params, queryFishPerChat, false)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Msg("Error querying fish database for fish count")
		return chatStats, err
	}

	for _, result := range results {
		chatStatsIDK[result.Chat].Count = result.Count
	}

	// active fishers who caught more than 10 fish
	// for this params.Date2 needs to be changed
	// to get the active fishers for last seven days

	datecopy := params.Date2

	// reparse date and then subtract 7 days from it
	// and use that as date2
	datetime, err := utils.ParseDate(params.Date)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Date", params.Date).
			Str("Board", board).
			Msg("Error parsing date into time.Time for active fishers")
		return chatStats, err
	}
	pastDate := datetime.AddDate(0, 0, -7)

	params.Date2 = pastDate.Format("2006-01-02")

	queryActiveFishers := `
		select count(*) as maxcount, chat
		from (
			select distinct playerid, chat
			from fish
			where date < $1
			and date >= $2
			group by playerid, chat
			having count(*) > 10
		) as subquery
		group by chat`

	results, err = ReturnFishSliceQuery(params, queryActiveFishers, false)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Msg("Error querying fish database for active fishers")
		return chatStats, err
	}

	for _, result := range results {
		chatStatsIDK[result.Chat].MaxCount = result.MaxCount
	}

	// change date2 back
	params.Date2 = datecopy

	// unique fishers
	queryUniqueFishers := `
		select count(distinct playerid) as fishid, chat
		from fish
		where date < $1
		and date > $2
		group by chat`

	results, err = ReturnFishSliceQuery(params, queryUniqueFishers, false)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Msg("Error querying fish database for unique fishers")
		return chatStats, err
	}

	for _, result := range results {
		chatStatsIDK[result.Chat].FishId = result.FishId
	}

	// unique fish
	queryUniqueFish := `
		select count(distinct fishname) as chatid, chat
		from fish
		where date < $1
		and date > $2
		group by chat`

	results, err = ReturnFishSliceQuery(params, queryUniqueFish, false)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Msg("Error querying fish database for unique fish")
		return chatStats, err
	}

	for _, result := range results {
		chatStatsIDK[result.Chat].ChatId = result.ChatId
	}

	// channel records
	// if there are multiple channel records with same weight fish wont always be the same ? or idk
	queryChannelRecord := `
		SELECT bub.weight, bub.fishname as typename, bub.chat, bub.date, bub.catchtype, bub.playerid 
		FROM (
        SELECT fish.*, 
        RANK() OVER (
            PARTITION BY chat
            ORDER BY weight DESC
        )
        FROM fish
		WHERE date < $1
	  	AND date > $2
    	) bub 
		WHERE RANK = 1`

	results, err = ReturnFishSliceQuery(params, queryChannelRecord, false)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Msg("Error querying fish database for unique fish")
		return chatStats, err
	}

	for _, result := range results {
		chatStatsIDK[result.Chat].Weight = result.Weight

		chatStatsIDK[result.Chat].Type, err = FishStuff(result.TypeName, params)
		if err != nil {
			return chatStats, err
		}

		chatStatsIDK[result.Chat].TypeName = result.TypeName

		chatStatsIDK[result.Chat].PlayerID = result.PlayerID

		chatStatsIDK[result.Chat].Player, _, _, _, err = PlayerStuff(result.PlayerID, params, params.Pool)
		if err != nil {
			return chatStats, err
		}
	}

	// >_< ? change it back to the other map idk
	for chat, stats := range chatStatsIDK {
		chatStats[chat] = *stats
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

	sortedChats := sortMapStringFishInfo(chatStats, "countdesc")

	rank := 1
	prevRank := 1
	prevCount := -1
	occupiedRanks := make(map[int]int)

	for _, chat := range sortedChats {
		count := chatStats[chat].Count
		pfp := chatStats[chat].ChatPfp
		weight := chatStats[chat].Weight
		fishtype := chatStats[chat].Type
		fishname := chatStats[chat].TypeName
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

		_, _ = fmt.Fprintf(file, "| %s %s | %s %s | %s | %s | %s | %s | %s %s %s lbs, %s |",
			ranks, changeEmoji, pfp, chatname, counts, activepl, uniquepl, uniquef, fishtype, fishname, fishweight, player)
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
	err = writeRaw(filePath, chatStats)
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
