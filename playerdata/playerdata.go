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

// This is finding all the players who used that players name before in the db
// and returning a slice of them
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
		possiblePlayer.Player = player

		if err := rows.Scan(&possiblePlayer.PlayerID, &possiblePlayer.TwitchID, &possiblePlayer.OldNames); err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error scanning row for player")
			return []PossiblePlayer{}, err
		}

		possiblePlayer.LastSeen, possiblePlayer.FirstSeen, err = PlayerDates(pool, possiblePlayer.PlayerID, possiblePlayer.Player)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error getting last and first seen for player")
			return []PossiblePlayer{}, err
		}

		possiblePlayers = append(possiblePlayers, possiblePlayer)
	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error iterating over rows")
		return []PossiblePlayer{}, err
	}

	// Query for all players which had that name as an oldname
	rows, err = pool.Query(context.Background(), "SELECT name, playerid, twitchid, oldnames FROM playerdata WHERE $1 = ANY(oldnames)", player)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Player", player).
			Msg("Error quering for oldnames")
		return []PossiblePlayer{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var possiblePlayer PossiblePlayer

		if err := rows.Scan(&possiblePlayer.Player, &possiblePlayer.PlayerID, &possiblePlayer.TwitchID, &possiblePlayer.OldNames); err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error scanning row for player")
			return []PossiblePlayer{}, err
		}

		// check if that player used the name multiple times
		howmanytimesusedname := make(map[string]int)
		for _, oldname := range possiblePlayer.OldNames {
			howmanytimesusedname[oldname]++
		}

		for oldname, count := range howmanytimesusedname {
			if count > 1 && oldname == player {
				// so this warns if the current name "player" which is being checked was used by a player multiple times
				// so they were renamed multiple times in the db and so the name should appear multiple times in the oldnames slice
				logs.Logs().Warn().
					Str("Player", possiblePlayer.Player).
					Str("OldName", player).
					Int("Count", count).
					Msg("Player used name multiple times in the past")
			}
		} // now wot ?

		// need to get the first and last seen of the old name (so "player"), since this is checking for old names
		possiblePlayer.LastSeen, possiblePlayer.FirstSeen, err = PlayerDates(pool, possiblePlayer.PlayerID, player)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error getting last and first seen for player")
			return []PossiblePlayer{}, err
		}

		possiblePlayers = append(possiblePlayers, possiblePlayer)
	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error iterating over rows")
		return []PossiblePlayer{}, err
	}

	var apiID int
	// If no possible players are found, check the api
	// i dont want to check the api for every player because that is really slow >_>_>>__>_> ?
	// wouldnt really change much, i would still need to do all those checks below
	// If the player is new or renamed i dont need to get their last and firstseen, since it is clear who the player is
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
		// If all the possible players last catch was more than 6 months away and the twitchids are different from the current player
		// add the player as a new entry (the player took a name which was used by other players before)
		// if the player isnt new, return all the possible players and go over them in data
		playerisnew := true
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

			// if months is > 6 or years < 6 then what ?
			if months < -6 || years < 0 {
				// check the twitchid of the name
				apiID, err = GetTwitchID(player)
				if err != nil && !errors.Is(err, ErrNoPlayerFound) {
					logs.Logs().Error().Err(err).
						Str("Player", player).
						Msg("Error getting twitch id for player")
					return []PossiblePlayer{}, err
				}

				if possiblePlayer.TwitchID.Int64 == int64(apiID) {

					playerisnew = false
					break
				}
			} else {
				playerisnew = false
				break
			}
		}
		if playerisnew {
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
	}

	return possiblePlayers, nil
}

// Get the first and last seen for the player from bag and fish
// for tournaments: would need to check all the different tournament tables since you can checkin in different chats
// probably doesnt matter ?

// there is one problem:
// like if player1 used "a" as a name and then renames and then after 6 months player2 uses "a" as a name and then after some time renames and then player1 uses "a" again ?
// but if noone else used that name between this is fine, but in this case, the check in data would be true for multiple players
// can see if one name appears multiple times in the []oldnames and then split that player and append them to possible players two times
// but would need to find the >=6 months gap so that last and first seen are different ?
// but also this is only a problem when checking older logs
func PlayerDates(pool *pgxpool.Pool, playerID int, player string) (time.Time, time.Time, error) {

	var lastseen, lastseenbag, firstseen, firstseenbag sql.NullTime
	// nulltime because not everyone has to be in both tables
	// there are fishers who only ever did +bag or never did +bag and only caught fish

	err := pool.QueryRow(context.Background(),
		"select max(date), min(date) from fish where playerid = $1 and player = $2",
		playerID, player).Scan(&lastseen, &firstseen)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	err = pool.QueryRow(context.Background(),
		"select max(date), min(date) from bag where playerid = $1 and player = $2",
		playerID, player).Scan(&lastseenbag, &firstseenbag)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	// update the lastseen and firstseen, if the times from bag are higher/lower and both arent null
	// if one of them isnt valid, return the other set of times
	if lastseenbag.Valid && lastseen.Valid {
		if lastseenbag.Time.After(lastseen.Time) {
			lastseen.Time = lastseenbag.Time
		}
		if firstseenbag.Time.Before(firstseen.Time) {
			firstseen.Time = firstseenbag.Time
		}
		return lastseen.Time, firstseen.Time, nil

	} else if !lastseenbag.Valid && lastseen.Valid {

		return lastseen.Time, firstseen.Time, nil
	} else if lastseenbag.Valid && !lastseen.Valid {

		return lastseenbag.Time, firstseenbag.Time, nil
	} else if !lastseen.Valid && !lastseenbag.Valid {
		// This only happened when i was updating and changing the db
		// if this is the case, the playerid in the db of the data which is being added will probably be 0 ...
		// if there are multiple possible players
		// can just check for that and update manually

		logs.Logs().Warn().
			Str("Player", player).
			Int("PlayerID", playerID).
			Msg("Cannot find last and firstseen for player!")
	}

	return lastseen.Time, firstseen.Time, nil
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
			SET name = $1, oldnames = array_append(oldnames, $2)
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
