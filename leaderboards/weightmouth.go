package leaderboards

import (
	"context"
	"fmt"
	"gofish/logs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

func processWeightMouth(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	config := params.Config
	limit := params.Limit
	mode := params.Mode

	filePath := returnPath(params)

	oldRecordWeight, err := getJsonBoardString(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error reading old leaderboard")
		return
	}

	var weightlimit float64

	if limit == "" {
		weightlimit = config.Chat["global"].Weightlimitmouth
	} else {
		weightlimit, err = strconv.ParseFloat(limit, 64)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Limit", limit).
				Str("Board", board).
				Msg("Error converting custom weight limit to float64")
			return
		}
	}

	recordWeight, err := getWeightMouth(params, weightlimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error getting weight records")
		return
	}

	AreMapsSame := didFishMapChange(params, oldRecordWeight, recordWeight)

	if AreMapsSame && mode != "force" {
		logs.Logs().Warn().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Not updating board because there are no changes")
		return
	}

	var title string

	if params.Title == "" {

		title = "### Biggest combined weight mouth bonus catches globally\n"

	} else {
		title = fmt.Sprintf("%s\n", params.Title)
	}

	err = printWeightMouth(filePath, recordWeight, oldRecordWeight, title, weightlimit)
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

func getWeightMouth(params LeaderboardParams, limit float64) (map[string]BoardData, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	date2 := params.Date2
	date := params.Date
	pool := params.Pool

	recordWeight := make(map[string]BoardData)

	rows, err := pool.Query(context.Background(), `
		SELECT f.playerid, f.weight, f.fishname, m.weight as weightmouth, m.fishname as fishnamemouth, f.weight + m.weight as totalweight, 
		f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid
		from fish f
		join 
		(
		select fishname, weight, date from fish
		where catchtype = 'mouth'
		and date < $1
		and date > $2
		) m on f.date = m.date
		where f.weight + m.weight >= $3`, date, date2, limit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error querying database")
		return recordWeight, err
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[BoardData])
	if err != nil && err != pgx.ErrNoRows {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error collecting rows")
		return recordWeight, err
	}

	for _, result := range results {

		result.Player, _, result.Verified, _, err = PlayerStuff(result.PlayerID, params, pool)
		if err != nil {
			return recordWeight, err
		}

		result.FishType, err = FishStuff(result.FishName, params)
		if err != nil {
			return recordWeight, err
		}

		result.FishTypeMouth, err = FishStuff(result.FishNameMouth, params)
		if err != nil {
			return recordWeight, err
		}

		result.ChatPfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", result.Chat, result.Chat)

		recordWeight[result.Date.Format("2006-01-02 15:04:05 UTC")] = result
	}

	return recordWeight, nil
}

func printWeightMouth(filePath string, recordWeight map[string]BoardData, oldRecordWeight map[string]BoardData, title string, weightlimit float64) error {

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

	_, _ = fmt.Fprintln(file, "| Rank | Player | Normal fish | Mouth bonus | Combined Weight | Date in UTC | Chat |")

	_, err = fmt.Fprintln(file, "|------|--------|---------|---------|---------|---------|---------|")
	if err != nil {
		return err
	}

	rank := 1
	prevRank := 1
	prevWeight := 0.0
	occupiedRanks := make(map[int]int)

	sortedWeightRecords := sortMapStringFishInfo(recordWeight, "totalweightdesc")

	for _, date := range sortedWeightRecords {
		weight := recordWeight[date].Weight
		fishType := recordWeight[date].FishType
		fishName := recordWeight[date].FishName
		weightmouth := recordWeight[date].WeightMouth
		fishTypemouth := recordWeight[date].FishTypeMouth
		fishNamemouth := recordWeight[date].FishNameMouth
		totalweight := recordWeight[date].TotalWeight
		player := recordWeight[date].Player
		chatpfp := recordWeight[date].ChatPfp

		// Increment rank only if the count has changed
		if weight != float64(prevWeight) {
			rank += occupiedRanks[rank]
			occupiedRanks[rank] = 1
		} else {
			rank = prevRank
			occupiedRanks[rank]++
		}

		var found bool

		oldRank := -1

		if info, ok := oldRecordWeight[date]; ok {
			found = true
			oldRank = info.Rank
		}

		// update and store the rank
		if copy, ok := recordWeight[date]; ok {

			copy.Rank = rank

			recordWeight[date] = copy
		}

		changeEmoji := ChangeEmoji(rank, oldRank, found)

		botIndicator := ""
		if recordWeight[date].Bot == "supibot" && !recordWeight[date].Verified {
			botIndicator = "*"
		}

		ranks := Ranks(rank)

		_, _ = fmt.Fprintf(file, "| %s %s | %s%s | %s %s %.2f | %s %s %.2f | %.2f | %s | %s |",
			ranks, changeEmoji, player, botIndicator, fishType, fishName, weight, fishTypemouth, fishNamemouth, weightmouth, totalweight, date, chatpfp)

		_, err = fmt.Fprintln(file)
		if err != nil {
			return err
		}

		prevWeight = weight
		prevRank = rank

	}

	_, _ = fmt.Fprintf(file, "\n_Only showing catches with a total weight greater than >= %v lbs_\n", weightlimit)

	_, _ = fmt.Fprintf(file, "\n_Last updated at %s_", time.Now().In(time.UTC).Format("2006-01-02 15:04:05 UTC"))

	// This has to be here, because im not getting the rank directly from the query
	err = writeRaw(filePath, recordWeight)
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
