package leaderboards

import (
	"context"
	"gofish/data"
	"gofish/logs"
	"path/filepath"
	"time"
)

func RunTypeGlobal(params LeaderboardParams) {
	config := params.Config
	pool := params.Pool
	mode := params.Mode

	globalRecordType := make(map[string]data.FishInfo)
	filePath := filepath.Join("leaderboards", "global", "type.md")
	oldType, err := ReadTypeRankings(filePath, pool)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error reading old global type leaderboard")
		return
	}

	// Query the database to get the biggest fish per type
	rows, err := pool.Query(context.Background(), `
		SELECT f.weight, f.fishname, f.bot, f.chat AS chatname, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
			SELECT fishname, MAX(weight) AS max_weight
			FROM fish 
			GROUP BY fishname
		) AS sub
		ON f.fishname = sub.fishname AND f.weight = sub.max_weight
		AND f.fishid = (
			SELECT MIN(fishid)
			FROM fish
			WHERE fishname = sub.fishname AND weight = sub.max_weight
	)`)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error querying database")
		return
	}
	defer rows.Close()

	// Iterate through the query results
	for rows.Next() {
		var fishInfo data.FishInfo

		if err := rows.Scan(&fishInfo.Weight, &fishInfo.TypeName, &fishInfo.Bot, &fishInfo.Chat,
			&fishInfo.Date, &fishInfo.CatchType, &fishInfo.FishId, &fishInfo.ChatId, &fishInfo.PlayerID); err != nil {
			logs.Logs().Error().Err(err).Msg("Error scanning row")
			continue
		}

		err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Player)
		if err != nil {
			logs.Logs().Error().Err(err).Msgf("Error retrieving player name for id '%d'", fishInfo.PlayerID)
			continue
		}

		if fishInfo.Bot == "supibot" {
			err := pool.QueryRow(context.Background(), "SELECT verified FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Verified)
			if err != nil {
				logs.Logs().Error().Err(err).Msgf("Error retrieving verified status for playerid '%d'", fishInfo.PlayerID)
			}
		}

		err = pool.QueryRow(context.Background(), "SELECT fishtype FROM fishinfo WHERE fishname = $1", fishInfo.TypeName).Scan(&fishInfo.Type)
		if err != nil {
			logs.Logs().Error().Err(err).Msgf("Error retrieving fish type for fish name '%s'", fishInfo.TypeName)
			continue
		}

		fishInfo.Chat = config.Chat[fishInfo.Chat].Emoji
		globalRecordType[fishInfo.Type] = fishInfo
	}

	if err := rows.Err(); err != nil {
		logs.Logs().Error().Err(err).Msg("Error iterating over query results")
		return
	}

	for fishType, typerecord := range globalRecordType {
		oldtyperecord, exists := oldType[fishType]
		if !exists {
			logs.Logs().Info().
				Str("Date", typerecord.Date.Format(time.RFC3339)).
				Str("Chat", typerecord.Chat).
				Float64("Weight", typerecord.Weight).
				Str("TypeName", typerecord.TypeName).
				Str("CatchType", typerecord.CatchType).
				Str("FishType", typerecord.Type).
				Str("Player", typerecord.Player).
				Int("ChatID", typerecord.ChatId).
				Int("FishID", typerecord.FishId).
				Msg("New Record Weight for fishType")
		} else {
			if typerecord.Weight > oldtyperecord.Weight {
				logs.Logs().Info().
					Str("Date", typerecord.Date.Format(time.RFC3339)).
					Str("Chat", typerecord.Chat).
					Float64("Weight", typerecord.Weight).
					Str("TypeName", typerecord.TypeName).
					Str("CatchType", typerecord.CatchType).
					Str("FishType", typerecord.Type).
					Str("Player", typerecord.Player).
					Int("ChatID", typerecord.ChatId).
					Int("FishID", typerecord.FishId).
					Msg("Updated Record Weight for fishType")
			}
		}
	}

	if mode == "check" {
		logs.Logs().Info().Msg("Finished checking for new global type records")
		return
	}

	updateTypeLeaderboard(globalRecordType, oldType, filePath)
}

func updateTypeLeaderboard(recordType map[string]data.FishInfo, oldType map[string]LeaderboardInfo, filePath string) {
	logs.Logs().Info().Msg("Updating global type leaderboard...")
	title := "### Biggest fish per type caught globally\n"
	isGlobal := true
	err := writeType(filePath, recordType, oldType, title, isGlobal)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error writing global type leaderboard")
	} else {
		logs.Logs().Info().Msg("Global type leaderboard updated successfully")
	}
}
