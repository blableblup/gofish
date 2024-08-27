package scripts

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"gofish/utils"
	"strings"

	"github.com/jackc/pgx/v4"
)

func ProcessRenamePairs(renamePairs string) ([]struct{ OldName, NewName string }, error) {
	// Split renamePairs into pairs
	renamePairsSlice := strings.Split(renamePairs, ",")
	var namePairs []struct{ OldName, NewName string }
	for _, pair := range renamePairsSlice {
		names := strings.Split(pair, ":")
		if len(names) != 2 {
			return nil, fmt.Errorf("invalid pair format: %s", pair)
		}
		namePairs = append(namePairs, struct{ OldName, NewName string }{OldName: names[0], NewName: names[1]})
	}
	return namePairs, nil
}

func UpdatePlayerNames(namePairs []struct{ OldName, NewName string }) error {

	config := utils.LoadConfig()

	pool, err := data.Connect()
	if err != nil {
		logs.Logs().Error().Err(err).
			Msgf("Error connecting to the database")
		return err
	}
	defer pool.Close()

	// Start a transaction
	tx, err := pool.Begin(context.Background())
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error starting transaction")
		return err
	}
	defer tx.Rollback(context.Background())

	for _, pair := range namePairs {
		oldName := pair.OldName
		newName := pair.NewName

		logs.Logs().Info().
			Str("OldName", oldName).
			Str("NewName", newName).
			Msg("Updating player name")

		// Get player IDs
		var oldPlayerID, newPlayerID int

		err = tx.QueryRow(context.Background(), `
		SELECT playerid FROM playerdata WHERE name = $1
			`, oldName).Scan(&oldPlayerID)
		if err != nil {
			if err == pgx.ErrNoRows {
				logs.Logs().Warn().
					Str("OldName", oldName).
					Msg("No player found with old name")
				return nil
			} else {
				logs.Logs().Error().Err(err).
					Str("OldName", oldName).
					Msg("Error retrieving player ID for name")
				return err
			}
		}

		err = tx.QueryRow(context.Background(), `
		SELECT playerid FROM playerdata WHERE name = $1
			`, newName).Scan(&newPlayerID)
		if err != nil {
			if err == pgx.ErrNoRows {
				// If the player renamed but never caught a fish since renaming. This only updates the old name in playerdata
				logs.Logs().Warn().
					Str("NewName", newName).
					Msg("Player does not have an entry in the playerdata table. ")

				confirm, err := utils.Confirm("Is the name correct? (y to continue, n to exit)")
				if err != nil {
					logs.Logs().Error().Err(err).
						Msg("Error reading input")
					return err
				}

				if !confirm {
					logs.Logs().Info().
						Msg("Exiting program")
					return nil
				}

				// Update player names and oldnames
				result, err := tx.Exec(context.Background(), `
					UPDATE playerdata
					SET name = $1, oldnames = CONCAT(oldnames, ' ', CAST($2 AS TEXT))
					WHERE playerid = $3
				`, newName, oldName, oldPlayerID)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("OldName", oldName).
						Str("NewName", newName).
						Int("OldID", oldPlayerID).
						Msg("Error updating player data for name")
					return err
				}

				rowsAffected := result.RowsAffected()
				if rowsAffected == 0 {
					logs.Logs().Fatal().
						Str("OldName", oldName).
						Str("NewName", newName).
						Int("OldID", oldPlayerID).
						Msg("No rows updated for player in playerdata.")
					// There should be an update unless something is wrong with the data
				} else {
					logs.Logs().Info().
						Str("OldName", oldName).
						Str("NewName", newName).
						Int("OldID", oldPlayerID).
						Int64("Rows Affected", rowsAffected).
						Msg("Player data updated for player")
				}

				break
			} else {
				logs.Logs().Error().Err(err).
					Str("OldName", oldName).
					Str("NewName", newName).
					Int("OldID", oldPlayerID).
					Msg("Error retrieving player ID for new name")
				return err
			}
		}

		// Update player names and oldnames
		result, err := tx.Exec(context.Background(), `
			UPDATE playerdata
			SET name = $1, oldnames = CONCAT(oldnames, ' ', CAST($2 AS TEXT))
			WHERE playerid = $3		
			`, newName, oldName, oldPlayerID)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("OldName", oldName).
				Str("NewName", newName).
				Int("OldID", oldPlayerID).
				Int("NewID", newPlayerID).
				Msg("Error updating player data for name")
			return err
		}

		rowsAffected := result.RowsAffected()
		if rowsAffected == 0 {
			logs.Logs().Fatal().
				Str("OldName", oldName).
				Str("NewName", newName).
				Int("OldID", oldPlayerID).
				Int("NewID", newPlayerID).
				Msg("No rows updated for player in playerdata.")
			// There should be an update unless something is wrong with the data
		} else {
			logs.Logs().Info().
				Str("OldName", oldName).
				Str("NewName", newName).
				Int("OldID", oldPlayerID).
				Int("NewID", newPlayerID).
				Int64("Rows Affected", rowsAffected).
				Msg("Player data updated for player")
		}

		// Update playerid in fish + tournament tables
		result, err = tx.Exec(context.Background(), `
            UPDATE fish
            SET playerid = $1
            WHERE playerid = $2
        `, oldPlayerID, newPlayerID)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("OldName", oldName).
				Str("NewName", newName).
				Int("OldID", oldPlayerID).
				Int("NewID", newPlayerID).
				Msg("Error updating playerids in fish table")
			return err
		}
		rowsAffected = result.RowsAffected()
		if rowsAffected == 0 {
			logs.Logs().Fatal().
				Str("OldName", oldName).
				Str("NewName", newName).
				Int("OldID", oldPlayerID).
				Int("NewID", newPlayerID).
				Msg("No rows updated for player in fish table.")
			// There should be an update unless something is wrong with the data
		} else {
			logs.Logs().Info().
				Int64("Rows Affected", rowsAffected).
				Msg("Updated playerids in fish table")
		}

		for chatName, chat := range config.Chat {
			if !chat.CheckTData {
				if chatName != "global" && chatName != "default" {
					logs.Logs().Warn().
						Str("Chat", chatName).
						Msgf("Skipping chat because checktdata is false")
				}
				continue
			}

			tableName := "tournaments" + chatName

			var exists bool
			err := pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE lower(table_name) = lower($1))", tableName).Scan(&exists)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Table", tableName).
					Msg("Error checking if the tournament table exists")
				return err
			}

			if !exists {
				logs.Logs().Warn().
					Str("Table", tableName).
					Msg("Tournament table does not exist skipping...")
				continue
			}

			result, err := tx.Exec(context.Background(), fmt.Sprintf(`
			UPDATE %s
			SET playerid = $1
			WHERE playerid = $2
		`, tableName), oldPlayerID, newPlayerID)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("OldName", oldName).
					Str("NewName", newName).
					Int("OldID", oldPlayerID).
					Int("NewID", newPlayerID).
					Str("Table", tableName).
					Msg("Error updating playerids in tournament table")
				return err
			}
			rowsAffected = result.RowsAffected()
			if rowsAffected == 0 {
				logs.Logs().Warn().
					Str("Table", tableName).
					Msgf("No rows updated for player in tournament table")
				// Because players wont have an entry in every tournament database for every chat, this doesnt need to be fatal
			} else {
				logs.Logs().Info().
					Str("Table", tableName).
					Int64("Rows Affected", rowsAffected).
					Msg("Updated playerids in tournament table")
			}

		}

		// Delete redundant entry
		result, err = tx.Exec(context.Background(), `
            DELETE FROM playerdata
            WHERE playerid = $1
        `, newPlayerID)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("OldName", oldName).
				Str("NewName", newName).
				Int("OldID", oldPlayerID).
				Int("NewID", newPlayerID).
				Msg("Error deleting new player entry in playerdata")
			return err
		}
		rowsAffected = result.RowsAffected()
		if rowsAffected == 0 {
			logs.Logs().Fatal().
				Str("OldName", oldName).
				Str("NewName", newName).
				Int("OldID", oldPlayerID).
				Int("NewID", newPlayerID).
				Msg("No rows updated in playerdata after deleting new player entry.")
			// There should be an update unless something is wrong with the data
		} else {
			logs.Logs().Info().
				Int64("Rows Affected", rowsAffected).
				Msg("Deleted new player entry in playerdata")
		}

	}

	// This is just in case something is weird. But not really needed. Editing the db manually is annoying so everything here has to be done correctly
	confirm, err := utils.Confirm("Continue with the transaction? (y to continue, n to exit)")
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error reading input")
		return err
	}

	if confirm {
		logs.Logs().Info().
			Msg("Committing transaction...")

	} else {
		logs.Logs().Info().
			Msg("Exiting program")
		return nil
	}

	// Commit the transaction
	err = tx.Commit(context.Background())
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error committing transaction")
		return err
	}

	return nil
}
