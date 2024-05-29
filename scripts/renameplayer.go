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
		logs.Logs().Error().Err(err).Msgf("Error connecting to the database")
		return err
	}
	defer pool.Close()

	// Start a transaction
	tx, err := pool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())

	for _, pair := range namePairs {
		oldName := pair.OldName
		newName := pair.NewName

		logs.Logs().Info().Msgf("Updating player name from '%s' to '%s'", oldName, newName)

		// Get player IDs
		var oldPlayerID, newPlayerID int

		err = tx.QueryRow(context.Background(), `
		SELECT playerid FROM playerdata WHERE name = $1
			`, oldName).Scan(&oldPlayerID)
		if err != nil {
			if err == pgx.ErrNoRows {
				logs.Logs().Warn().Msgf("No player found with name '%s'", oldName)
			} else {
				logs.Logs().Error().Err(err).Msgf("Error retrieving player ID for name '%s'", oldName)
			}
			continue
		}

		err = tx.QueryRow(context.Background(), `
		SELECT playerid FROM playerdata WHERE name = $1
			`, newName).Scan(&newPlayerID)
		if err != nil {
			if err == pgx.ErrNoRows {
				// If the player renamed but never caught a fish since renaming. This only updates the old name in playerdata
				var confirm string
				logs.Logs().Warn().Msgf("Player '%s' does not have an entry in the playerdata table. Is the name correct? (y/n): ", newName)
				_, err = fmt.Scanln(&confirm)
				if err != nil {
					return err
				}

				if confirm != "y" {
					logs.Logs().Info().Msg("Player not renamed")
					continue
				}

				// Update player names and oldnames
				result, err := tx.Exec(context.Background(), `
					UPDATE playerdata
					SET name = $1, oldnames = CONCAT(oldnames, ' ', CAST($2 AS TEXT))
					WHERE playerid = $3
				`, newName, oldName, oldPlayerID)
				if err != nil {
					return fmt.Errorf("error updating player data for player %s: %v", newName, err)
				}

				rowsAffected := result.RowsAffected()
				if rowsAffected == 0 {
					logs.Logs().Fatal().Msgf("No rows updated for player %s in playerdata. Exiting the program due to potential data inconsistency.", newName)
					// There should be an update unless something is wrong with the data
				} else {
					logs.Logs().Info().Msgf("Player data updated for player %s", newName)
				}

				break
			} else {
				logs.Logs().Error().Err(err).Msgf("Error retrieving player ID for name '%s'", newName)
			}
			continue
		}

		// Update player names and oldnames
		result, err := tx.Exec(context.Background(), `
			UPDATE playerdata
			SET name = $1, oldnames = CONCAT(oldnames, ' ', CAST($2 AS TEXT))
			WHERE playerid = $3		
			`, newName, oldName, oldPlayerID)
		if err != nil {
			logs.Logs().Error().Err(err).Msgf("error updating player data for player %s", newName)
		}

		rowsAffected := result.RowsAffected()
		if rowsAffected == 0 {
			logs.Logs().Fatal().Msgf("No rows updated for player %s in playerdata. Exiting the program due to potential data inconsistency.", newName)
			// There should be an update unless something is wrong with the data
		} else {
			logs.Logs().Info().Msgf("Player data updated for player %s", newName)
		}

		// Update playerid in fish + tournament tables
		result, err = tx.Exec(context.Background(), `
            UPDATE fish
            SET playerid = $1
            WHERE playerid = $2
        `, oldPlayerID, newPlayerID)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		if rowsAffected == 0 {
			logs.Logs().Fatal().Msgf("No rows updated for player %s in fish table. Exiting the program due to potential data inconsistency.", newName)
			// There should be an update unless something is wrong with the data
		} else {
			logs.Logs().Info().Msgf("Rows affected in fish table for player %s: %d", newName, rowsAffected)
		}

		for chatName, chat := range config.Chat {
			if !chat.CheckEnabled {
				if chatName != "global" && chatName != "default" {
					logs.Logs().Warn().Msgf("Skipping chat '%s' because check_enabled is false", chatName)
				}
				continue
			}

			tableName := "tournaments" + chatName

			var exists bool
			err := pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE lower(table_name) = lower($1))", tableName).Scan(&exists)
			if err != nil {
				return err
			}

			if !exists {
				logs.Logs().Warn().Msgf("Tournament table '%s' does not exist skipping...", tableName)
				continue
			}

			result, err := tx.Exec(context.Background(), fmt.Sprintf(`
			UPDATE %s
			SET playerid = $1
			WHERE playerid = $2
		`, tableName), oldPlayerID, newPlayerID)
			if err != nil {
				return err
			}
			rowsAffected = result.RowsAffected()
			if rowsAffected == 0 {
				logs.Logs().Warn().Msgf("No rows updated for player %s in tournament table '%s'", newName, tableName)
				// Because players wont have an entry in every tournament database for every chat, this doesnt need to be fatal
			} else {
				logs.Logs().Info().Msgf("Rows affected in tournament table '%s' for player %s: %d", tableName, newName, rowsAffected)
			}

		}

		// Delete redundant entry
		result, err = tx.Exec(context.Background(), `
            DELETE FROM playerdata
            WHERE playerid = $1
        `, newPlayerID)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		if rowsAffected == 0 {
			logs.Logs().Fatal().Msgf("No rows updated for player %s after deletion. Exiting the program due to potential data inconsistency.", newName)
			// There should be an update unless something is wrong with the data
		} else {
			logs.Logs().Info().Msgf("Rows affected in playerdata table for player %s after deletion: %d", newName, rowsAffected)
		}

	}

	// Commit the transaction
	err = tx.Commit(context.Background())
	if err != nil {
		return err
	}

	return nil
}
