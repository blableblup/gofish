package data

import (
	"context"
	"database/sql"
	"gofish/logs"
	"gofish/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ReturnFishLocation(pool *pgxpool.Pool, table string, ambienceMessage string) (string, string, string, error) {

	var exists bool

	err := pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM "+table+" WHERE ambience = $1)", ambienceMessage).Scan(&exists)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Table", table).
			Str("Ambience", ambienceMessage).
			Msg("Error checking if ambience exists")
		return "", "", "", err
	}

	if exists {
		return queryLocation(pool, table, ambienceMessage)
	}

	err = pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM "+table+" WHERE $1 = any(oldemojis))", ambienceMessage).Scan(&exists)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Table", table).
			Str("Ambience", ambienceMessage).
			Msg("Error checking if ambience exists")
		return "", "", "", err
	}

	if exists {
		return queryLocationOldEmoji(pool, table, ambienceMessage)
	} else {
		return addAmbience(pool, table, ambienceMessage)
	}
}

func queryLocation(pool *pgxpool.Pool, table string, ambience string) (string, string, string, error) {
	var ambiencename, location string
	var sublocation sql.NullString
	err := pool.QueryRow(context.Background(), "SELECT ambiencename, location, sublocation FROM "+table+" WHERE ambience = $1", ambience).Scan(&ambiencename, &location, &sublocation)

	if sublocation.Valid {
		return ambiencename, location, sublocation.String, err
	} else {
		return ambiencename, location, "", err
	}
}

func queryLocationOldEmoji(pool *pgxpool.Pool, table string, ambience string) (string, string, string, error) {
	var ambiencename, location string
	var sublocation sql.NullString
	err := pool.QueryRow(context.Background(), "SELECT ambiencename, location, sublocation FROM "+table+" WHERE $1 = any(oldemojis)", ambience).Scan(&ambiencename, &location, &sublocation)

	if sublocation.Valid {
		return ambiencename, location, sublocation.String, err
	} else {
		return ambiencename, location, "", err
	}
}

func addAmbience(pool *pgxpool.Pool, table string, ambience string) (string, string, string, error) {

	logs.Logs().Info().Msgf("Unknown ambience '%s' detected", ambience)
	logs.Logs().Info().Msg("Is it new or a diff emoji for an existing ambience? (new/diff)")

	for {

		response, err := utils.ScanAndReturn()
		if err != nil {
			return "", "", "", err
		}

		switch response {
		case "new":
			var ambiencename, location, sublocation string

			logs.Logs().Info().Msg("What is the name of the ambience: ")

			response, err = utils.ScanAndReturn()
			if err != nil {
				return "", "", "", err
			}

			ambiencename = response

			logs.Logs().Info().Msg("Which location is it: ")

			response, err = utils.ScanAndReturn()
			if err != nil {
				return "", "", "", err
			}

			location = response

			ja, err := utils.Confirm("Is it a sublocation? (y / n)")
			if err != nil {
				return "", "", "", err
			}

			if ja {

				logs.Logs().Info().Msg("Which sublocation is it: ")

				response, err := utils.ScanAndReturn()
				if err != nil {
					return "", "", "", err
				}

				sublocation = response

				_, err = pool.Exec(context.Background(), "INSERT INTO "+table+" (ambiencename, ambience, location, sublocation) VALUES ($1, $2, $3, $4)",
					ambiencename, ambience, location, sublocation)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Table", table).
						Str("Ambience", ambience).
						Str("Location", response).
						Str("Sublocation", sublocation).
						Msg("Error adding location")
					return "", "", "", err
				}

				logs.Logs().Info().
					Str("Ambiencename", ambiencename).
					Str("Ambience", ambience).
					Str("Location", location).
					Str("SubLocation", sublocation).
					Msg("Added ambience")

			} else {

				_, err := pool.Exec(context.Background(), "INSERT INTO "+table+" (ambiencename, ambience, location) VALUES ($1, $2, $3)",
					ambiencename, ambience, location)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Table", table).
						Str("Ambience", ambience).
						Str("Location", location).
						Msg("Error adding location")
					return "", "", "", err
				}

				logs.Logs().Info().
					Str("Ambiencename", ambiencename).
					Str("Ambience", ambience).
					Str("Location", location).
					Msg("Added ambience")

			}

			return ambiencename, location, sublocation, nil

		case "diff":

			logs.Logs().Info().Msg("What is the name of the ambience ?")

			response, err = utils.ScanAndReturn()
			if err != nil {
				return "", "", "", err
			}

			ambiencename := response

			logs.Logs().Info().Msg("Is the ambience new or old version? (new/old)")

			for {
				response, err = utils.ScanAndReturn()
				if err != nil {
					return "", "", "", err
				}

				switch response {
				case "new":

					var oldAmbience string

					row := pool.QueryRow(context.Background(), "SELECT ambience FROM "+table+"  WHERE ambiencename = $1", ambiencename)
					if err := row.Scan(&oldAmbience); err != nil {
						return "", "", "", err
					}

					_, err := pool.Exec(context.Background(), "UPDATE "+table+" SET ambience = $1, oldemojis = array_append(oldemojis, $2) WHERE ambiencename = $3", ambience, oldAmbience, ambiencename)
					if err != nil {
						return "", "", "", err
					}

					logs.Logs().Info().
						Str("Ambiencename", ambiencename).
						Str("NewAmbience", ambience).
						Str("OldAmbience", oldAmbience).
						Msg("Updated ambience")

					return queryLocation(pool, table, ambience)

				case "old":

					_, err := pool.Exec(context.Background(), "UPDATE "+table+" SET oldemojis = array_append(oldemojis, $1) WHERE ambiencename = $2", ambience, ambiencename)
					if err != nil {
						return "", "", "", err
					}

					logs.Logs().Info().
						Str("Ambiencename", ambiencename).
						Str("OldAmbience", ambience).
						Msg("Added old ambience to ambience")

					return queryLocationOldEmoji(pool, table, ambience)

				default:
					logs.Logs().Warn().Msg("Use new / old !!")

				}
			}

		default:
			logs.Logs().Warn().Msg("Use new / diff !!!!")

		}
	}

}
