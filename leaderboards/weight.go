package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
)

func processWeight(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	config := params.Config
	date2 := params.Date2
	title := params.Title
	chat := params.Chat
	pool := params.Pool
	date := params.Date
	path := params.Path
	mode := params.Mode

	var filePath, titleweight string
	var weightlimit float64
	var rows pgx.Rows
	var isGlobal bool

	if chatName == "global" {
		isGlobal = true
	} else {
		isGlobal = false
	}

	if path == "" {
		filePath = filepath.Join("leaderboards", chatName, "weight.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", chatName, path)
	}

	oldRecordWeight, err := ReadWeightRankings(filePath, pool)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error reading old leaderboard")
		return
	}

	weightlimit = chat.Weightlimit
	if weightlimit == 0 {
		weightlimit = config.Chat["default"].Weightlimit
	}

	recordWeight := make(map[string]data.FishInfo)

	// Query the database to get the biggest fish per player for the specific chat or globally
	if !isGlobal {
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
			return
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
				Msg("Error querying database")
			return
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
			return
		}

		err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Player)
		if err != nil {
			logs.Logs().Error().Err(err).
				Int("PlayerID", fishInfo.PlayerID).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error retrieving player name for id")
			return
		}

		if fishInfo.Bot == "supibot" {
			err := pool.QueryRow(context.Background(), "SELECT verified FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Verified)
			if err != nil {
				logs.Logs().Error().Err(err).
					Int("PlayerID", fishInfo.PlayerID).
					Str("Board", board).
					Str("Chat", chatName).
					Msg("Error retrieving verified status for playerid")
				return
			}
		}

		err = pool.QueryRow(context.Background(), "SELECT fishtype FROM fishinfo WHERE fishname = $1", fishInfo.TypeName).Scan(&fishInfo.Type)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("FishName", fishInfo.TypeName).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error retrieving fish type for fish name")
			return
		}

		if isGlobal {
			fishInfo.ChatPfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", fishInfo.Chat, fishInfo.Chat)
		}

		recordWeight[fishInfo.Player] = fishInfo

	}

	if err := rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error iterating over query results")
		return
	}

	logRecord(recordWeight, oldRecordWeight, board)

	// Stops the program if it is in "just checking" mode
	if mode == "check" {
		logs.Logs().Info().
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Finished checking for new records")
		return
	}

	if title == "" {
		if !isGlobal {
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

	logs.Logs().Info().
		Str("Board", board).
		Str("Chat", chatName).
		Msg("Updating leaderboard")

	err = writeWeight(filePath, recordWeight, oldRecordWeight, titleweight, isGlobal)
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

func writeWeight(filePath string, recordWeight map[string]data.FishInfo, oldRecordWeight map[string]LeaderboardInfo, title string, isGlobal bool) error {

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

	_, _ = fmt.Fprintln(file, "| Rank | Player | Fish | Weight in lbs ⚖️ |"+func() string {
		if isGlobal {
			return " Chat |"
		}
		return ""
	}())
	_, err = fmt.Fprintln(file, "|------|--------|-----------|---------|"+func() string {
		if isGlobal {
			return "-------|"
		}
		return ""
	}())
	if err != nil {
		return err
	}

	sortedPlayers := SortMapByWeightDesc(recordWeight)

	for _, player := range sortedPlayers {
		weight := recordWeight[player].Weight
		fishType := recordWeight[player].Type
		fishName := recordWeight[player].TypeName
		rank := recordWeight[player].Rank

		var found bool

		oldWeight := weight
		oldRank := -1

		if info, ok := oldRecordWeight[player]; ok {
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
		if recordWeight[player].Bot == "supibot" && !recordWeight[player].Verified {
			botIndicator = "*"
		}

		ranks := Ranks(rank)

		// Write the leaderboard row
		_, _ = fmt.Fprintf(file, "| %s %s | %s%s | %s %s | %s |", ranks, changeEmoji, player, botIndicator, fishType, fishName, fishweight)
		if isGlobal {
			_, _ = fmt.Fprintf(file, " %s |", recordWeight[player].ChatPfp)
		}
		_, err = fmt.Fprintln(file)
		if err != nil {
			return err
		}

	}

	_, _ = fmt.Fprintf(file, "\n_Last updated at %s_", time.Now().In(time.UTC).Format("2006-01-02 15:04:05 UTC"))

	return nil
}
