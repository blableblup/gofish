package leaderboards

import (
	"context"
	"fmt"
	"gofish/logs"

	"github.com/jackc/pgx/v5"
)

func processShinies(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	title := params.Title
	mode := params.Mode

	filePath := returnPath(params)

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

func getShinies(params LeaderboardParams) (map[int]BoardData, error) {
	board := params.LeaderboardType
	date2 := params.Date2
	date := params.Date
	pool := params.Pool

	Shinies := make(map[int]BoardData)

	// This will work if there are multiple shinies for the same fishtype
	rows, err := pool.Query(context.Background(), `
		select f.fishid, f.chatid, f.fishtype, f.fishname, f.weight, f.catchtype, f.playerid, f.date, f.bot, f.chat 
		from fish f
		join(
		select shiny
		from fishinfo
		where shiny != '{}'
		) shinyfish on f.fishtype = any(shiny)
		where date < $1
		and date > $2`, date, date2)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Msg("Error querying database")
		return Shinies, err
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[BoardData])
	if err != nil && err != pgx.ErrNoRows {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Msg("Error collecting rows")
		return Shinies, err
	}

	for _, result := range results {

		result.Player, _, result.Verified, _, err = PlayerStuff(result.PlayerID, params, pool)
		if err != nil {
			return Shinies, err
		}

		result.ChatPfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", result.Chat, result.Chat)
		result.FishType = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/shiny/%s.png)", result.FishType, result.FishType)

		Shinies[result.FishId] = result

	}

	return Shinies, nil
}

func writeFishList(filePath string, fishy map[int]BoardData, oldFishy map[int]BoardData, title string, global bool, board string, weightlimit float64) error {

	header := []string{"#", "Player", "Fish", "Weight in lbs", "Date in UTC"}

	if global {
		header = append(header, "Chat")
	}

	rank := len(fishy) + 1

	sortedFish := sortMapIntFishInfo(fishy, "datedesc")

	var data [][]string

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

		row := []string{
			fmt.Sprintf("%d %s", rank, changeEmoji),
			fmt.Sprintf("%s%s", fishy[fishid].Player, botIndicator),
			fmt.Sprintf("%s %s", fishy[fishid].FishType, fishy[fishid].FishName),
			fmt.Sprint(fishy[fishid].Weight),
			fishy[fishid].Date.Format("2006-01-02 15:04:05"),
		}

		if global {
			row = append(row, fishy[fishid].ChatPfp)
		}

		data = append(data, row)

	}

	var notes []string

	if board == "records" || board == "recordsglobal" {
		notes = append(notes, fmt.Sprintf("Only showing fish weighing >= %v lbs", weightlimit))
	}

	err := writeBoard(filePath, title, header, data, notes)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Msg("Error writing leaderboard")
		return err
	}

	return nil
}
