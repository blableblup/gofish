package scripts

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/playerdata"
	"time"
)

func VerifiedPlayers() {
	pool, err := data.Connect()
	if err != nil {
		fmt.Println("Error connecting to the database:", err)
		return
	}
	defer pool.Close()

	verifiedPlayers := playerdata.ReadVerifiedPlayers()

	tx, err := pool.Begin(context.Background())
	if err != nil {
		fmt.Println("Error starting transaction:", err)
		return
	}
	defer tx.Rollback(context.Background())

	// Prepare a map to store the verification status for each player
	verificationStatus := make(map[string]bool)

	// Get player data names and their first fish date
	rows, err := tx.Query(context.Background(), `
        SELECT name, firstfishdate FROM playerdata
    `)
	if err != nil {
		fmt.Println("Error retrieving player data:", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var playerName string
		var firstFishDate time.Time
		if err := rows.Scan(&playerName, &firstFishDate); err != nil {
			fmt.Println("Error scanning row:", err)
			continue
		}

		// Check if player's first fish date is before 2023-09-15
		if firstFishDate.After(time.Date(2023, 9, 15, 0, 0, 0, 0, time.UTC)) {
			continue // Skip players whose first fish date is after the specified date
		}

		// Check if player is in the verified players list
		verified := false
		for _, verifiedPlayer := range verifiedPlayers {
			if verifiedPlayer == playerName {
				verified = true
				break
			}
		}

		// Store the verification status for the player
		verificationStatus[playerName] = verified
	}

	if err := rows.Err(); err != nil {
		fmt.Println("Error iterating over player data rows:", err)
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
			fmt.Printf("Error updating verified field for player %s: %v\n", playerName, err)
			return
		}
		fmt.Printf("Verified field set to %v for player %s\n", verified, playerName)
	}

	// Commit the transaction
	err = tx.Commit(context.Background())
	if err != nil {
		fmt.Println("Error committing transaction:", err)
		return
	}
}
