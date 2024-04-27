package playerdata

import (
	"bufio"
	"context"
	"fmt"
	"gofish/utils"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

func GetPlayerID(pool *pgxpool.Pool, playerName string, firstFishDate time.Time, firstFishChat string) (int, error) {
	if err := utils.EnsureTableExists(pool, "playerdata"); err != nil {
		return 0, err
	}

	var playerID int
	err := pool.QueryRow(context.Background(), "SELECT playerid FROM playerdata WHERE name = $1", playerName).Scan(&playerID)
	if err == nil {
		return playerID, nil // Player already exists, return their ID
	} else if err != pgx.ErrNoRows {
		return 0, err
	}

	// Check if they renamed first
	newPlayer := PlayerLeaderboard(playerName, pool)

	if newPlayer == playerName {
		// Player doesn't exist, add them to the playerdata table
		err = pool.QueryRow(context.Background(), "INSERT INTO playerdata (name, firstfishdate, firstfishchat) VALUES ($1, $2, $3) RETURNING playerid", playerName, firstFishDate, firstFishChat).Scan(&playerID)
		if err != nil {
			return 0, err
		}
		fmt.Printf("Added player '%s' to the playerdata table. First fish caught on %s in chat '%s'.\n", playerName, firstFishDate, firstFishChat)

	} else { // If they were renamed before but the database wasnt updated so they still caught a fish with their old name, or if you recheck old logs
		err := pool.QueryRow(context.Background(), "SELECT playerid FROM playerdata WHERE name = $1", newPlayer).Scan(&playerID)
		if err != nil {
			return 0, err
		}
	}

	return playerID, nil
}

func PlayerLeaderboard(player string, pool *pgxpool.Pool) string {
	var newPlayer string

	// Check if the player exists in the playerdata table
	// This doesnt consider the case where two players in the databse have the same name
	// Maybe one player caught a fish and then renamed and never caught a fish again and then another one renamed to that name ?
	err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE name = $1", player).Scan(&player)
	if err != nil {
		fmt.Printf("Player name '%s' cannot be found as a current name in the playerdata table. Checking if they renamed...\n", player)

		// Check if the name is an old name for a player
		query := `
            SELECT name
            FROM playerdata
            WHERE $1 = ANY(STRING_TO_ARRAY(oldnames, ' '))
        `
		rows, err := pool.Query(context.Background(), query, player)
		if err != nil {
			fmt.Printf("Error querying for old names for player '%s': %v\n", player, err)
			return player
		}
		defer rows.Close()

		matchingPlayers := make([]string, 0)
		for rows.Next() {
			var matchingPlayer string
			if err := rows.Scan(&matchingPlayer); err != nil {
				fmt.Printf("Error scanning player for player '%s': %v\n", player, err)
				continue
			}
			matchingPlayers = append(matchingPlayers, matchingPlayer)
		}

		if len(matchingPlayers) == 0 {
			fmt.Printf("Player '%s' also doesn't appear as an old name.\n", player)
			return player // If the player is new (for GetPlayerID) or if the player was renamed incorrectly
		}

		if len(matchingPlayers) == 1 {
			newPlayer = matchingPlayers[0]
			fmt.Printf("Player '%s' renamed to '%s'.\n", player, newPlayer)
			return newPlayer
		}

		// Old Twitch names can become available after 6 months so it is better having this even if it might never be needed
		for {
			fmt.Printf("Player '%s' renamed to one of the following names:\n", player)
			for i, name := range matchingPlayers {
				fmt.Printf("%d. %s\n", i+1, name)
			}

			fmt.Print("Enter the number corresponding to the correct new name: ")
			reader := bufio.NewReader(os.Stdin)
			choiceStr, _ := reader.ReadString('\n')
			choiceStr = strings.TrimSpace(choiceStr)
			choice, err := strconv.Atoi(choiceStr)
			if err != nil || choice < 1 || choice > len(matchingPlayers) {
				fmt.Println("Enter a valid number (ㆆ_ㆆ).")
				continue
			}

			newPlayer = matchingPlayers[choice-1]
			fmt.Printf("Player '%s' renamed to '%s'.\n", player, newPlayer)
			return newPlayer
		}
	}

	return player
}
