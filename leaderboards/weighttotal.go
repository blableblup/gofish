package leaderboards

import (
	"context"
	"fmt"
	"gofish/logs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func processWeightTotal(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	config := params.Config
	global := params.Global
	limit := params.Limit
	chat := params.Chat
	mode := params.Mode

	filePath := returnPath(params)

	oldRecordWeight, err := getJsonBoard(filePath)
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
		weightlimit = chat.Weightlimittotal
		if weightlimit == 0 {
			weightlimit = config.Chat["default"].Weightlimittotal
		}
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

	recordWeight, err := getTotalWeight(params, weightlimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error getting weight records")
		return
	}

	AreMapsSame := didPlayerMapsChange(params, oldRecordWeight, recordWeight)

	if AreMapsSame && mode != "force" {
		logs.Logs().Warn().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Not updating board because there are no changes")
		return
	}

	var title string

	if params.Title == "" {
		if !global {
			if strings.HasSuffix(chatName, "s") {
				title = fmt.Sprintf("### Total weight of all fish caught per player in %s' chat\n", chatName)
			} else {
				title = fmt.Sprintf("### Total weight of all fish caught per player in %s's chat\n", chatName)
			}
		} else {
			title = "### Total weight of all fish caught per player globally\n"
		}
	} else {
		title = fmt.Sprintf("%s\n", params.Title)
	}

	err = printTotalWeight(filePath, recordWeight, oldRecordWeight, title, global, weightlimit)
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

func getTotalWeight(params LeaderboardParams, limit float64) (map[int]BoardData, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	global := params.Global
	date2 := params.Date2
	pool := params.Pool
	date := params.Date

	recordWeight := make(map[int]BoardData)
	var rows pgx.Rows
	var err error

	if !global {
		rows, err = pool.Query(context.Background(), `
		select playerid, sum(weight) as totalweight
		from fish
		where chat = $1
		and date < $3
		and date > $4
		group by playerid
		having sum(weight) > $2`, chatName, limit, date, date2)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error querying database")
			return recordWeight, err
		}
	} else {
		rows, err = pool.Query(context.Background(), `
		select playerid, sum(weight) as totalweight
		from fish
		where date < $1
		and date > $2
		group by playerid
		having sum(weight) > $3`, date, date2, limit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error querying database")
			return recordWeight, err
		}
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[BoardData])
	if err != nil && err != pgx.ErrNoRows {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error collecting rows")
		return recordWeight, err
	}

	var playerIDs []int

	for _, result := range results {

		result.Player, result.Date, result.Verified, _, err = PlayerStuff(result.PlayerID, params, pool)
		if err != nil {
			return recordWeight, err
		}

		if result.Date.Before(time.Date(2023, time.September, 14, 0, 0, 0, 0, time.UTC)) {
			result.Bot = "supibot"
		}

		if global {
			result.ChatPfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", result.Chat, result.Chat)
		}

		playerIDs = append(playerIDs, result.PlayerID)

		recordWeight[result.PlayerID] = result
	}

	if global {
		// Get the weight per chat for the chatters above the limit
		rows, err = pool.Query(context.Background(), `
		select playerid, sum(weight) as weight, chat
		from fish
		where playerid = any($1)
		and date < $2
		and date > $3
		group by playerid, chat
		`, playerIDs, date, date2)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error querying database")
			return recordWeight, err
		}

		results, err = pgx.CollectRows(rows, pgx.RowToStructByNameLax[BoardData])
		if err != nil && err != pgx.ErrNoRows {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error collecting rows")
			return recordWeight, err
		}

		for _, result := range results {

			pfp := fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", result.Chat, result.Chat)

			existingFishInfo, exists := recordWeight[result.PlayerID]
			if exists {

				if existingFishInfo.ChatWeights == nil {
					existingFishInfo.ChatWeights = make(map[string]float64)
				}
				existingFishInfo.ChatWeights[pfp] += result.Weight

				recordWeight[result.PlayerID] = existingFishInfo
			}
		}

	}

	return recordWeight, nil
}

func printTotalWeight(filePath string, recordWeight map[int]BoardData, oldRecordWeight map[int]BoardData, title string, global bool, weightlimit float64) error {

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

	_, _ = fmt.Fprintln(file, "| Rank | Player | Total Weight in lbs |"+func() string {
		if global {
			return " Chat |"
		}
		return ""
	}())
	_, err = fmt.Fprintln(file, "|------|--------|---------|"+func() string {
		if global {
			return "-------|"
		}
		return ""
	}())
	if err != nil {
		return err
	}

	rank := 1
	prevRank := 1
	prevWeight := 0.0
	occupiedRanks := make(map[int]int)

	sortedWeightRecords := sortMapIntFishInfo(recordWeight, "totalweightdesc")

	for _, playerID := range sortedWeightRecords {
		chatweights := recordWeight[playerID].ChatWeights
		weight := recordWeight[playerID].TotalWeight
		player := recordWeight[playerID].Player

		// Increment rank only if the count has changed
		if weight != float64(prevWeight) {
			rank += occupiedRanks[rank]
			occupiedRanks[rank] = 1
		} else {
			rank = prevRank
			occupiedRanks[rank]++
		}

		var found bool

		oldWeight := weight
		oldRank := -1

		if info, ok := oldRecordWeight[playerID]; ok {
			found = true
			oldWeight = info.TotalWeight
			oldRank = info.Rank
		}

		// update and store the rank
		if copy, ok := recordWeight[playerID]; ok {

			copy.Rank = rank

			recordWeight[playerID] = copy
		}

		changeEmoji := ChangeEmoji(rank, oldRank, found)

		var totalweight string

		weightDifference := weight - oldWeight

		if weightDifference > 0.01 {
			totalweight = fmt.Sprintf("%.2f (+%.2f)", weight, weightDifference)
		} else {
			totalweight = fmt.Sprintf("%.2f", weight)
		}

		botIndicator := ""
		if recordWeight[playerID].Bot == "supibot" && !recordWeight[playerID].Verified {
			botIndicator = "*"
		}

		ranks := Ranks(rank)

		_, _ = fmt.Fprintf(file, "| %s %s | %s%s | %s |", ranks, changeEmoji, player, botIndicator, totalweight)

		if global {
			ChatWeightsSlice := make([]struct {
				chat   string
				weight float64
			}, 0, 2)

			for k, v := range chatweights {
				ChatWeightsSlice = append(ChatWeightsSlice, struct {
					chat   string
					weight float64
				}{k, v})
			}

			sort.Slice(ChatWeightsSlice, func(i, j int) bool {
				return ChatWeightsSlice[i].chat < ChatWeightsSlice[j].chat
			})

			for _, weight := range ChatWeightsSlice {
				_, _ = fmt.Fprintf(file, " %s %.2f ", weight.chat, weight.weight)
			}
			_, _ = fmt.Fprint(file, "|")
		}

		_, err = fmt.Fprintln(file)
		if err != nil {
			return err
		}

		prevWeight = weight
		prevRank = rank

	}

	_, _ = fmt.Fprintf(file, "\n_Only showing fishers with a total weight of >= %v lbs_\n", weightlimit)

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
