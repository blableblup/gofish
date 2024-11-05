package utils

import (
	"context"
	"fmt"
	"gofish/logs"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
)

func EnsureTableExists(pool *pgxpool.Pool, tableName string) error {
	// Check if the table exists by querying the information_schema.tables
	var exists bool
	err := pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE lower(table_name) = lower($1))", tableName).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		logs.Logs().Info().Msgf("Table '%s' does not exist, creating...", tableName)
	}

	// Create the appropriate table if it doesn't exist
	if !exists {
		switch {
		case tableName == "fish":
			_, err := pool.Exec(context.Background(), fmt.Sprintf(`
				CREATE TABLE %s (
					fishid SERIAL PRIMARY KEY,
					chatid INT,
					fishtype VARCHAR(255),
					fishname VARCHAR(255),
					weight FLOAT,
					catchtype VARCHAR(255),
					player VARCHAR(255),
					playerid INT,
					date TIMESTAMP,
					bot VARCHAR(255),
					chat VARCHAR(255)
				)
			`, tableName))
			if err != nil {
				return err
			}

			logs.Logs().Info().Msgf("Table '%s' created successfully", tableName)
		case tableName == "fishinfo":
			_, err := pool.Exec(context.Background(), fmt.Sprintf(`
				CREATE TABLE %s (
					fishname VARCHAR(255) PRIMARY KEY,
					fishtype VARCHAR(255),
					oldemojis TEXT,
					shiny TEXT
				)
			`, tableName))
			if err != nil {
				return err
			}

			logs.Logs().Info().Msgf("Table '%s' created successfully", tableName)
		case tableName == "playerdata":
			_, err := pool.Exec(context.Background(), fmt.Sprintf(`
				CREATE TABLE %s (
					playerid SERIAL PRIMARY KEY,
					twitchid INT,
					name VARCHAR(255),
					oldnames TEXT,
					verified BOOLEAN,
					firstfishdate TIMESTAMP,
					firstfishchat VARCHAR(255)
				)
			`, tableName))
			if err != nil {
				return err
			}

			logs.Logs().Info().Msgf("Table '%s' created successfully", tableName)
		case strings.HasPrefix(tableName, "tournaments"):
			_, err := pool.Exec(context.Background(), fmt.Sprintf(`
				CREATE TABLE %s (
					trnmid SERIAL PRIMARY KEY,
					player VARCHAR(255),
					playerid INT,
					fishcaught INT,
					placement1 INT,
					totalweight FLOAT,
					placement2 INT,
					biggestfish FLOAT,
					placement3 INT,
					date TIMESTAMP,
					bot VARCHAR(255),
					chat VARCHAR(255)
				)
			`, tableName))
			if err != nil {
				return err
			}

			logs.Logs().Info().Msgf("Table '%s' created successfully", tableName)

		default:
			return fmt.Errorf("unsupported table name: %s", tableName)
		}
	}

	return nil
}
