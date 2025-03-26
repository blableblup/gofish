package scripts

import (
	"context"
	"gofish/logs"
	"gofish/playerdata"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func VerifiedPlayers(pool *pgxpool.Pool) {

	verifiedPlayers := playerdata.ReadVerifiedPlayers()

	tx, err := pool.Begin(context.Background())
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error starting transaction")
		return
	}
	defer tx.Rollback(context.Background())

	verificationStatus := make(map[string]bool)

	date := "2023-09-15"

	// Get the player data for supibot fishers
	rows, err := tx.Query(context.Background(), `
        SELECT name, oldnames, firstfishdate FROM playerdata
		where firstfishdate < $1`, date)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Retrieving playerdata")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var playerName string
		var oldnames []string
		var firstFishDate time.Time
		if err := rows.Scan(&playerName, &oldnames, &firstFishDate); err != nil {
			logs.Logs().Error().Err(err).Msg("Error scanning row")
			return
		}

		// Check if player or one of their old names is in the verified players list
		verified := false
		for _, verifiedPlayer := range verifiedPlayers {
			if verifiedPlayer == playerName {
				verified = true
				break
			}
			for _, name := range oldnames {
				if verifiedPlayer == name {
					logs.Logs().Info().
						Str("Player", playerName).
						Str("OldName", name).
						Msg("Player was verified with an old name")
					verified = true
					break
				}
			}
		}

		verificationStatus[playerName] = verified
	}

	if err := rows.Err(); err != nil {
		logs.Logs().Error().Err(err).Msg("Error iterating over player rows")
		return
	}

	// Update verified field for all players in a single transaction
	for playerName, verified := range verificationStatus {
		_, err = tx.Exec(context.Background(), `
            UPDATE playerdata
            SET verified = $1
            WHERE name = $2
        `, verified, playerName)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", playerName).
				Bool("Verified", verified).
				Msg("Error updating verified field for player")
			return
		}
		logs.Logs().Info().
			Str("Player", playerName).
			Bool("Verified", verified).
			Msg("Verified field updated for player")
	}

	// Commit the transaction
	err = tx.Commit(context.Background())
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error committing transaction")
		return
	}
}
