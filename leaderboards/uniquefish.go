package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func processUniqueFish(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	config := params.Config
	global := params.Global
	title := params.Title
	limit := params.Limit
	chat := params.Chat
	path := params.Path
	mode := params.Mode

	var filePath, titleunique string
	var uniquelimit int

	if path == "" {
		filePath = filepath.Join("leaderboards", chatName, "uniquefish.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", chatName, path)
	}

	olduniquefishy, err := getJsonBoard(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error reading old leaderboard")
		return
	}

	if limit == "" {
		uniquelimit = chat.Uniquelimit
		if uniquelimit == 0 {
			uniquelimit = config.Chat["default"].Uniquelimit
		}
	} else {
		uniquelimit, err = strconv.Atoi(limit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Limit", limit).
				Str("Board", board).
				Msg("Error converting custom limit to int")
			return
		}
	}

	uniquefishy, err := getUnique(params, uniquelimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error getting leaderboard")
		return
	}

	AreMapsSame := didPlayerMapsChange(params, olduniquefishy, uniquefishy)

	if AreMapsSame && mode != "force" {
		logs.Logs().Warn().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Not updating board because there are no changes")
		return
	}

	if title == "" {
		if !global {
			if strings.HasSuffix(chatName, "s") {
				titleunique = fmt.Sprintf("### Players who have seen the most fish in %s' chat\n", chatName)
			} else {
				titleunique = fmt.Sprintf("### Players who have seen the most fish in %s's chat\n", chatName)
			}
		} else {
			titleunique = "### Players who have seen the most fish globally\n"
		}
	} else {
		titleunique = fmt.Sprintf("%s\n", title)
	}

	logs.Logs().Info().
		Str("Board", board).
		Str("Chat", chatName).
		Msg("Updating leaderboard")

	err = writeCount(filePath, uniquefishy, olduniquefishy, titleunique, global, board, uniquelimit)
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

func getUnique(params LeaderboardParams, uniquelimit int) (map[int]data.FishInfo, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	global := params.Global
	date2 := params.Date2
	pool := params.Pool
	date := params.Date

	uniquefishy := make(map[int]data.FishInfo)
	var rows pgx.Rows
	var err error

	if !global {
		rows, err = pool.Query(context.Background(), `
		SELECT playerid, unique_fish_caught
		FROM (
		SELECT playerid, COUNT(DISTINCT fishname) as unique_fish_caught
		FROM fish
		WHERE chat = $1
		AND date < $2
		AND date > $3
		GROUP BY playerid
		) as subquery
		WHERE unique_fish_caught >= $4`, chatName, date, date2, uniquelimit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error querying database")
			return uniquefishy, err
		}
		defer rows.Close()
	} else {
		rows, err = pool.Query(context.Background(), `
		SELECT playerid, unique_fish_caught
		FROM (
		SELECT playerid, COUNT(DISTINCT fishname) as unique_fish_caught
		FROM fish
		WHERE date < $1
		AND date > $2
		GROUP BY playerid
		) as subquery
		WHERE unique_fish_caught >= $3`, date, date2, uniquelimit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error querying database")
			return uniquefishy, err
		}
		defer rows.Close()
	}

	for rows.Next() {
		var fishInfo data.FishInfo

		if err := rows.Scan(&fishInfo.PlayerID, &fishInfo.Count); err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error scanning row for unique fish caught")
			return uniquefishy, err
		}

		err = pool.QueryRow(context.Background(), "SELECT name, firstfishdate FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Player, &fishInfo.Date)
		if err != nil {
			logs.Logs().Error().Err(err).
				Int("PlayerID", fishInfo.PlayerID).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error retrieving player name for id")
			return uniquefishy, err
		}

		if fishInfo.Date.Before(time.Date(2023, time.September, 14, 0, 0, 0, 0, time.UTC)) {
			fishInfo.Bot = "supibot"
			err := pool.QueryRow(context.Background(), "SELECT verified FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Verified)
			if err != nil {
				logs.Logs().Error().Err(err).
					Int("PlayerID", fishInfo.PlayerID).
					Str("Board", board).
					Str("Chat", chatName).
					Msg("Error retrieving verified status for playerid")
				return uniquefishy, err
			}
		}

		uniquefishy[fishInfo.PlayerID] = fishInfo
	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error iterating over rows")
		return uniquefishy, err
	}

	if global {
		// Get the unique fish caught per chat for the chatters above the uniquelimit
		rows, err = pool.Query(context.Background(), `
		select playerid, chat, count(distinct fishname)
		from fish
		where date < $1
		and date > $2
		group by playerid, chat
		order by count desc`, date, date2)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error querying database")
			return uniquefishy, err
		}
		defer rows.Close()

		for rows.Next() {
			var fishInfo data.FishInfo

			if err := rows.Scan(&fishInfo.PlayerID, &fishInfo.Chat, &fishInfo.Count); err != nil {
				logs.Logs().Error().Err(err).
					Str("Chat", chatName).
					Str("Board", board).
					Msg("Error scanning row for unique fish caught")
				return uniquefishy, err
			}

			pfp := fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", fishInfo.Chat, fishInfo.Chat)

			existingFishInfo, exists := uniquefishy[fishInfo.PlayerID]
			if exists {

				if existingFishInfo.ChatCounts == nil {
					existingFishInfo.ChatCounts = make(map[string]int)
				}
				existingFishInfo.ChatCounts[pfp] += fishInfo.Count

				uniquefishy[fishInfo.PlayerID] = existingFishInfo
			}
		}

		if err = rows.Err(); err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error iterating over rows")
			return uniquefishy, err
		}
	}

	return uniquefishy, nil
}
