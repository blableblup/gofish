package data

import (
	"context"
	"fmt"
	"gofish/logs"

	"github.com/jackc/pgx/v4/pgxpool"
)

var fishName, newfishType, oldfishType, response string

// I didnt really test this for new fish types, but should work ? i think

func GetFishName(pool *pgxpool.Pool, fishinfotable string, fishType string) (string, error) {

	var exists bool
	err := pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM "+fishinfotable+" WHERE fishtype = $1)", fishType).Scan(&exists)
	if err != nil {
		return fishName, err
	}

	if !exists {
		err = pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM "+fishinfotable+" WHERE $1 = ANY(STRING_TO_ARRAY(oldemojis, ' ')))", fishType).Scan(&exists)
		if err != nil {
			return fishName, err
		}

		if !exists {
			err = pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM "+fishinfotable+" WHERE $1 = ANY(STRING_TO_ARRAY(shiny, ' ')))", fishType).Scan(&exists)
			if err != nil {
				return fishName, err
			}

			if !exists {
				// The fishtype doesn't exist as oldemoji/shiny/fishtype in the database, add it
				newfishType, err = addFishType(pool, fishinfotable, fishType)
				if err != nil {
					return fishName, err
				}

				// Query the fish name again
				row := pool.QueryRow(context.Background(), "SELECT fishname FROM "+fishinfotable+" WHERE fishtype = $1", newfishType)
				if err := row.Scan(&fishName); err != nil {
					return fishName, err
				}

				return fishName, nil
			}

		} else {
			row := pool.QueryRow(context.Background(), "SELECT fishname FROM "+fishinfotable+" WHERE $1 = ANY(STRING_TO_ARRAY(oldemojis, ' '))", fishType)
			if err := row.Scan(&fishName); err != nil {
				return fishName, err
			}
			return fishName, nil
		}

	} else {
		row := pool.QueryRow(context.Background(), "SELECT fishname FROM "+fishinfotable+" WHERE fishtype = $1", fishType)
		if err := row.Scan(&fishName); err != nil {
			return fishName, err
		}
		return fishName, nil
	}

	return fishName, nil
}

func addFishType(pool *pgxpool.Pool, fishinfotable string, fishType string) (string, error) {

	logs.Logs().Info().Msgf("Unknown fish type '%s' detected. Is it a new fish type, a shiny or a different emote for an existing fish type?", fishType)
	logs.Logs().Info().Msg("Use new/shiny/emote: ")

	for {
		fmt.Scanln(&response)
		switch response {
		case "new":
			// Add the new fish into the database
			logs.Logs().Info().Msgf("Enter the fishname for the new fish '%s': ", fishType)
			fmt.Scanln(&fishName)
			_, err := pool.Exec(context.Background(), "INSERT INTO "+fishinfotable+" (fishname, fishtype) VALUES ($1, $2)", fishName, fishType)
			if err != nil {
				return fishType, err
			}

			logs.Logs().Info().Msgf("New fish type '%s' added to the database with name '%s'", fishType, fishName)

			return fishType, nil

		case "shiny":
			// Add the shiny to the fishType and return the actual fishType
			logs.Logs().Info().Msgf("Enter the fishname for the shiny '%s': ", fishType)
			fmt.Scanln(&fishName)

			_, err := pool.Exec(context.Background(), "UPDATE "+fishinfotable+" SET shiny = CONCAT(shiny, ' ', CAST($1 AS TEXT)) WHERE fishname = $2", fishType, fishName)
			if err != nil {
				return fishType, err
			}

			logs.Logs().Info().Msgf("Added '%s' as a shiny for fish '%s'", fishType, fishName)

			row := pool.QueryRow(context.Background(), "SELECT fishtype FROM "+fishinfotable+"  WHERE fishname = $1", fishName)
			if err := row.Scan(&fishType); err != nil {
				return fishType, err
			}

			return fishType, nil

		case "emote":
			// Check if the emote is new/old for an existing fish
			logs.Logs().Info().Msgf("Enter the fishname for the emote '%s': ", fishType)
			fmt.Scanln(&fishName)

			logs.Logs().Info().Msg("Is the emote new or old? Type new/old")

			for {
				fmt.Scanln(&response)
				switch response {
				case "new":
					// Update the fishtype and add the current emoji to oldemojis
					row := pool.QueryRow(context.Background(), "SELECT fishtype FROM "+fishinfotable+"  WHERE fishname = $1", fishName)
					if err := row.Scan(&oldfishType); err != nil {
						return fishType, err
					}

					_, err := pool.Exec(context.Background(), "UPDATE "+fishinfotable+" SET fishtype = $1, oldemojis = CONCAT(oldemojis, ' ', CAST($2 AS TEXT)) WHERE fishname = $3", fishType, oldfishType, fishName)
					if err != nil {
						return fishType, err
					}

					logs.Logs().Info().Msgf("Updated the fishType for fish name '%s' from '%s' to '%s'", fishName, oldfishType, fishType)

					return fishType, nil

				case "old":
					// Update the oldemojis for that fishType
					_, err := pool.Exec(context.Background(), "UPDATE "+fishinfotable+" SET oldemojis = CONCAT(oldemojis, ' ', CAST($1 AS TEXT)) WHERE fishname = $2", fishType, fishName)
					if err != nil {
						return fishType, err
					}

					logs.Logs().Info().Msgf("Added '%s' to oldemojis for fish name '%s'", fishType, fishName)

					return fishType, nil
				default:
					logs.Logs().Warn().Msgf("-.- Invalid response '%s'. Use new/old", response)

				}
			}

		default:
			logs.Logs().Warn().Msgf(">.< Invalid response '%s'. Use new/shiny/emote", response)
		}
	}
}
