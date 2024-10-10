package scripts

import (
	"context"
	"gofish/data"
	"gofish/logs"
	"gofish/playerdata"

	"github.com/jackc/pgx/v4"
)

// Mode ble is checking the twitch ids only for players who dont have one
func UpdateTwitchIDs(mode string) {

	pool, err := data.Connect()
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error connecting to the database")
		return
	}
	defer pool.Close()

	var rows pgx.Rows

	if mode == "ble" {
		rows, err = pool.Query(context.Background(), `
		SELECT name from playerdata
		WHERE twitchid is null`)
		if err != nil {
			logs.Logs().Error().Err(err).
				Msg("Error querying database")
			return
		}
		defer rows.Close()
	} else {
		rows, err = pool.Query(context.Background(), `
		SELECT name from playerdata`)
		if err != nil {
			logs.Logs().Error().Err(err).
				Msg("Error querying database")
			return
		}
		defer rows.Close()
	}

	for rows.Next() {
		var name string

		if err := rows.Scan(&name); err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", name).
				Msg("Error scanning row for player name")
			continue
		}

		// If you cant find the player with the first website
		// Check the other websites api
		// Or check the official data from bready
		id, err := playerdata.GetTwitchID(name)
		if err != nil {
			id, err = CheckTwitchID23(name)
			if err != nil {
				continue
			}
		}

		_, err = pool.Exec(context.Background(), `
            UPDATE playerdata
            SET twitchid = $1
            WHERE name = $2`, id, name)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", name).
				Int("ID", id).
				Msg("Error updating player data for name")
			continue
		}

		logs.Logs().Info().
			Str("Player", name).
			Int("ID", id).
			Msg("Updated twitch id")
	}
}

func CheckTwitchID23(name string) (int, error) {

	id, err := playerdata.GetTwitchID2(name)
	if err != nil {
		id, err = playerdata.GetTwitchID3(name)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", name).
				Msg("Error getting twitch id for player")
			return 0, err
		}

	}
	return id, nil
}
