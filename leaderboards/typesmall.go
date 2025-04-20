package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
)

func processTypeSmall(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	global := params.Global
	title := params.Title
	path := params.Path
	mode := params.Mode

	var filePath, titletype string

	if path == "" {
		filePath = filepath.Join("leaderboards", chatName, "typesmall.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", chatName, path)
	}

	oldType, err := getJsonBoardString(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error reading old leaderboard")
		return
	}

	recordType, err := getTypeRecordsSmall(params)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error getting type records")
		return
	}

	AreMapsSame := didFishMapChange(params, oldType, recordType)

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
				titletype = fmt.Sprintf("### Smallest fish per type caught in %s' chat\n", chatName)
			} else {
				titletype = fmt.Sprintf("### Smallest fish per type caught in %s's chat\n", chatName)
			}
		} else {
			titletype = "### Smallest fish per type caught globally\n"
		}
	} else {
		titletype = fmt.Sprintf("%s\n", title)
	}

	err = writeType(filePath, recordType, oldType, titletype, global)
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

	err = writeRawString(filePath, recordType)
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

func getTypeRecordsSmall(params LeaderboardParams) (map[string]data.FishInfo, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	global := params.Global
	date2 := params.Date2
	pool := params.Pool
	date := params.Date

	recordType := make(map[string]data.FishInfo)
	var rows pgx.Rows
	var err error

	// Query the database to get the smallest fish per type for the specific chat or globally
	// Fish you get from releasing and squirrels dont show their weight so they get ignored here
	// Ignoring them on the "biggest" type board wouldnt make a difference but could also do that there
	if !global {
		rows, err = pool.Query(context.Background(), `
		SELECT f.weight, f.fishname, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid,
		RANK() OVER (ORDER BY f.weight DESC)
		FROM fish f
		JOIN (
			SELECT fishname, MIN(weight) AS min_weight
			FROM fish 
			WHERE chat = $1
			AND date < $2
	  		AND date > $3
			AND catchtype != 'release'
			AND catchtype != 'squirrel'
			GROUP BY fishname
		) AS sub
		ON f.fishname = sub.fishname AND f.weight = sub.min_weight
		WHERE f.chat = $1
		AND f.date = (
			SELECT MIN(date)
			FROM fish
			WHERE fishname = sub.fishname AND weight = sub.min_weight AND chat = $1 AND catchtype != 'release' AND catchtype != 'squirrel'
		)`, chatName, date, date2)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error querying database")
			return recordType, err
		}
		defer rows.Close()
	} else {
		rows, err = pool.Query(context.Background(), `
		SELECT f.weight, f.fishname, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid,
		RANK() OVER (ORDER BY f.weight DESC)
		FROM fish f
		JOIN (
			SELECT fishname, MIN(weight) AS min_weight
			FROM fish 
			WHERE date < $1
	  		AND date > $2
			AND catchtype != 'release'
			AND catchtype != 'squirrel'
			GROUP BY fishname
		) AS sub
		ON f.fishname = sub.fishname AND f.weight = sub.min_weight
		AND f.date = (
			SELECT MIN(date)
			FROM fish
			WHERE fishname = sub.fishname AND weight = sub.min_weight AND catchtype != 'release' AND catchtype != 'squirrel'
		)`, date, date2)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error querying database")
			return recordType, err
		}
		defer rows.Close()
	}

	// Iterate through the query results
	for rows.Next() {
		var fishInfo data.FishInfo

		if err := rows.Scan(&fishInfo.Weight, &fishInfo.TypeName, &fishInfo.Bot,
			&fishInfo.Chat, &fishInfo.Date, &fishInfo.CatchType, &fishInfo.FishId, &fishInfo.ChatId, &fishInfo.PlayerID, &fishInfo.Rank); err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error scanning row")
			return recordType, err
		}

		fishInfo.Player, _, fishInfo.Verified, _, err = PlayerStuff(fishInfo.PlayerID, params, pool)
		if err != nil {
			return recordType, err
		}

		fishInfo.Type, err = FishStuff(fishInfo.TypeName, params, pool)
		if err != nil {
			return recordType, err
		}

		if global {
			fishInfo.ChatPfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", fishInfo.Chat, fishInfo.Chat)
		}

		recordType[fishInfo.TypeName] = fishInfo
	}

	if err := rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error iterating over query results")
		return recordType, err
	}

	return recordType, nil
}
