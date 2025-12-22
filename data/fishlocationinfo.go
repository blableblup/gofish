package data

import (
	"context"
	"gofish/logs"
	"gofish/utils"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ReturnFishLocation(pool *pgxpool.Pool, table string, ambienceMessage string) (string, error) {

	var exists bool

	err := pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM "+table+" WHERE $1 = ANY(ambience))", ambienceMessage).Scan(&exists)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Table", table).
			Str("Ambience", ambienceMessage).
			Msg("Error checking if ambience exists")
		return "", err
	}
	if exists {
		return queryLocation(pool, table, ambienceMessage)
	} else {
		return addAmbience(pool, table, ambienceMessage)
	}
}

func queryLocation(pool *pgxpool.Pool, table string, ambience string) (string, error) {
	var location string
	err := pool.QueryRow(context.Background(), "SELECT location FROM "+table+" WHERE $1 = ANY(ambience)", ambience).Scan(&location)
	return location, err
}

func addAmbience(pool *pgxpool.Pool, table string, ambience string) (string, error) {

	logs.Logs().Info().Msgf("Unknown ambience '%s' detected", ambience)
	logs.Logs().Info().Msg("Which location is it: ")

	response, err := utils.ScanAndReturn()
	if err != nil {
		return "", err
	}

	var exists bool

	err = pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM "+table+" WHERE $1 =location)", response).Scan(&exists)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Table", table).
			Str("Location", response).
			Msg("Error checking if location exists")
		return "", err
	}

	if exists {
		_, err = pool.Exec(context.Background(), "UPDATE "+table+" SET ambience = array_append(ambience, $1) WHERE location = $2", ambience, response)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Table", table).
				Str("Ambience", ambience).
				Msg("Error updating ambience table")
			return response, err
		}
	} else {
		// sep is ü, because: "If s does not contain sep and sep is not empty, Split returns a slice of length 1 whose only element is s."
		_, err := pool.Exec(context.Background(), "INSERT INTO "+table+" (location, ambience) VALUES ($1, $2)", response, strings.Split(ambience, "ü"))
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Table", table).
				Str("Ambience", ambience).
				Str("Location", response).
				Msg("Error adding location")
			return response, err
		}

		logs.Logs().Info().
			Str("Location", response).
			Msg("Added new location")
	}

	return response, nil
}
