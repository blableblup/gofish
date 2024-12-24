package scripts

import (
	"context"
	"fmt"
	"gofish/logs"
	"gofish/utils"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Updates fish names in fish and fishinfo, needs to be oldFish:newFish

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

func UpdateFishNames(pool *pgxpool.Pool, namePairs []struct{ OldName, NewName string }) error {

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
			Msg("Updating fish name")

		// Update fishname in fishinfo
		result, err := tx.Exec(context.Background(), `
			UPDATE fishinfo
			SET fishname = $1
			WHERE fishname = $2	
			`, newName, oldName)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("OldName", oldName).
				Str("NewName", newName).
				Msg("Error updating fish name in fishinfo")
			return err
		}

		rowsAffected := result.RowsAffected()
		if rowsAffected == 0 {
			logs.Logs().Fatal().
				Str("OldName", oldName).
				Str("NewName", newName).
				Msg("No rows updated for fish in fishinfo")
			// There should be an update unless something is wrong with the data
			// ...or you misspelled, there isnt a check if the fish with the "oldname" exists in the table
		} else {
			logs.Logs().Info().
				Str("OldName", oldName).
				Str("NewName", newName).
				Int64("Rows Affected", rowsAffected).
				Msg("Fish name updated for fish")
		}

		// Update fishname in fish
		result, err = tx.Exec(context.Background(), `
            UPDATE fish
            SET fishname = $1
            WHERE fishname = $2
        `, newName, oldName)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("OldName", oldName).
				Str("NewName", newName).
				Msg("Error updating fishname in fish table")
			return err
		}
		rowsAffected = result.RowsAffected()
		if rowsAffected == 0 {
			// No need for this to be fatal since you can rename a fish which has never been caught before
			// Just make sure that it has actually never been caught
			logs.Logs().Warn().
				Str("OldName", oldName).
				Str("NewName", newName).
				Msg("No rows updated for fishname in fish table.")
		} else {
			logs.Logs().Info().
				Int64("Rows Affected", rowsAffected).
				Msg("Updated fishname in fish table")
		}
	}

	// This is just in case something is weird. But not really needed.
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
