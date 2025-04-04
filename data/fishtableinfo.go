package data

import (
	"bufio"
	"context"
	"gofish/logs"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// This is to get the fishname for a fishtype before inserting the fish into the db
// The fishtype in the table is the emote of the fish. A fish type can have a shiny version and old versions of the emote (like 🕷🕷️ for spider)

func GetFishName(pool *pgxpool.Pool, fishinfotable string, fishType string) (string, error) {

	var exists bool

	// Check if fishType exists directly
	err := pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM "+fishinfotable+" WHERE fishtype = $1)", fishType).Scan(&exists)
	if err != nil {
		return "", err
	}
	if exists {
		return queryFishNameByType(pool, fishinfotable, fishType)
	}

	// Check if fishType exists as old emoji
	err = pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM "+fishinfotable+" WHERE $1 = ANY(oldemojis))", fishType).Scan(&exists)
	if err != nil {
		return "", err
	}
	if exists {
		return queryFishNameByEmoji(pool, fishinfotable, fishType)
	}

	// Check if fishType exists as shiny
	err = pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM "+fishinfotable+" WHERE $1 = ANY(shiny))", fishType).Scan(&exists)
	if err != nil {
		return "", err
	}
	if exists {
		return queryFishNameByShiny(pool, fishinfotable, fishType)
	}

	// If not found, add it and retrieve the fish name
	newFishType, err := addFishType(pool, fishinfotable, fishType)
	if err != nil {
		return "", err
	}
	return queryFishNameByType(pool, fishinfotable, newFishType)
}

func queryFishNameByType(pool *pgxpool.Pool, fishinfotable string, fishType string) (string, error) {
	var fishName string
	err := pool.QueryRow(context.Background(), "SELECT fishname FROM "+fishinfotable+" WHERE fishtype = $1", fishType).Scan(&fishName)
	return fishName, err
}

func queryFishNameByEmoji(pool *pgxpool.Pool, fishinfotable string, fishType string) (string, error) {
	var fishName string
	err := pool.QueryRow(context.Background(), "SELECT fishname FROM "+fishinfotable+" WHERE $1 = ANY(oldemojis)", fishType).Scan(&fishName)
	return fishName, err
}

func queryFishNameByShiny(pool *pgxpool.Pool, fishinfotable string, fishType string) (string, error) {
	var fishName string
	err := pool.QueryRow(context.Background(), "SELECT fishname FROM "+fishinfotable+" WHERE $1 = ANY(shiny)", fishType).Scan(&fishName)
	return fishName, err
}

func addFishType(pool *pgxpool.Pool, fishinfotable string, fishType string) (string, error) {
	var oldfishType, newfishType string

	logs.Logs().Info().Msgf("Unknown fish type '%s' detected. Is it a new fish type, a shiny or a different emote for an existing fish type?", fishType)
	logs.Logs().Info().Msg("Use new/shiny/emote: ")

	scanner := bufio.NewScanner(os.Stdin)

	for {
		scanner.Scan()
		err := scanner.Err()
		if err != nil {
			return fishType, err
		}
		response := scanner.Text()

		switch response {
		case "new":
			// Add the new fish into the database
			logs.Logs().Info().Msgf("Enter the fishname for the new fish '%s': ", fishType)
			scanner.Scan()
			err = scanner.Err()
			if err != nil {
				return fishType, err
			}
			fishName := scanner.Text()

			_, err := pool.Exec(context.Background(), "INSERT INTO "+fishinfotable+" (fishname, fishtype) VALUES ($1, $2)", fishName, fishType)
			if err != nil {
				return fishType, err
			}

			logs.Logs().Info().Msgf("New fish type '%s' added to the database with name '%s'", fishType, fishName)

			return fishType, nil

		case "shiny":
			// Add the shiny to the fishType and return the actual fishType
			logs.Logs().Info().Msgf("Enter the fishname for the shiny '%s': ", fishType)
			scanner.Scan()
			err = scanner.Err()
			if err != nil {
				return fishType, err
			}
			fishName := scanner.Text()

			_, err := pool.Exec(context.Background(), "UPDATE "+fishinfotable+" SET shiny = array_append(shiny, $1) WHERE fishname = $2", fishType, fishName)
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
			scanner.Scan()
			err = scanner.Err()
			if err != nil {
				return fishType, err
			}
			fishName := scanner.Text()

			logs.Logs().Info().Msg("Is the emote new or old? Type new/old")

			for {
				scanner.Scan()
				err := scanner.Err()
				if err != nil {
					return fishType, err
				}
				response := scanner.Text()

				switch response {
				case "new":
					// Update the fishtype and add the current emoji to oldemojis
					row := pool.QueryRow(context.Background(), "SELECT fishtype FROM "+fishinfotable+"  WHERE fishname = $1", fishName)
					if err := row.Scan(&oldfishType); err != nil {
						return fishType, err
					}

					_, err := pool.Exec(context.Background(), "UPDATE "+fishinfotable+" SET fishtype = $1, oldemojis = array_append(oldemojis, $2) WHERE fishname = $3", fishType, oldfishType, fishName)
					if err != nil {
						return fishType, err
					}

					logs.Logs().Info().Msgf("Updated the fishType for fish name '%s' from '%s' to '%s'", fishName, oldfishType, fishType)

					return fishType, nil

				case "old":
					// Update the oldemojis for that fishType
					_, err := pool.Exec(context.Background(), "UPDATE "+fishinfotable+" SET oldemojis = array_append(oldemojis, $1) WHERE fishname = $2", fishType, fishName)
					if err != nil {
						return fishType, err
					}

					logs.Logs().Info().Msgf("Added '%s' to oldemojis for fish name '%s'", fishType, fishName)

					row := pool.QueryRow(context.Background(), "SELECT fishtype FROM "+fishinfotable+"  WHERE fishname = $1", fishName)
					if err := row.Scan(&newfishType); err != nil {
						return fishType, err
					}

					return newfishType, nil

				default:
					logs.Logs().Warn().Msgf("-.- Invalid response '%s'. Use new/old", response)

				}
			}

		default:
			logs.Logs().Warn().Msgf(">.< Invalid response '%s'. Use new/shiny/emote", response)
		}
	}
}
