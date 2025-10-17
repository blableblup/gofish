package scripts

import (
	"context"
	"gofish/logs"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PlayerData struct {
	Name           string
	LatestName     string
	LatestNameDate time.Time
}

// so... the playerdata function has only been renaming players
// if their name was completely new and not used by someone else before
// this has been like that since i think december 2024
// oopert
// this func is to find players where the latest name from bag/fish
// doesnt match their playerdata entry
func CheckWhoDidntGetRenamedOops(pool *pgxpool.Pool) error {

	data := make(map[int]PlayerData)

	rows, err := pool.Query(context.Background(), "select name, playerid from playerdata")
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error quering for playerdata")
		return err
	}
	defer rows.Close()

	for rows.Next() {

		var playerID int
		var name string

		if err := rows.Scan(&name, &playerID); err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", name).
				Msg("Error scanning row for player")
			return err
		}

		data[playerID] = PlayerData{Name: name}
	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error iterating over rows")
		return err
	}

	// latest date + name from fish
	rows, err = pool.Query(context.Background(),
		`select date, f.playerid, player
		from fish f
		join
		(
		select max(date) as max_date, playerid
		from fish
		group by playerid
		) max on f.playerid = max.playerid and f.date = max.max_date`)
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error quering for latest names in fish")
		return err
	}
	defer rows.Close()

	for rows.Next() {

		var frick PlayerData
		var playerID int

		if err := rows.Scan(&frick.LatestNameDate, &playerID, &frick.LatestName); err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", frick.LatestName).
				Msg("Error scanning row for player")
			return err
		}

		existData, exists := data[playerID]
		if exists {
			existData.LatestName = frick.LatestName
			existData.LatestNameDate = frick.LatestNameDate

			data[playerID] = existData
		}

	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error iterating over rows")
		return err
	}

	// latest date + name from bag
	rows, err = pool.Query(context.Background(),
		`select date, b.playerid, player
		from bag b
		join
		(
		select max(date) as max_date, playerid
		from bag
		group by playerid
		) max on b.playerid = max.playerid and b.date = max.max_date`)
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error quering for latest names in bag")
		return err
	}
	defer rows.Close()

	for rows.Next() {

		var frick PlayerData
		var playerID int

		if err := rows.Scan(&frick.LatestNameDate, &playerID, &frick.LatestName); err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", frick.LatestName).
				Msg("Error scanning row for player")
			return err
		}

		existData, exists := data[playerID]
		if exists {
			if existData.LatestNameDate.Before(frick.LatestNameDate) {
				existData.LatestName = frick.LatestName
				existData.LatestNameDate = frick.LatestNameDate

				data[playerID] = existData
			}
		}

	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error iterating over rows")
		return err
	}

	for playerid, playerdata := range data {

		if playerdata.Name != playerdata.LatestName {
			logs.Logs().Warn().
				Str("Name in playerdata", playerdata.Name).
				Str("Latest name", playerdata.LatestName).
				Str("Latest date", playerdata.LatestNameDate.Format("2006-01-02 15:04:05 UTC")).
				Int("PlayerID", playerid).
				Msg("NAME DOESNT MATCH")
		}

	}

	logs.Logs().Info().Msg("done")

	return nil
}
