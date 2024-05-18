package leaderboards

import (
	"context"
	"gofish/data"
	"gofish/logs"
	"path/filepath"
	"time"
)

func RunWeightGlobal(params LeaderboardParams) {
	config := params.Config
	pool := params.Pool
	mode := params.Mode

	globalRecordWeight := make(map[string]data.FishInfo)
	filePath := filepath.Join("leaderboards", "global", "weight.md")
	oldWeight, err := ReadWeightRankings(filePath, pool)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error reading old global weight leaderboard")
		return
	}

	WeightLimit := config.Chat["global"].Weightlimit

	// Query the database to get the biggest fish per player
	rows, err := pool.Query(context.Background(), `
		SELECT f.playerid, f.weight, f.fishname, f.bot, f.chat AS chatname, f.date, f.catchtype, f.fishid, f.chatid
		FROM fish f
		JOIN (
			SELECT playerid, MAX(weight) AS max_weight
			FROM fish 
			GROUP BY playerid
		) max_fish ON f.playerid = max_fish.playerid AND f.weight = max_fish.max_weight
		WHERE f.weight >= $1`, WeightLimit)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error querying database")
		return
	}
	defer rows.Close()

	// Iterate through the query results
	for rows.Next() {
		var fishInfo data.FishInfo

		if err := rows.Scan(&fishInfo.PlayerID, &fishInfo.Weight, &fishInfo.TypeName, &fishInfo.Bot,
			&fishInfo.Chat, &fishInfo.Date, &fishInfo.CatchType, &fishInfo.FishId, &fishInfo.ChatId); err != nil {
			logs.Logs().Error().Err(err).Msg("Error scanning row")
			continue
		}

		err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Player)
		if err != nil {
			logs.Logs().Error().Err(err).Msgf("Error retrieving player name for id '%d'", fishInfo.PlayerID)
			continue
		}

		err = pool.QueryRow(context.Background(), "SELECT fishtype FROM fishinfo WHERE fishname = $1", fishInfo.TypeName).Scan(&fishInfo.Type)
		if err != nil {
			logs.Logs().Error().Err(err).Msgf("Error retrieving fish type for fish name '%s'", fishInfo.TypeName)
			continue
		}

		fishInfo.Chat = config.Chat[fishInfo.Chat].Emoji
		globalRecordWeight[fishInfo.Player] = fishInfo

	}

	if err := rows.Err(); err != nil {
		logs.Logs().Error().Err(err).Msg("Error iterating over query results")
		return
	}

	for player, weightrecord := range globalRecordWeight {
		oldweightrecord, exists := oldWeight[player]
		if !exists {
			logs.Logs().Info().
				Str("Date", weightrecord.Date.Format(time.RFC3339)).
				Str("Chat", weightrecord.Chat).
				Float64("Weight", weightrecord.Weight).
				Str("TypeName", weightrecord.TypeName).
				Str("CatchType", weightrecord.CatchType).
				Str("FishType", weightrecord.Type).
				Str("Player", weightrecord.Player).
				Int("ChatID", weightrecord.ChatId).
				Int("FishID", weightrecord.FishId).
				Msg("New Record Weight for Player")
		} else {
			if weightrecord.Weight > oldweightrecord.Weight {
				logs.Logs().Info().
					Str("Date", weightrecord.Date.Format(time.RFC3339)).
					Str("Chat", weightrecord.Chat).
					Float64("Weight", weightrecord.Weight).
					Str("TypeName", weightrecord.TypeName).
					Str("CatchType", weightrecord.CatchType).
					Str("FishType", weightrecord.Type).
					Str("Player", weightrecord.Player).
					Int("ChatID", weightrecord.ChatId).
					Int("FishID", weightrecord.FishId).
					Msg("Updated Record Weight for Player")
			}
		}
	}

	if mode == "check" {
		logs.Logs().Info().Msg("Finished checking for new global weight records")
		return
	}

	updateWeightLeaderboard(globalRecordWeight, oldWeight, filePath)
}

func updateWeightLeaderboard(recordWeight map[string]data.FishInfo, oldWeight map[string]LeaderboardInfo, filePath string) {
	logs.Logs().Info().Msg("Updating global weight leaderboard...")
	title := "### Biggest fish caught per player globally\n"
	isGlobal := true
	err := writeWeight(filePath, recordWeight, oldWeight, title, isGlobal)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error writing global weight leaderboard")
	} else {
		logs.Logs().Info().Msg("Global weight leaderboard updated successfully")
	}
}
