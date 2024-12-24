package playerdata

import (
	"context"
	"database/sql"
	"errors"
	"gofish/logs"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetPlayerID(pool *pgxpool.Pool, player string, firstFishDate time.Time, firstFishChat string) (int, error) {

	var playerID int
	var twitchID sql.NullInt64

	// Returns 0 if no player found in api
	// Problem: When rechecking old logs, this GetPlayerID function will assign the wrong playerid to players, who renamed and whose name is now used by another account
	// Maybe if someone hasnt caught a fish in 6 months make sure that thats still the same fisher if that name is an old name and the name is being actively used by a player?
	apiID, err := GetTwitchID(player)
	if err != nil && !errors.Is(err, ErrNoPlayerFound) {
		logs.Logs().Error().Err(err).
			Str("Player", player).
			Msg("Error getting twitch id for player")
		return 0, err
	}

	rows, err := pool.Query(context.Background(), "SELECT playerid, twitchid FROM playerdata WHERE name = $1", player)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Player", player).
			Msg("Error quering playerid and twitchid for player")
		return 0, err
	}
	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&playerID, &twitchID); err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error scanning playerid and twitchid for player")
			return 0, err
		}

		if twitchID.Valid {
			// If player already exists and the twitchid is the same, return their player ID
			if int(twitchID.Int64) == apiID {
				return playerID, nil
			}
			// If a player with that name was found, but the apiID and the twitchID in the table are different
			// This will always log though, until one of the players with that name renames again
			if int(twitchID.Int64) != apiID {
				logs.Logs().Warn().
					Str("Player", player).
					Int("TwitchID", apiID).
					Msg("Player is using a name which was already used by a different person before")
			}
			// If the api cant find the twitchid, but the player has a non null twitchid entry: they likely renamed but havent caught a fish with their new name yet
			if apiID == 0 {
				logs.Logs().Warn().
					Str("Player", player).
					Int("PlayerID", playerID).
					Int("TwitchID", int(twitchID.Int64)).
					Msg("A player cannot be found in the API, but has an entry")
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
	}

	// If the playerid didnt get returned yet, the player is new or renamed
	playerID, err = Asdfjsadgaiga(apiID, player, firstFishDate, firstFishChat, pool)
	if err != nil {
		return 0, err
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
		err = RenamePlayer(player, oldname, apiID, playerID, pool)
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

	var playerID int
	var lastoldname string

	// Check if an entry with that twitchid exists if the twitchid isnt 0
	if twitchid != 0 {
		err := pool.QueryRow(context.Background(), "SELECT name, playerid FROM playerdata WHERE twitchid = $1", twitchid).Scan(&lastoldname, &playerID)
		if err == nil {
			logs.Logs().Warn().
				Str("LastOldName", lastoldname).
				Str("Player", player).
				Int("PlayerID", playerID).
				Int("TwitchID", twitchid).
				Msg("A player renamed")
			return true, false, lastoldname, playerID, nil
		} else if err != pgx.ErrNoRows {
			return false, false, lastoldname, playerID, err
		}
	}

	// Check if the player name is an old name for a player if there is no twitchid found for that name
	// This wont work though if the name is an old name for multiple players -.- then the data gets mixed up ?
	if twitchid == 0 {
		err := pool.QueryRow(context.Background(), "SELECT name, playerid FROM playerdata WHERE $1 = ANY(STRING_TO_ARRAY(oldnames, ' '))", player).Scan(&lastoldname, &playerID)
		if err == nil {
			logs.Logs().Warn().
				Str("CurrentName", lastoldname).
				Str("Player", player).
				Int("PlayerID", playerID).
				Msg("A player is an old name")
			return false, true, lastoldname, playerID, nil
		} else if err != pgx.ErrNoRows {
			return false, false, lastoldname, playerID, err
		}
	}

	return false, false, lastoldname, playerID, nil
}

func AddNewPlayer(twitchid int, player string, firstFishDate time.Time, firstFishChat string, pool *pgxpool.Pool) (int, error) {

	// Add a new player and return their id
	// If a players twitchid cannot be found in the api, twitchid is left empty so that we dont have multiple players with the same twitchid (0)
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

func RenamePlayer(newName string, oldName string, twitchid int, playerid int, pool *pgxpool.Pool) error {

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
			Int("TwitchID", twitchid).
			Int("PlayerID", playerid).
			Msg("Error updating player data for name")
		return err
	}

	logs.Logs().Info().
		Str("OldName", oldName).
		Str("NewName", newName).
		Int("TwitchID", twitchid).
		Int("PlayerID", playerid).
		Msg("Renamed player")

	return nil
}
