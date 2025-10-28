package scripts

import (
	"context"
	"fmt"
	"gofish/logs"
	"gofish/utils"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// need to rename pfp + entry in config + name of folder if the channel has leaderboards
// and need to also update all the global boards and profiles so that chat name changes there
// idk would prob be better to use twitchid everywhere idk
func UpdateChannelName(pool *pgxpool.Pool, namePairs []struct{ OldName, NewName string }) error {

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

		err = UpdateNames(tx, "fish", "chat", oldName, newName)
		if err != nil {
			return err
		}

		err = UpdateNames(tx, "bag", "chat", oldName, newName)
		if err != nil {
			return err
		}

		err = UpdateNames(tx, "tournaments", "chat", oldName, newName)
		if err != nil {
			return err
		}

		err = UpdateNames(tx, "playerdata", "firstfishchat", oldName, newName)
		if err != nil {
			return err
		}

	}

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

	err = tx.Commit(context.Background())
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error committing transaction")
		return err
	}

	return nil
}

func UpdateNames(tx pgx.Tx, tableName string, what string, oldName string, newName string) error {

	query := fmt.Sprintf(`update %s set %s = $1 where %s = $2`,
		tableName,
		what,
		what,
	)

	result, err := tx.Exec(context.Background(), query, newName, oldName)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Table", tableName).
			Msg("Error updating name in fish table")
		return err
	}
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {

		logs.Logs().Warn().
			Str("Table", tableName).
			Msg("No rows updated in table")
	} else {
		logs.Logs().Info().
			Int64("Rows Affected", rowsAffected).
			Str("Table", tableName).
			Msg("Updated name in table")
	}

	return nil
}
