package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func processWeight(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	config := params.Config
	global := params.Global
	title := params.Title
	limit := params.Limit
	chat := params.Chat
	path := params.Path
	mode := params.Mode

	var filePath, titleweight string
	var weightlimit float64

	if path == "" {
		filePath = filepath.Join("leaderboards", chatName, "weight.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", chatName, path)
	}

	oldRecordWeight, err := getJsonBoard(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error reading old leaderboard")
		return
	}

	if limit == "" {
		weightlimit = chat.Weightlimit
		if weightlimit == 0 {
			weightlimit = config.Chat["default"].Weightlimit
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

	recordWeight, err := getWeightRecords(params, weightlimit)
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

	// Stops the program if it is in "just checking" mode
	if mode == "check" {
		logs.Logs().Info().
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Finished checking for new records")
		return
	}

	if title == "" {
		if !global {
			if strings.HasSuffix(chatName, "s") {
				titleweight = fmt.Sprintf("### Biggest fish caught per player in %s' chat\n", chatName)
			} else {
				titleweight = fmt.Sprintf("### Biggest fish caught per player in %s's chat\n", chatName)
			}
		} else {
			titleweight = "### Biggest fish caught per player globally\n"
		}
	} else {
		titleweight = fmt.Sprintf("%s\n", title)
	}

	err = writeWeight(filePath, recordWeight, oldRecordWeight, titleweight, global, board, weightlimit)
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

	err = writeRaw(filePath, recordWeight)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error writing raw leaderboard")
	} else {
		logs.Logs().Info().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Raw leaderboard updated successfully")
	}
}

func getWeightRecords(params LeaderboardParams, weightlimit float64) (map[int]data.FishInfo, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	global := params.Global
	date2 := params.Date2
	pool := params.Pool
	date := params.Date

	recordWeight := make(map[int]data.FishInfo)
	var rows pgx.Rows
	var err error

	// Query the database to get the biggest fish per player for the specific chat or globally
	if !global {
		rows, err = pool.Query(context.Background(), `
		SELECT f.playerid, f.weight, f.fishname, f.bot, f.chat AS chatname, f.date, f.catchtype, f.fishid, f.chatid,
		RANK() OVER (ORDER BY f.weight DESC)
		FROM fish f
		JOIN (
			SELECT playerid, MAX(weight) AS max_weight
			FROM fish 
			WHERE chat = $1
			AND date < $3
	  		AND date > $4
			GROUP BY playerid
		) max_fish ON f.playerid = max_fish.playerid AND f.weight = max_fish.max_weight
		WHERE f.chat = $1 AND f.weight >= $2`, chatName, weightlimit, date, date2)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error querying database")
			return recordWeight, err
		}
		defer rows.Close()
	} else {
		rows, err = pool.Query(context.Background(), `
		SELECT f.playerid, f.weight, f.fishname, f.bot, f.chat AS chatname, f.date, f.catchtype, f.fishid, f.chatid,
		RANK() OVER (ORDER BY f.weight DESC)
		FROM fish f
		JOIN (
			SELECT playerid, MAX(weight) AS max_weight
			FROM fish 
			WHERE date < $1
			AND date > $2
			GROUP BY playerid
		) max_fish ON f.playerid = max_fish.playerid AND f.weight = max_fish.max_weight
		WHERE f.weight >= $3`, date, date2, weightlimit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error querying database")
			return recordWeight, err
		}
		defer rows.Close()
	}

	for rows.Next() {
		var fishInfo data.FishInfo

		if err := rows.Scan(&fishInfo.PlayerID, &fishInfo.Weight, &fishInfo.TypeName, &fishInfo.Bot,
			&fishInfo.Chat, &fishInfo.Date, &fishInfo.CatchType, &fishInfo.FishId, &fishInfo.ChatId, &fishInfo.Rank); err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error scanning row for biggest fish")
			return recordWeight, err
		}

		fishInfo.Player, _, fishInfo.Verified, err = PlayerStuff(fishInfo.PlayerID, params, pool)
		if err != nil {
			return recordWeight, err
		}

		fishInfo.Type, err = FishStuff(fishInfo.TypeName, params, pool)
		if err != nil {
			return recordWeight, err
		}

		if global {
			fishInfo.ChatPfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", fishInfo.Chat, fishInfo.Chat)
		}

		recordWeight[fishInfo.PlayerID] = fishInfo

	}

	if err := rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error iterating over query results")
		return recordWeight, err
	}

	return recordWeight, nil
}

func writeWeight(filePath string, recordWeight map[int]data.FishInfo, oldRecordWeight map[int]data.FishInfo, title string, global bool, board string, weightlimit float64) error {

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

	_, _ = fmt.Fprintln(file, "| Rank | Player | Fish | Weight in lbs | Date in UTC |"+func() string {
		if global {
			return " Chat |"
		}
		return ""
	}())
	_, err = fmt.Fprintln(file, "|------|--------|-----------|---------|-----|"+func() string {
		if global {
			return "-------|"
		}
		return ""
	}())
	if err != nil {
		return err
	}

	sortedWeightRecords := sortPlayerRecords(recordWeight)

	for _, playerID := range sortedWeightRecords {
		weight := recordWeight[playerID].Weight
		fishType := recordWeight[playerID].Type
		fishName := recordWeight[playerID].TypeName
		rank := recordWeight[playerID].Rank
		player := recordWeight[playerID].Player
		date := recordWeight[playerID].Date

		var found bool

		oldWeight := weight
		oldRank := -1

		if info, ok := oldRecordWeight[playerID]; ok {
			found = true
			oldWeight = info.Weight
			oldRank = info.Rank
		}

		changeEmoji := ChangeEmoji(rank, oldRank, found)

		var fishweight string

		weightDifference := weight - oldWeight

		if weightDifference > 0 {
			fishweight = fmt.Sprintf("%.2f (+%.2f)", weight, weightDifference)
		} else {
			fishweight = fmt.Sprintf("%.2f", weight)
		}

		botIndicator := ""
		if recordWeight[playerID].Bot == "supibot" && !recordWeight[playerID].Verified {
			botIndicator = "*"
		}

		ranks := Ranks(rank)

		// Write the leaderboard row
		_, _ = fmt.Fprintf(file, "| %s %s | %s%s | %s %s | %s | %s |", ranks, changeEmoji, player, botIndicator, fishType, fishName, fishweight, date.Format("2006-01-02 15:04:05"))
		if global {
			_, _ = fmt.Fprintf(file, " %s |", recordWeight[playerID].ChatPfp)
		}
		_, err = fmt.Fprintln(file)
		if err != nil {
			return err
		}

	}

	if board == "weight" || board == "weightglobal" {
		_, _ = fmt.Fprintf(file, "\n_Only showing fish weighing >= %v lbs_\n", weightlimit)
	}

	_, _ = fmt.Fprintf(file, "\n_Last updated at %s_", time.Now().In(time.UTC).Format("2006-01-02 15:04:05 UTC"))

	return nil
}
