package scripts

import (
	"context"
	"database/sql"
	"fmt"
	"gofish/logs"
	"gofish/utils"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Results struct {
	Player      string
	PlayerID    int
	FishCaught  int
	Placement1  int
	TotalWeight float64
	Placement2  int
	BiggestFish float64
	Placement3  int

	Date time.Time
	Bot  string
	Chat string
	Url  sql.NullString
}

// because i had one table for each chat
// this was only used once
func MergeTTables(pool *pgxpool.Pool) error {

	config := utils.LoadConfig()

	var allResults []Results

	tableName := "tournaments"

	for chatName := range config.Chat {

		selectAllResults := fmt.Sprintf(`
		select player, playerid, fishcaught, placement1, totalweight, placement2, biggestfish, placement3,
		date, bot, chat, url
		from tournaments%s`, chatName)

		rows, err := pool.Query(context.Background(), selectAllResults)
		if err != nil {

			if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42P01" {
				logs.Logs().Warn().
					Str("Chat", chatName).
					Msg("Chat has no tournament results")
				continue
			} else {
				logs.Logs().Error().Err(err).
					Str("Chat", chatName).
					Msg("Error quering for results")
				return err
			}

		}
		defer rows.Close()

		for rows.Next() {

			var result Results

			if err := rows.Scan(&result.Player, &result.PlayerID,
				&result.FishCaught, &result.Placement1, &result.TotalWeight, &result.Placement2, &result.BiggestFish, &result.Placement3,
				&result.Date, &result.Bot, &result.Chat, &result.Url); err != nil {
				logs.Logs().Error().Err(err).
					Str("Chat", chatName).
					Msg("Error scanning row for result")
				return err
			}

			allResults = append(allResults, result)

		}

		if err = rows.Err(); err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Msg("Error iterating over rows")
			return err
		}

	}

	sort.SliceStable(allResults, func(i, j int) bool {
		return allResults[i].Date.Before(allResults[j].Date)
	})

	if err := utils.EnsureTableExists(pool, tableName); err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error ensuring tournament table exists")
		return err
	}

	tx, err := pool.Begin(context.Background())
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error starting transaction")
		return err
	}
	defer tx.Rollback(context.Background())

	for _, result := range allResults {

		query := fmt.Sprintf("INSERT INTO %s ( player, playerid, fishcaught, placement1, totalweight, placement2, biggestfish, placement3, date, bot, chat, url) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)", tableName)
		_, err = tx.Exec(context.Background(), query, result.Player, result.PlayerID, result.FishCaught, result.Placement1, result.TotalWeight,
			result.Placement2, result.BiggestFish, result.Placement3, result.Date, result.Bot, result.Chat, result.Url)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Query", query).
				Msg("Error inserting tournament data")
			return err
		}
	}

	if err := tx.Commit(context.Background()); err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error committing transaction")
		return err
	}

	logs.Logs().Info().
		Int("Number of results", len(allResults)).
		Msg("Done merging tables")

	return nil
}
