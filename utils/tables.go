package utils

import (
	"context"
	"fmt"
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
		fmt.Printf("Table '%s' does not exist, creating...\n", tableName)
	}

	// Create the appropriate table if it doesn't exist
	if !exists {
		switch {
		case strings.HasPrefix(tableName, "fish"):
			_, err := pool.Exec(context.Background(), fmt.Sprintf(`
				CREATE TABLE %s (
					fishid SERIAL PRIMARY KEY,
					chatid INT,
					type VARCHAR(255),
					typename VARCHAR(255),
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

			fmt.Printf("Table '%s' created successfully\n", tableName)
		case tableName == "typename":
			_, err := pool.Exec(context.Background(), fmt.Sprintf(`
				CREATE TABLE %s (
					type VARCHAR(255) PRIMARY KEY,
					typename VARCHAR(255)
				)
			`, tableName))
			if err != nil {
				return err
			}

			fmt.Printf("Table '%s' created successfully\n", tableName)
		case tableName == "playerdata":
			_, err := pool.Exec(context.Background(), fmt.Sprintf(`
				CREATE TABLE %s (
					playerid SERIAL PRIMARY KEY,
					twitchid INT,
					name VARCHAR(255),
					oldnames TEXT,
					verified BOOLEAN,
					cheated BOOLEAN,
					firstfishdate TIMESTAMP,
					firstfishchat VARCHAR(255)
				)
			`, tableName))
			if err != nil {
				return err
			}

			fmt.Printf("Table '%s' created successfully\n", tableName)
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

			fmt.Printf("Table '%s' created successfully\n", tableName)

		default:
			return fmt.Errorf("unsupported table name: %s", tableName)
		}
	}

	return nil
}
