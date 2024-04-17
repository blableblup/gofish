package scripts

import (
	"context"
	"fmt"
	"gofish/data"
	"os"
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
	// Insert fish data into the database
	pool, err := data.Connect()
	if err != nil {
		fmt.Println("Error connecting to the database:", err)
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

		fmt.Printf("Updating player name from '%s' to '%s'.\n", oldName, newName)

		// Get player IDs
		var oldPlayerID, newPlayerID int

		err = tx.QueryRow(context.Background(), `
		SELECT playerid FROM playerdata WHERE name = $1
			`, oldName).Scan(&oldPlayerID)
		if err != nil {
			if err == pgx.ErrNoRows {
				fmt.Printf("No player found with name '%s'.\n", oldName)
			} else {
				fmt.Printf("Error retrieving player ID for name '%s': %v\n", oldName, err)
			}
			continue
		}

		err = tx.QueryRow(context.Background(), `
		SELECT playerid FROM playerdata WHERE name = $1
			`, newName).Scan(&newPlayerID)
		if err != nil {
			if err == pgx.ErrNoRows {
				var confirm string // If the player renamed but never caught a fish since renaming. This only updates the old name in playerdata
				fmt.Printf("Player '%s' does not have an entry in the playerdata table. Is the name correct? (yes/no): ", newName)
				_, err = fmt.Scanln(&confirm)
				if err != nil {
					return err
				}

				if confirm != "yes" {
					fmt.Println("Player not renamed.")
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

				// Check if any rows were affected by the update operation
				rowsAffected := result.RowsAffected()
				if rowsAffected == 0 {
					fmt.Printf("No rows updated for player %s\n", newName)
					fmt.Println("Exiting the program due to potential data inconsistency.")
					os.Exit(1) // There should be an update unless something is wrong with the data
				}

				fmt.Printf("Player data updated for player %s\n", newName)
				break
			} else {
				fmt.Printf("Error retrieving player ID for name '%s': %v\n", newName, err)
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
			return fmt.Errorf("error updating player data for player %s: %v", newName, err)
		}

		// Check if any rows were affected by the update operation
		rowsAffected := result.RowsAffected()
		if rowsAffected == 0 {
			fmt.Printf("No rows updated for player %s\n", newName)
			fmt.Println("Exiting the program due to potential data inconsistency.")
			os.Exit(1) // There should be an update unless something is wrong with the data
		}

		fmt.Printf("Player data updated for player %s\n", newName)

		// Update playerid in fish table
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
			fmt.Printf("No rows updated for player %s\n", newName)
			fmt.Println("Exiting the program due to potential data inconsistency.")
			os.Exit(1) // There should be an update unless something is wrong with the data
		}

		fmt.Printf("Rows affected in fish table for player %s: %d\n", newName, rowsAffected)

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
			fmt.Printf("No rows updated for player %s\n", newName)
			fmt.Println("Exiting the program due to potential data inconsistency.")
			os.Exit(1) // There should be an update unless something is wrong with the data
		}

		fmt.Printf("Rows affected in playerdata table for player %s after deletion: %d\n", newName, rowsAffected)
	}

	// Commit the transaction
	err = tx.Commit(context.Background())
	if err != nil {
		return err
	}

	return nil
}
