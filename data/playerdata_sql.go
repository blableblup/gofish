package data

import (
	"context"
	"database/sql"
	"gofish/logs"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetPlayersForPlayerName(player string, pool *pgxpool.Pool) ([]PlayerDataInDB, error) {

	var players []PlayerDataInDB

	rows, err := pool.Query(context.Background(),
		"SELECT name, playerid, twitchid, oldnames FROM playerdata WHERE name = $1",
		player)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Player", player).
			Msg("Error quering for player name in playerdata")
		return players, err
	}
	defer rows.Close()

	for rows.Next() {

		var DBData PlayerDataInDB

		if err := rows.Scan(&DBData.Name, &DBData.PlayerID, &DBData.TwitchID, &DBData.OldNames); err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error scanning row for player")
			return players, err
		}

		players = append(players, DBData)

	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error iterating over rows")
		return players, err
	}

	return players, nil
}

func GetPlayersForOldName(player string, pool *pgxpool.Pool) ([]PlayerDataInDB, error) {

	var players []PlayerDataInDB

	rows, err := pool.Query(context.Background(),
		"SELECT name, playerid, twitchid, oldnames FROM playerdata WHERE $1 = any(oldnames)",
		player)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Player", player).
			Msg("Error quering for old player names in playerdata")
		return players, err
	}
	defer rows.Close()

	for rows.Next() {

		var DBData PlayerDataInDB

		if err := rows.Scan(&DBData.Name, &DBData.PlayerID, &DBData.TwitchID, &DBData.OldNames); err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error scanning row for player")
			return players, err
		}

		players = append(players, DBData)

	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error iterating over rows")
		return players, err
	}

	return players, nil
}

func CheckForTwitchIDInDB(twitchID int, pool *pgxpool.Pool) (PlayerDataInDB, bool, error) {

	var DBData PlayerDataInDB

	err := pool.QueryRow(context.Background(),
		"SELECT name, playerid, twitchid, oldnames FROM playerdata WHERE twitchid = $1",
		twitchID).Scan(&DBData.Name, &DBData.PlayerID, &DBData.TwitchID, &DBData.OldNames)

	if err != nil {
		if err == pgx.ErrNoRows {
			return DBData, false, nil
		} else {
			return DBData, false, err
		}
	}

	return DBData, true, nil
}

func PlayerDates(pool *pgxpool.Pool, playerID int, player string) (time.Time, time.Time, error) {

	// this can be weird, if the name was used by the person multiple times
	// and someone else used that name between the other person
	// their time will overlap then
	var lastseen, firstseen sql.NullTime

	err := pool.QueryRow(context.Background(),
		`select max(max), min(min)
		from (
		select max(date), min(date) from ambience where playerid = $1 and player = $2
		union all
		select max(date), min(date) from bag where playerid = $1 and player = $2
		union all
		select max(date), min(date) from fish where playerid = $1 and player = $2
		) as all_dates
		`,
		playerID, player).Scan(&lastseen, &firstseen)
	if err != nil {
		logs.Logs().Error().Err(err).
			Int("PlayerID", playerID).
			Str("Player", player).
			Msg("Error querying DB for last and firstseen!!")
		return time.Time{}, time.Time{}, err
	}

	if !lastseen.Valid {
		logs.Logs().Error().
			Str("Player", player).
			Int("PlayerID", playerID).
			Msg("Cant find valid lastseen and firstseen for player!!!!")
		return time.Time{}, time.Time{}, nil
	}

	return lastseen.Time, firstseen.Time, nil
}

// years and months is positive if day / $1 is the higher day
func DiffBetweenTwoDates(day time.Time, day2 time.Time, pool *pgxpool.Pool) (int, int, error) {

	var months, years int

	err := pool.QueryRow(context.Background(),
		"select date_part('month', age($1, $2)), date_part('year', age($1, $2))",
		day, day2).Scan(&months, &years)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Date2", day2.Format("2006-01-2 15:04:05")).
			Str("Date1", day.Format("2006-01-2 15:04:05")).
			Msg("Error getting month difference for dates")
		return months, years, err
	}

	return months, years, nil

}

func AddNewPlayer(twitchid int, player string, firstFishDate time.Time, firstFishChat string, pool *pgxpool.Pool) (int, error) {

	// Add a new player and return their id
	// If a players twitchid cannot be found in the api, twitchid is null
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

func AppendOldName(player string, oldName string, twitchID int, playerID int, pool *pgxpool.Pool) error {

	_, err := pool.Exec(context.Background(), `
			UPDATE playerdata
			SET oldnames = array_append(oldnames, $1)
			WHERE twitchid = $2		
			`, oldName, twitchID)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Player", player).
			Str("OldName", oldName).
			Int("TwitchID", twitchID).
			Int("PlayerID", playerID).
			Msg("Error updating player data for name")
		return err
	}

	logs.Logs().Info().
		Str("Player", player).
		Str("OldName", oldName).
		Int("TwitchID", twitchID).
		Int("PlayerID", playerID).
		Msg("Added old name for player")

	return nil
}
