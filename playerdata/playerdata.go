package playerdata

import (
	"context"
	"database/sql"
	"errors"
	"gofish/logs"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PossiblePlayer struct {
	FirstSeen time.Time
	LastSeen  time.Time
	TwitchID  sql.NullInt64
	PlayerID  int
	Player    string
	OldNames  []string
}

// >_________________________________________________________<
func FindAllThePossiblePlayers(pool *pgxpool.Pool, player string, firstFishDate time.Time, firstFishChat string) ([]PossiblePlayer, error) {

	var possiblePlayers []PossiblePlayer

	// Query for all players with that name
	rows, err := pool.Query(context.Background(), "SELECT playerid, twitchid, oldnames FROM playerdata WHERE name = $1", player)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Player", player).
			Msg("Error quering for player name")
		return []PossiblePlayer{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var possiblePlayer PossiblePlayer
		var oldnames sql.NullString
		possiblePlayer.Player = player

		if err := rows.Scan(&possiblePlayer.PlayerID, &possiblePlayer.TwitchID, &oldnames); err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error scanning row for player")
			return []PossiblePlayer{}, err
		}

		possiblePlayer.OldNames = strings.Split(oldnames.String, " ")

		possiblePlayer.LastSeen, possiblePlayer.FirstSeen, err = PlayerDates(pool, possiblePlayer.PlayerID, possiblePlayer.Player)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error getting last and first seen for player")
			return []PossiblePlayer{}, err
		}

		possiblePlayers = append(possiblePlayers, possiblePlayer)
	}

	// Query for all players which had that name as an oldname
	rows, err = pool.Query(context.Background(), "SELECT name, playerid, twitchid, oldnames FROM playerdata WHERE $1 = ANY(STRING_TO_ARRAY(oldnames, ' '))", player)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Player", player).
			Msg("Error quering for oldnames")
		return []PossiblePlayer{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var possiblePlayer PossiblePlayer
		var oldnames sql.NullString

		if err := rows.Scan(&possiblePlayer.Player, &possiblePlayer.PlayerID, &possiblePlayer.TwitchID, &oldnames); err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error scanning row for player")
			return []PossiblePlayer{}, err
		}

		possiblePlayer.OldNames = strings.Split(oldnames.String, " ")

		possiblePlayer.LastSeen, possiblePlayer.FirstSeen, err = PlayerDates(pool, possiblePlayer.PlayerID, player)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error getting last and first seen for player")
			return []PossiblePlayer{}, err
		}

		possiblePlayers = append(possiblePlayers, possiblePlayer)
	}

	var apiID int
	// If no possible players are found, check the api
	// i dont want to check the api for every player because that is really slow >_>_>>__>_> ?
	// wouldnt really change much, i would still need to do all those checks below
	if len(possiblePlayers) == 0 {
		apiID, err = GetTwitchID(player)
		if err != nil && !errors.Is(err, ErrNoPlayerFound) {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error getting twitch id for player")
			return []PossiblePlayer{}, err
		}

		if apiID != 0 {
			var possiblePlayer PossiblePlayer
			var oldnames sql.NullString
			err = pool.QueryRow(context.Background(),
				"SELECT name, playerid, twitchid, oldnames FROM playerdata WHERE twitchid = $1",
				apiID).Scan(&possiblePlayer.Player, &possiblePlayer.PlayerID, &possiblePlayer.TwitchID, &oldnames)
			if err == nil {

				// The player can be found in the api and there is a player in the db with that id
				// which means the player renamed
				err = RenamePlayer(player, possiblePlayer.Player, apiID, possiblePlayer.PlayerID, pool)
				if err != nil {
					logs.Logs().Error().Err(err).
						Int("TwitchID", apiID).
						Int("PlayerID", possiblePlayer.PlayerID).
						Str("Player", player).
						Str("OldName", possiblePlayer.Player).
						Msg("Error renaming player")
					return []PossiblePlayer{}, err
				}

				possiblePlayer.OldNames = strings.Split(oldnames.String, " ")

				possiblePlayers = append(possiblePlayers, possiblePlayer)

			} else if err != pgx.ErrNoRows {
				return []PossiblePlayer{}, err
			}
			// No player in the db with that twitchid, the player has to be new
			// player could also be a rename of one of the players in the db which dont have a twitchid
			// whatever
			if err == pgx.ErrNoRows {
				playerID, err := AddNewPlayer(apiID, player, firstFishDate, firstFishChat, pool)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Date", firstFishDate.Format(time.RFC3339)).
						Str("Chat", firstFishChat).
						Int("TwitchID", apiID).
						Str("Player", player).
						Msg("Error adding player to playerdata")
					return []PossiblePlayer{}, err
				}

				newplayer := PossiblePlayer{
					FirstSeen: firstFishDate,
					PlayerID:  playerID,
					Player:    player,
				}

				newplayer.TwitchID.Int64 = int64(apiID)

				possiblePlayers = append(possiblePlayers, newplayer)
			}
		} else {
			// has to be someone who renamed and then never caught a fish again with their new name
			// so the player is added to the db with "0" as their twitchid
			playerID, err := AddNewPlayer(apiID, player, firstFishDate, firstFishChat, pool)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Date", firstFishDate.Format(time.RFC3339)).
					Str("Chat", firstFishChat).
					Int("TwitchID", apiID).
					Str("Player", player).
					Int("PlayerID", playerID).
					Msg("Error adding player to playerdata")
				return []PossiblePlayer{}, err
			}

			newplayer := PossiblePlayer{
				FirstSeen: firstFishDate,
				PlayerID:  playerID,
				Player:    player,
			}

			newplayer.TwitchID.Int64 = 0

			possiblePlayers = append(possiblePlayers, newplayer)
		}

	} else {
		// Need to check if the players last catch was more than six months away to make sure that its still the same players
		// because twitch names can be reused six months after someone renamed
		for _, possiblePlayer := range possiblePlayers {
			var months, years int
			err := pool.QueryRow(context.Background(),
				"select date_part('month', age($1, $2)), date_part('year', age($1, $2))",
				possiblePlayer.LastSeen, firstFishDate).Scan(&months, &years)
			if err != nil {
				logs.Logs().Error().Err(err).
					Int("TwitchID", int(possiblePlayer.TwitchID.Int64)).
					Int("PlayerID", possiblePlayer.PlayerID).
					Str("Player", player).
					Msg("Error getting month difference for possible player")
				return []PossiblePlayer{}, err
			}

			// this has to be negative
			if months < -5 || years < 0 {
				// check the twitchid of the name
				apiID, err = GetTwitchID(player)
				if err != nil && !errors.Is(err, ErrNoPlayerFound) {
					logs.Logs().Error().Err(err).
						Str("Player", player).
						Msg("Error getting twitch id for player")
					return []PossiblePlayer{}, err
				}

				// the player has to be new if the twitchids are different
				if possiblePlayer.TwitchID.Int64 != int64(apiID) {

					// only add them as a new player if there is no other possible player
					if len(possiblePlayers) == 1 {
						playerID, err := AddNewPlayer(apiID, player, firstFishDate, firstFishChat, pool)
						if err != nil {
							logs.Logs().Error().Err(err).
								Str("Date", firstFishDate.Format(time.RFC3339)).
								Str("Chat", firstFishChat).
								Int("TwitchID", apiID).
								Str("Player", player).
								Msg("Error adding player to playerdata")
							return []PossiblePlayer{}, err
						}

						newplayer := PossiblePlayer{
							FirstSeen: firstFishDate,
							PlayerID:  playerID,
							Player:    player,
						}

						newplayer.TwitchID.Int64 = int64(apiID)

						// return only this new player
						var newpossibleplayer []PossiblePlayer

						newpossibleplayer = append(newpossibleplayer, newplayer)

						return newpossibleplayer, nil
					}
				} // else it has to be another of the possible players
			} // else do nothing i think ?
		}
	}

	return possiblePlayers, nil
}

// If a player used the name multiple times in the past, this will select a huge range
// there is one problem:
// like if player1 used "a" as a name and then renames and then after 6 months player2 uses "a" as a name and then after some time renames and then player1 uses "a" again ?
// but if noone else used that name between this is fine
// can see if one name appears multiple times in the []oldnames and then split that player and append them to possible players two times
// but would need to find the >=6 months gap so that last and first seen are different ?
func PlayerDates(pool *pgxpool.Pool, playerID int, player string) (time.Time, time.Time, error) {

	var lastseen, firstseen time.Time

	err := pool.QueryRow(context.Background(),
		"select max(date), min(date) from fish where playerid = $1 and player = $2",
		playerID, player).Scan(&lastseen, &firstseen)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	return lastseen, firstseen, nil
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
		logs.Logs().Warn().
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
