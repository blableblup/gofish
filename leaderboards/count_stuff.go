package leaderboards

import (
	"fmt"
	"gofish/logs"
	"strings"
)

func countBoardTitles(params LeaderboardParams) string {
	board := params.LeaderboardType
	chatName := params.ChatName

	var title string

	switch board {
	default:
		logs.Logs().Error().
			Str("Board", board).
			Msg("NO TITLE FOR BOARD!")
		title = "### no title >____<!\n"

	case "count":
		if chatName != "global" {
			if strings.HasSuffix(chatName, "s") {
				title = fmt.Sprintf("### Most fish caught in %s' chat\n", chatName)
			} else {
				title = fmt.Sprintf("### Most fish caught in %s's chat\n", chatName)
			}
		} else {
			title = "### Most fish caught globally\n"
		}

	case "uniquefish":
		if chatName != "global" {
			if strings.HasSuffix(chatName, "s") {
				title = fmt.Sprintf("### Players who have seen the most fish in %s' chat\n", chatName)
			} else {
				title = fmt.Sprintf("### Players who have seen the most fish in %s's chat\n", chatName)
			}
		} else {
			title = "### Players who have seen the most fish globally\n"
		}

	case "fishweek":
		if chatName != "global" {
			if strings.HasSuffix(chatName, "s") {
				title = fmt.Sprintf("### Most fish caught in a single week in tournaments in %s' chat\n", chatName)
			} else {
				title = fmt.Sprintf("### Most fish caught in a single week in tournaments in %s's chat\n", chatName)
			}
		} else {
			title = "### this cant happen because fishweek doesnt have a global board\n"
		}

	}

	return title
}

func countBoardSql(params LeaderboardParams) map[string]string {
	board := params.LeaderboardType
	chatName := params.ChatName

	query := make(map[string]string)

	switch board {
	default:
		logs.Logs().Error().
			Str("Board", board).
			Msg("NO QUERIES FOR BOARD!")

	case "count":
		if chatName != "global" {
			query["1"] = `
			SELECT playerid, COUNT(*) 
			FROM fish
			WHERE chat = $1
			AND date < $2
			AND date > $3
			GROUP BY playerid
			HAVING COUNT(*) >= $4`
		} else {
			query["1"] = `
			SELECT playerid, COUNT(*) 
			FROM fish
			WHERE date < $1
			AND date > $2
			GROUP BY playerid
			HAVING COUNT(*) >= $3`

			query["2"] = `
			select playerid, chat, count(*)
			from fish
			where playerid = any($1)
			and date < $2
			and date > $3
			group by playerid, chat
			order by count desc`
		}

	case "uniquefish":
		if chatName != "global" {
			query["1"] = `
			SELECT playerid, count
			FROM (
			SELECT playerid, COUNT(DISTINCT fishname)
			FROM fish
			WHERE chat = $1
			AND date < $2
			AND date > $3
			GROUP BY playerid
			) as subquery
			WHERE count >= $4`
		} else {
			query["1"] = `
			SELECT playerid, count
			FROM (
			SELECT playerid, COUNT(DISTINCT fishname)
			FROM fish
			WHERE date < $1
			AND date > $2
			GROUP BY playerid
			) as subquery
			WHERE count >= $3`

			query["2"] = `
			select playerid, chat, count(distinct fishname)
			from fish
			where playerid = any($1)
			and date < $2
			and date > $3
			group by playerid, chat
			order by count desc`
		}

	case "fishweek":
		if chatName != "global" {
			query["1"] = `
			SELECT t.playerid, t.fishcaught as count, t.bot
			FROM tournaments t
			JOIN (
				SELECT playerid, MAX(fishcaught) AS max_count
				FROM tournaments
				WHERE chat = $1
				AND date < $2
				AND date > $3
				GROUP BY playerid
			) max_t ON t.playerid = max_t.playerid AND t.fishcaught = max_t.max_count
			WHERE t.chat = $1 AND max_count >= $4`

		} else {
			query["1"] = `bubi`
		}

	}

	return query
}
