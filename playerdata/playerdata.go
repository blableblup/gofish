package playerdata

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gofish/logs"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

func GetPlayerID(pool *pgxpool.Pool, player string, firstFishDate time.Time, firstFishChat string) (int, error) {

	var playerID int
	var twitchID sql.NullInt64

	// If a player renamed and hasnt caught a fish since the rename...
	// this wont find the player in the api and will return 0
	apiID, err := GetTwitchID(player)
	if err != nil && !errors.Is(err, ErrNoPlayerFound) {
		logs.Logs().Error().Err(err).
			Str("Player", player).
			Msg("Error getting twitch id for player")
		return 0, err
	}

	err = pool.QueryRow(context.Background(), "SELECT playerid, twitchid FROM playerdata WHERE name = $1", player).Scan(&playerID, &twitchID)
	if err == nil {
		if twitchID.Valid {
			// If player already exists and the twitchid is the same, return their player ID
			// If the api cant find the twitchid, but the player has a non null twitchid entry: they likely renamed but havent caught a fish with their new name yet
			if int(twitchID.Int64) == apiID || apiID == 0 {
				return playerID, nil
			}
		} else {
			// This should only happen if you recheck old logs and one of the 35 players without a twitchid caught a fish
			if apiID == 0 {
				logs.Logs().Warn().
					Str("Player", player).
					Int("PlayerID", playerID).
					Int("TwitchID", apiID).
					Msg("A player does not have a twitchID")
				return playerID, nil
			}
		}
	} else if err != pgx.ErrNoRows {
		return 0, err
	}

	// That players name isnt in playerdata
	if err == pgx.ErrNoRows {

		playerID, err = Asdfjsadgaiga(apiID, player, firstFishDate, firstFishChat, pool)
		if err != nil {
			return 0, err
		}

		return playerID, nil

	}

	// Same name but different twitch id means that the player took someone elses name who fished before
	if int(twitchID.Int64) != apiID {

		logs.Logs().Warn().
			Str("Date", firstFishDate.Format(time.RFC3339)).
			Str("Chat", firstFishChat).
			Int("TwitchID", apiID).
			Str("Player", player).
			Msg("Player took someone elses name")

		playerID, err = Asdfjsadgaiga(apiID, player, firstFishDate, firstFishChat, pool)
		if err != nil {
			return 0, err
		}

		return playerID, nil

	}

	return playerID, nil
}

// To check if the player renamed, is an old name or is completely new
func Asdfjsadgaiga(apiID int, player string, firstFishDate time.Time, firstFishChat string, pool *pgxpool.Pool) (int, error) {

	renamed, isoldname, oldname, playerID, err := DidPlayerRename(apiID, player, pool)
	if err != nil {
		logs.Logs().Error().Err(err).
			Int("TwitchID", apiID).
			Str("Player", player).
			Msg("Error checking if player renamed")
		return 0, err
	}

	if renamed {

		// Rename them and return their id
		err = RenamePlayer(player, oldname, apiID, pool)
		if err != nil {
			logs.Logs().Error().Err(err).
				Int("TwitchID", apiID).
				Int("PlayerID", playerID).
				Str("Player", player).
				Str("OldName", oldname).
				Msg("Error renaming player")
			return 0, err
		}

		return playerID, nil

	}

	if isoldname {

		// Return their playerid
		return playerID, nil
	}

	// Add the new player if they didnt rename and arent an old name
	playerID, err = AddNewPlayer(apiID, player, firstFishDate, firstFishChat, pool)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Date", firstFishDate.Format(time.RFC3339)).
			Str("Chat", firstFishChat).
			Int("TwitchID", apiID).
			Str("Player", player).
			Int("PlayerID", playerID).
			Msg("Error adding player to playerdata")
		return 0, err
	}
	return playerID, nil

}

func DidPlayerRename(twitchid int, player string, pool *pgxpool.Pool) (bool, bool, string, int, error) {

	// Check if an entry with that twitchid exists
	var playerID int
	var lastoldname string
	err := pool.QueryRow(context.Background(), "SELECT name, playerid FROM playerdata WHERE twitchid = $1", twitchid).Scan(&lastoldname, &playerID)
	if err == nil {
		logs.Logs().Warn().
			Str("LastOldName", lastoldname).
			Str("Player", player).
			Int("PlayerID", playerID).
			Int("TwitchID", twitchid).
			Msg("A player renamed")
		return true, false, lastoldname, playerID, nil // The player renamed
	} else if err != pgx.ErrNoRows {
		return false, false, lastoldname, playerID, err
	}

	// Check if the player name is an old name for a player
	// This wont work though if the name is an old name for multiple players -.- then the data gets mixed up ?
	err = pool.QueryRow(context.Background(), "SELECT name, playerid FROM playerdata WHERE $1 = ANY(STRING_TO_ARRAY(oldnames, ' '))", player).Scan(&lastoldname, &playerID)
	if err == nil {
		logs.Logs().Warn().
			Str("CurrentName", lastoldname).
			Str("Player", player).
			Int("PlayerID", playerID).
			Msg("A player is an old name")
		return false, true, lastoldname, playerID, nil // The player renamed
	} else if err != pgx.ErrNoRows {
		return false, false, lastoldname, playerID, err
	}

	return false, false, lastoldname, playerID, nil
}

func AddNewPlayer(twitchid int, player string, firstFishDate time.Time, firstFishChat string, pool *pgxpool.Pool) (int, error) {

	// Add a new player and return their id
	// For older logs: If a players twitchid cannot be found in the api...
	// twitchid is left empty so that we dont have multiple players with the same twitchid (0)
	// Can maybe run updatetwitchids to check for it afterwards
	var playerID int
	if twitchid == 0 {
		err := pool.QueryRow(context.Background(), "INSERT INTO playerdata (name,  firstfishdate, firstfishchat) VALUES ($1, $2, $3) RETURNING playerid", player, firstFishDate, firstFishChat).Scan(&playerID)
		if err != nil {
			return 0, err
		}
		logs.Logs().Info().
			Str("Date", firstFishDate.Format(time.RFC3339)).
			Str("Chat", firstFishChat).
			Str("TwitchID", "no TwitchID found").
			Str("Player", player).
			Int("PlayerID", playerID).
			Msg("Added new player to playerdata")
	} else {
		err := pool.QueryRow(context.Background(), "INSERT INTO playerdata (name, twitchid, firstfishdate, firstfishchat) VALUES ($1, $2, $3, $4) RETURNING playerid", player, twitchid, firstFishDate, firstFishChat).Scan(&playerID)
		if err != nil {
			return 0, err
		}
		logs.Logs().Info().
			Str("Date", firstFishDate.Format(time.RFC3339)).
			Str("Chat", firstFishChat).
			Int("TwitchID", twitchid).
			Str("Player", player).
			Int("PlayerID", playerID).
			Msg("Added new player to playerdata")
	}

	return playerID, nil
}

func RenamePlayer(newName string, oldName string, twitchid int, pool *pgxpool.Pool) error {

	// Update the player in playerdata
	_, err := pool.Exec(context.Background(), `
			UPDATE playerdata
			SET name = $1, oldnames = CONCAT(oldnames, ' ', CAST($2 AS TEXT))
			WHERE twitchid = $3		
			`, newName, oldName, twitchid)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("OldName", oldName).
			Str("NewName", newName).
			Msg("Error updating player data for name")
		return err
	}

	logs.Logs().Info().
		Str("OldName", oldName).
		Str("NewName", newName).
		Int("TwitchID", twitchid).
		Msg("Renamed player")

	return nil
}

// This is now only used for the leaderboards, so that it doesnt show NEW after a player renamed
// But could reuse the same code as above for the leaderboards ?
// And need to update the leaderboards to allow for the same name to appear multiple times :D
var loggedPlayers = make(map[string]bool)

func PlayerRenamed(player string, pool *pgxpool.Pool) (string, error) {
	var newPlayer string

	// Check if the player exists in the playerdata table
	// This doesnt consider the case where two players in the databse have the same name
	// Maybe one player caught a fish and then renamed and never caught a fish again and then another one renamed to that name ?
	err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE name = $1", player).Scan(&player)
	if err != nil {

		// Check if the name is an old name for a player
		query := `
            SELECT name
            FROM playerdata
            WHERE $1 = ANY(STRING_TO_ARRAY(oldnames, ' '))
        `
		rows, err := pool.Query(context.Background(), query, player)
		if err != nil {
			logs.Logs().Error().Err(err).Str("Player", player).Msg("Error querying for old names for player")
			return player, err
		}
		defer rows.Close()

		matchingPlayers := make([]string, 0)
		for rows.Next() {
			var matchingPlayer string
			if err := rows.Scan(&matchingPlayer); err != nil {
				logs.Logs().Error().Err(err).Str("Player", player).Msg("Error scanning player for player")
				return player, err
			}
			matchingPlayers = append(matchingPlayers, matchingPlayer)
		}

		if len(matchingPlayers) == 0 {
			logs.Logs().Warn().Str("Player", player).Msg("Unknown player on leaderboard")
			return player, nil // This should never happen
		}

		// So that you dont get spammed with log messages if you check old logs again
		if len(matchingPlayers) == 1 {
			newPlayer := matchingPlayers[0]

			if !loggedPlayers[player] {
				logs.Logs().Info().Str("Old Name", player).Str("New Name", newPlayer).Msg("Player was previously renamed")

				loggedPlayers[player] = true
			}

			return newPlayer, nil
		}

		// This is needed if the name is an old name for multiple players
		// Could happen if someone renames and then someone else renames to their old name and then also renames
		for {
			logs.Logs().Info().Msgf("Player '%s' renamed to one of the following names:", player)
			for i, name := range matchingPlayers {
				fmt.Printf("%d. %s\n", i+1, name)
			}

			logs.Logs().Info().Msg("Enter the number corresponding to the correct new name: ")
			reader := bufio.NewReader(os.Stdin)
			choiceStr, _ := reader.ReadString('\n')
			choiceStr = strings.TrimSpace(choiceStr)
			choice, err := strconv.Atoi(choiceStr)
			if err != nil || choice < 1 || choice > len(matchingPlayers) {
				logs.Logs().Warn().Msgf("Enter a valid number (ㆆ_ㆆ).")
				continue
			}

			newPlayer = matchingPlayers[choice-1]
			if !loggedPlayers[player] {
				logs.Logs().Info().Str("Old Name", player).Str("New Name", newPlayer).Msg("Player was previously renamed")

				loggedPlayers[player] = true
			}
			return newPlayer, nil
		}
	}

	return player, nil
}
