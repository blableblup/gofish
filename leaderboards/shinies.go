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
)

func processShinies(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	title := params.Title
	path := params.Path
	mode := params.Mode

	var filePath string

	if path == "" {
		filePath = filepath.Join("leaderboards", "global", "shiny.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", "global", path)
	}

	oldShinies, err := getJsonBoard(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error reading old leaderboard")
		return
	}

	Shinies, err := getShinies(params)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error getting leaderboard")
		return
	}

	AreMapsSame := didPlayerMapsChange(params, oldShinies, Shinies)

	if AreMapsSame && mode != "force" {
		logs.Logs().Warn().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Not updating board because there are no changes")
		return
	}

	if title == "" {
		title = "### A list of shinies\n"
	} else {
		title = fmt.Sprintf("%s\n", title)
	}

	global := true
	weightlimit := 0.0
	err = writeFishList(filePath, Shinies, oldShinies, title, global, board, weightlimit)
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

	err = writeRaw(filePath, Shinies)
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

func getShinies(params LeaderboardParams) (map[int]data.FishInfo, error) {
	board := params.LeaderboardType
	date2 := params.Date2
	date := params.Date
	pool := params.Pool

	Shinies := make(map[int]data.FishInfo)

	// This will work if there are multiple shinies for the same fishtype; trim, because there is one space in front of the shiny string when adding them (fix that?)
	rows, err := pool.Query(context.Background(), `
		select f.fishid, f.chatid, f.fishtype, f.fishname, f.weight, f.catchtype, f.playerid, f.date, f.bot, f.chat 
		from fish f
		join(
		select STRING_TO_ARRAY(trim(' ' from shiny), ' ') as shiny_list
		from fishinfo
		where shiny != ''
		) shinyfish on f.fishtype = any(shiny_list)
		where date < $1
		and date > $2`, date, date2)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Msg("Error querying database")
		return Shinies, err
	}
	defer rows.Close()

	for rows.Next() {
		var fishInfo data.FishInfo

		if err := rows.Scan(&fishInfo.FishId, &fishInfo.ChatId, &fishInfo.Type, &fishInfo.TypeName, &fishInfo.Weight, &fishInfo.CatchType,
			&fishInfo.PlayerID, &fishInfo.Date, &fishInfo.Bot, &fishInfo.Chat); err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Msg("Error scanning row")
			return Shinies, err
		}

		fishInfo.Player, _, fishInfo.Verified, err = PlayerStuff(fishInfo.PlayerID, params, pool)
		if err != nil {
			return Shinies, err
		}

		fishInfo.ChatPfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", fishInfo.Chat, fishInfo.Chat)
		fishInfo.Type = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/shiny/%s.png)", fishInfo.Type, fishInfo.Type)

		Shinies[fishInfo.FishId] = fishInfo

	}

	if err := rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Msg("Error iterating over query results")
		return Shinies, err
	}

	return Shinies, nil
}

func writeFishList(filePath string, fishy map[int]data.FishInfo, oldFishy map[int]data.FishInfo, title string, global bool, board string, weightlimit float64) error {

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

	_, _ = fmt.Fprintln(file, "| # | Player | Fish | Weight in lbs ⚖️ | Date in UTC |"+func() string {
		if global {
			return " Chat |"
		}
		return ""
	}())
	_, err = fmt.Fprintln(file, "|-----|------|--------|-----------|---------|"+func() string {
		if global {
			return "-------|"
		}
		return ""
	}())
	if err != nil {
		return err
	}

	rank := len(fishy) + 1

	sortedFish := sortFishRecords2(fishy)

	for _, fishid := range sortedFish {

		rank--

		changeEmoji := "🆕"

		_, ok := oldFishy[fishid]
		if ok {
			changeEmoji = " "
		}

		botIndicator := ""
		if fishy[fishid].Bot == "supibot" && !fishy[fishid].Verified {
			botIndicator = "*"
		}

		_, _ = fmt.Fprintf(file, "| %d %s | %s%s | %s %s | %v | %s |",
			rank, changeEmoji, fishy[fishid].Player, botIndicator, fishy[fishid].Type, fishy[fishid].TypeName, fishy[fishid].Weight, fishy[fishid].Date.Format("2006-01-02 15:04:05"))
		if global {
			_, _ = fmt.Fprintf(file, " %s |", fishy[fishid].ChatPfp)
		}
		_, err = fmt.Fprintln(file)
		if err != nil {
			return err
		}
	}

	if board == "records" || board == "recordsglobal" {
		_, _ = fmt.Fprintf(file, "\n_Only showing fish weighing >= %v lbs_\n", weightlimit)
	}

	_, _ = fmt.Fprintf(file, "\n_Last updated at %s_", time.Now().In(time.UTC).Format("2006-01-02 15:04:05 UTC"))

	return nil
}
