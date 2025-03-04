package scripts

import (
	"context"
	"fmt"
	"gofish/logs"
	"gofish/utils"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// This script was needed because I got the twitchids without merging the players right away
// So there were multiple entries with the same twitchid in playerdata
// This doesnt update the verified status of the player because some renamed
// And this doesnt consider cases where a name has been used by different accounts multiple times in the past
// Also: If a player who was merged had oldnames in the table, the oldnames are lost, if the oldnames were part of an "oldname" and not the current name
// But this didnt happen (I think)

// To check ids which are in a table but not in playerdata
// SELECT playerid from fish
// except
// select playerid from playerdata
// order by playerid asc

// After doing this there were two players which were in fish but not in playerdata: ids 1590 and 2115 ?
// Maybe they were deleted at some point by accident ? or idk
// 2115 has a twitchid (143555482) and renamed, 1590 aswell (901340198). add them to playerdata manually and then merge
// And mikel1g and restartmikel have to be merged aswell (current name as of writing this was mikelpikol) id:211629518
func MergePlayers(pool *pgxpool.Pool) {

	config := utils.LoadConfig()

	// Get all the twitchids which appear multiple times in playerdata
	rows, err := pool.Query(context.Background(), `
	SELECT twitchid FROM playerdata
	where twitchid != '0'
	group by twitchid 
	having count(*) >1`)
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error querying database for twitchids")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var twitchid, playerid int
		var oldestdate, newestdate time.Time

		if err := rows.Scan(&twitchid); err != nil {
			logs.Logs().Error().Err(err).
				Int("TwitchID", twitchid).
				Msg("Error scanning row for twitch id")
			return
		}

		// Start a transaction
		tx, err := pool.Begin(context.Background())
		if err != nil {
			logs.Logs().Error().Err(err).
				Msg("Error starting transaction")
			return
		}
		defer tx.Rollback(context.Background())

		// Get the oldest entry for the twitchid
		// This is actually selecting mutliple rows, but since we order by date
		// the first row is the one with the lowest date
		err = tx.QueryRow(context.Background(), `
            select firstfishdate, playerid
			from playerdata
			where twitchid = $1
			order by firstfishdate asc`, twitchid).Scan(&oldestdate, &playerid)
		if err != nil {
			logs.Logs().Error().Err(err).
				Int("TwitchID", twitchid).
				Msg("Error getting oldest player entry for id")
			return
		}

		// Get the newest entry for the twitchid
		err = tx.QueryRow(context.Background(), `
		select firstfishdate
		from playerdata
		where twitchid = $1
		order by firstfishdate desc`, twitchid).Scan(&newestdate)
		if err != nil {
			logs.Logs().Error().Err(err).
				Int("TwitchID", twitchid).
				Msg("Error getting oldest player entry for id")
			return
		}

		// Get the oldnames and the current name
		oldnames, currentname, err := getoldnames(twitchid, newestdate, pool)
		if err != nil {
			logs.Logs().Error().Err(err).
				Int("TwitchID", twitchid).
				Msg("Error getting old names and current name for id")
			return
		}

		// Update the players oldnames and the playerids in fish and the tournament tables
		for _, name := range oldnames {
			_, err = tx.Exec(context.Background(), `
			update playerdata
			SET oldnames = CONCAT(oldnames, ' ', CAST($1 AS TEXT))
			WHERE firstfishdate = $2
			AND playerid = $3`, name, oldestdate, playerid)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("OldName", name).
					Str("CurrentName", currentname).
					Int("TwitchID", twitchid).
					Msg("Error updating old names for player")
				return
			}

			_, err = tx.Exec(context.Background(), `
			update fish
			SET playerid = $1
			WHERE player = $2`, playerid, name)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("OldName", name).
					Str("CurrentName", currentname).
					Int("TwitchID", twitchid).
					Msg("Error updating fish ids for old player")
				return
			}

			_, err = tx.Exec(context.Background(), `
			update bag
			SET playerid = $1
			WHERE player = $2`, playerid, name)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("OldName", name).
					Str("CurrentName", currentname).
					Int("TwitchID", twitchid).
					Msg("Error updating bag ids for old player")
				return
			}

			for chatName, chat := range config.Chat {
				if !chat.CheckFData {
					// No need to always log this
					continue
				}

				tableName := "tournaments" + chatName

				var exists bool
				err := pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE lower(table_name) = lower($1))", tableName).Scan(&exists)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Table", tableName).
						Msg("Error checking if the tournament table exists")
					return
				}

				if !exists {
					// No need to always log this
					continue
				}

				_, err = tx.Exec(context.Background(), fmt.Sprintf(`
						UPDATE %s
						SET playerid = $1
						WHERE player = $2
					`, tableName), playerid, name)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Table", tableName).
						Str("Player", name).
						Int("PlayerID", playerid).
						Msg("Error updating playerid in table")
				}
			}
		}

		// Also update the ids for the current name
		_, err = tx.Exec(context.Background(), `
		update fish
		SET playerid = $1
		WHERE player = $2`, playerid, currentname)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("CurrentName", currentname).
				Int("TwitchID", twitchid).
				Msg("Error updating fish ids for current player")
			return
		}

		_, err = tx.Exec(context.Background(), `
		update bag
		SET playerid = $1
		WHERE player = $2`, playerid, currentname)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("CurrentName", currentname).
				Int("TwitchID", twitchid).
				Msg("Error updating bag ids for current player")
			return
		}

		for chatName, chat := range config.Chat {
			if !chat.CheckFData {
				// No need to always log this
				continue
			}

			tableName := "tournaments" + chatName

			var exists bool
			err := pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE lower(table_name) = lower($1))", tableName).Scan(&exists)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Table", tableName).
					Msg("Error checking if the tournament table exists")
				return
			}

			if !exists {
				// No need to always log this
				continue
			}

			_, err = tx.Exec(context.Background(), fmt.Sprintf(`
					UPDATE %s
					SET playerid = $1
					WHERE player = $2
				`, tableName), playerid, currentname)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Table", tableName).
					Str("Player", currentname).
					Int("PlayerID", playerid).
					Msg("Error updating playerid in table")
			}
		}

		// Update the players current name
		_, err = tx.Exec(context.Background(), `
			update playerdata
			SET name = $1
			WHERE firstfishdate = $2
			AND playerid = $3`, currentname, oldestdate, playerid)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("CurrentName", currentname).
				Int("TwitchID", twitchid).
				Msg("Error updating name for player")
			return
		}

		// Delete all the other entries for the twitchid
		_, err = tx.Exec(context.Background(), `
		delete from playerdata
		where twitchid = $1
		and firstfishdate != $2`, twitchid, oldestdate)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", currentname).
				Int("TwitchID", twitchid).
				Msg("Error deleting other entries for twitchid")
			return
		}

		// Commit the transaction
		err = tx.Commit(context.Background())
		if err != nil {
			logs.Logs().Error().Err(err).
				Msg("Error committing transaction")
			return
		}

		logs.Logs().Info().
			Str("CurrentName", currentname).
			Strs("OldNames", oldnames).
			Int("TwitchID", twitchid).
			Int("PlayerID", playerid).
			Str("FirstFishCaught", oldestdate.Format(time.RFC3339)).
			Msg("Merged players entries in playerdata")

	}
}

// This doesnt get the oldnames from the "oldnames" column but instead all the names with the twitchid
func getoldnames(twitchid int, newestdate time.Time, pool *pgxpool.Pool) ([]string, string, error) {

	rows, err := pool.Query(context.Background(), `
	SELECT name, firstfishdate
	from playerdata
	where twitchid = $1`, twitchid)
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error querying database for twitchids")
		return nil, "", err
	}
	defer rows.Close()

	var oldnames []string
	var currentname string

	for rows.Next() {

		var name string
		var date time.Time

		if err := rows.Scan(&name, &date); err != nil {
			logs.Logs().Error().Err(err).
				Int("ID", twitchid).
				Msg("Error scanning row for name and firstfishdate")
			return nil, "", err
		}

		// The name with the newest name is their current twitch username
		if date != newestdate {
			oldnames = append(oldnames, name)
		} else {
			currentname = name
		}
	}

	return oldnames, currentname, nil
}
