package leaderboards

import (
	"fmt"
	"gofish/logs"
	"strings"
)

func typeBoardTitles(params LeaderboardParams) string {
	board := params.LeaderboardType
	chatName := params.ChatName

	var title string

	switch board {
	default:
		logs.Logs().Error().
			Str("Board", board).
			Msg("NO TITLE FOR BOARD!")
		title = "### no title >____<!\n"

	case "type":
		if chatName != "global" {
			if strings.HasSuffix(chatName, "s") {
				title = fmt.Sprintf("### Biggest fish per type caught in %s' chat\n", chatName)
			} else {
				title = fmt.Sprintf("### Biggest fish per type caught in %s's chat\n", chatName)
			}
		} else {
			title = "### Biggest fish per type caught globally\n"
		}

	case "typesmall":
		if chatName != "global" {
			if strings.HasSuffix(chatName, "s") {
				title = fmt.Sprintf("### Smallest fish per type caught in %s' chat\n", chatName)
			} else {
				title = fmt.Sprintf("### Smallest fish per type caught in %s's chat\n", chatName)
			}
		} else {
			title = "### Smallest fish per type caught globally\n"
		}

	case "typefirst":
		if chatName != "global" {
			if strings.HasSuffix(chatName, "s") {
				title = fmt.Sprintf("### First fish per type caught in %s' chat\n", chatName)
			} else {
				title = fmt.Sprintf("### First fish per type caught in %s's chat\n", chatName)
			}
		} else {
			title = "### First fish per type caught globally\n"
		}

	case "typelast":
		if chatName != "global" {
			if strings.HasSuffix(chatName, "s") {
				title = fmt.Sprintf("### Last fish per type caught in %s' chat\n", chatName)
			} else {
				title = fmt.Sprintf("### Last fish per type caught in %s's chat\n", chatName)
			}
		} else {
			title = "### Last fish per type caught globally\n"
		}
	}

	return title
}

func typeBoardSql(params LeaderboardParams) string {
	board := params.LeaderboardType
	chatName := params.ChatName

	var query string

	switch board {
	default:
		logs.Logs().Error().
			Str("Board", board).
			Msg("NO QUERY FOR BOARD!")
		query = ">______________________<"

	// for type and typesmall, ingore the catchtypes i dont see the weight of in the catch
	case "type":
		if chatName != "global" {
			query = `
			SELECT f.weight, f.fishname, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid,
			RANK() OVER (ORDER BY f.weight DESC)
			FROM fish f
			JOIN (
				SELECT fishname, MAX(weight) AS max_weight
				FROM fish 
				WHERE chat = $1
				AND date < $2
				AND date > $3
				AND catchtype != 'release'
				AND catchtype != 'squirrel'
				AND catchtype != 'sonnythrow'
				GROUP BY fishname
			) AS sub
			ON f.fishname = sub.fishname AND f.weight = sub.max_weight
			WHERE f.chat = $1
			AND f.date = (
				SELECT MIN(date)
				FROM fish
				WHERE fishname = sub.fishname AND weight = sub.max_weight AND chat = $1 AND catchtype != 'release' AND catchtype != 'squirrel' AND catchtype != 'sonnythrow'
			)`
		} else {
			query = `
			SELECT f.weight, f.fishname, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid,
			RANK() OVER (ORDER BY f.weight DESC)
			FROM fish f
			JOIN (
				SELECT fishname, MAX(weight) AS max_weight
				FROM fish 
				WHERE date < $1
				AND date > $2
				AND catchtype != 'release'
				AND catchtype != 'squirrel'
				AND catchtype != 'sonnythrow'
				GROUP BY fishname
			) AS sub
			ON f.fishname = sub.fishname AND f.weight = sub.max_weight
			AND f.date = (
				SELECT MIN(date)
				FROM fish
				WHERE fishname = sub.fishname AND weight = sub.max_weight AND catchtype != 'release' AND catchtype != 'squirrel' AND catchtype != 'sonnythrow'
			)`
		}

	case "typesmall":
		if chatName != "global" {
			query = `
			SELECT f.weight, f.fishname, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid,
			RANK() OVER (ORDER BY f.weight DESC)
			FROM fish f
			JOIN (
				SELECT fishname, MIN(weight) AS min_weight
				FROM fish 
				WHERE chat = $1
				AND date < $2
				AND date > $3
				AND catchtype != 'release'
				AND catchtype != 'squirrel'
				AND catchtype != 'sonnythrow'
				GROUP BY fishname
			) AS sub
			ON f.fishname = sub.fishname AND f.weight = sub.min_weight
			WHERE f.chat = $1
			AND f.date = (
				SELECT MIN(date)
				FROM fish
				WHERE fishname = sub.fishname AND weight = sub.min_weight AND chat = $1 AND catchtype != 'release' AND catchtype != 'squirrel' AND catchtype != 'sonnythrow'
			)`
		} else {
			query = `
			SELECT f.weight, f.fishname, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid,
			RANK() OVER (ORDER BY f.weight DESC)
			FROM fish f
			JOIN (
				SELECT fishname, MIN(weight) AS min_weight
				FROM fish 
				WHERE date < $1
				AND date > $2
				AND catchtype != 'release'
				AND catchtype != 'squirrel'
				AND catchtype != 'sonnythrow'
				GROUP BY fishname
			) AS sub
			ON f.fishname = sub.fishname AND f.weight = sub.min_weight
			AND f.date = (
				SELECT MIN(date)
				FROM fish
				WHERE fishname = sub.fishname AND weight = sub.min_weight AND catchtype != 'release' AND catchtype != 'squirrel' AND catchtype != 'sonnythrow'
			)`
		}

	// if first or last catch of a type was a mouth bonus catch
	// where the fish and the bonus were of the same type
	// this can select two catches for one type
	case "typefirst":
		if chatName != "global" {
			query = `
			SELECT f.weight, f.fishname, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid,
			RANK() OVER (ORDER BY f.date ASC)
			FROM fish f
			JOIN (
			SELECT MIN(date) AS min_date, fishname
			FROM fish
			WHERE chat = $1
			AND date < $2
			AND date > $3
			GROUP BY fishname
			) AS sub
			ON f.date = sub.min_date AND f.fishname = sub.fishname`

		} else {
			query = `
			SELECT f.weight, f.fishname, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid,
			RANK() OVER (ORDER BY f.date ASC)
			FROM fish f
			JOIN (
			SELECT MIN(date) AS min_date, fishname
			FROM fish
			WHERE date < $1
			AND date > $2
			GROUP BY fishname
			) AS sub
			ON f.date = sub.min_date AND f.fishname = sub.fishname`
		}

	case "typelast":
		if chatName != "global" {
			query = `
			SELECT f.weight, f.fishname, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid,
			RANK() OVER (ORDER BY f.date ASC)
			FROM fish f
			JOIN (
			SELECT MAX(date) AS max_date, fishname
			FROM fish
			WHERE chat = $1
			AND date < $2
			AND date > $3
			GROUP BY fishname
			) AS sub
			ON f.date = sub.max_date AND f.fishname = sub.fishname`

		} else {
			query = `
			SELECT f.weight, f.fishname, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid,
			RANK() OVER (ORDER BY f.date ASC)
			FROM fish f
			JOIN (
			SELECT MAX(date) AS max_date, fishname
			FROM fish
			WHERE date < $1
			AND date > $2
			GROUP BY fishname
			) AS sub
			ON f.date = sub.max_date AND f.fishname = sub.fishname`
		}
	}

	return query
}
