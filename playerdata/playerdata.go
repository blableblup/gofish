package playerdata

import (
	"context"
	"fmt"
	"gofish/utils"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

func GetPlayerID(pool *pgxpool.Pool, playerName string, firstFishDate time.Time, firstFishChat string) (int, error) {
	if err := utils.EnsureTableExists(pool, "playerdata"); err != nil {
		return 0, err
	}

	// Check if the player already exists in the playerdata table
	var playerID int
	err := pool.QueryRow(context.Background(), "SELECT playerid FROM playerdata WHERE name = $1", playerName).Scan(&playerID)
	if err == nil {
		return playerID, nil // Player already exists, return their ID
	}

	// Player doesn't exist, add them to the playerdata table
	err = pool.QueryRow(context.Background(), "INSERT INTO playerdata (name, firstfishdate, firstfishchat) VALUES ($1, $2, $3) RETURNING playerid", playerName, firstFishDate, firstFishChat).Scan(&playerID)
	if err != nil {
		return 0, err
	}

	fmt.Printf("Added player '%s' to the playerdata table. First fish caught on %s in chat '%s'.\n", playerName, firstFishDate, firstFishChat)

	return playerID, nil
}