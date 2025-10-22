package leaderboards

import (
	"fmt"
	"gofish/logs"
	"gofish/utils"
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

func getChatStats(params LeaderboardParams) (map[string]BoardData, error) {
	board := params.LeaderboardType
	config := params.Config

	chatStats := make(map[string]BoardData)
	chatStatsIDK := make(map[string]*BoardData)

	// add every chat in the config to the map first
	for chatName := range config.Chat {

		if chatName == "global" || chatName == "default" {
			continue
		}

		chatStatsIDK[chatName] = &BoardData{
			Count:         0,
			ActiveFishers: 0,
			UniqueFishers: 0,
			UniqueFish:    0,
			Player:        "",
			PlayerID:      0,
			FishType:      "",
			FishName:      "",
			Weight:        0.0,
			Chat:          chatName,
			ChatPfp:       fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chatName, chatName),
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
		select count(*) as activefishers, chat
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
		chatStatsIDK[result.Chat].ActiveFishers = result.ActiveFishers
	}

	// change date2 back
	params.Date2 = datecopy

	// unique fishers
	queryUniqueFishers := `
		select count(distinct playerid) as uniquefishers, chat
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
		chatStatsIDK[result.Chat].UniqueFishers = result.UniqueFishers
	}

	// unique fish
	queryUniqueFish := `
		select count(distinct fishname) as uniquefish, chat
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
		chatStatsIDK[result.Chat].UniqueFish = result.UniqueFish
	}

	// channel records
	// if there are multiple channel records with same weight fish wont always be the same ? or idk
	queryChannelRecord := `
		SELECT bub.weight, bub.fishname, bub.chat, bub.date, bub.catchtype, bub.playerid 
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

		chatStatsIDK[result.Chat].FishType, err = FishStuff(result.FishName, params)
		if err != nil {
			return chatStats, err
		}

		chatStatsIDK[result.Chat].FishName = result.FishName

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

func writeChatStats(filePath string, chatStats map[string]BoardData, oldChatStats map[string]BoardData, title string) error {

	header := []string{"Rank", "Chat", "Fish Caught", "Active Players", "Unique Players", "Unique Fish", "Channel Record 🎊"}

	sortedChats := sortMapStringFishInfo(chatStats, "countdesc")

	rank := 1
	prevRank := 1
	prevCount := -1
	occupiedRanks := make(map[int]int)

	var data [][]string

	for _, chat := range sortedChats {
		count := chatStats[chat].Count
		pfp := chatStats[chat].ChatPfp
		weight := chatStats[chat].Weight
		fishtype := chatStats[chat].FishType
		fishname := chatStats[chat].FishName
		player := chatStats[chat].Player
		chatname := chatStats[chat].Chat
		activefishers := chatStats[chat].ActiveFishers
		uniquefishers := chatStats[chat].UniqueFishers
		uniquefish := chatStats[chat].UniqueFish

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
			oldActive = oldChatInfo.ActiveFishers
			oldUnique = oldChatInfo.UniqueFishers
			oldUniquef = oldChatInfo.UniqueFish
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
		} else if uniquefdiff < 0 {
			uniquef = fmt.Sprintf("%d (%d)", uniquefish, uniquefdiff)
		} else {
			uniquef = fmt.Sprintf("%d", uniquefish)
		}

		ranks := Ranks(rank)

		row := []string{
			fmt.Sprintf("%s %s", ranks, changeEmoji),
			fmt.Sprintf("%s %s", pfp, chatname),
			counts,
			activepl,
			uniquepl,
			uniquef,
			fmt.Sprintf("%s %s %s lbs, %s", fishtype, fishname, fishweight, player),
		}

		data = append(data, row)

		prevCount = count
		prevRank = rank
	}

	notes := []string{
		"Active players means that they caught more than 10 fish in the last seven days",
		"Unique players is how many different players caught a fish in that chat",
	}

	err := writeBoard(filePath, title, header, data, notes)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Msg("Error writing leaderboard")
		return err
	}

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
