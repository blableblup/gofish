package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// add date and date 2 to the queries so that they are like the leaderboards
// so that they only count the data for the tournament week ?
func GetThePlayerProfiles(params LeaderboardParams, validPlayers []int, allShinies []string) (map[int]*PlayerProfile, error) {
	pool := params.Pool

	// pointer to the map so i can directly modify the maps
	Profiles := make(map[int]*PlayerProfile, len(validPlayers))

	// idk how to scan directly into the maps
	// so scan into this struct and then range over the struct to get the map
	// name of the rows needs to match the names here
	type Frick struct {
		FishInfo data.FishInfo
		String   string
		String2  string
		String3  string
		String4  []string
		String5  string
		Int      int
		PlayerID int
	}

	// The count per year per chat
	rows, err := pool.Query(context.Background(), `
		select count(*) as int, 
		to_char(date_trunc('year', date), 'YYYY') as string,
		chat as string2,
		playerid
		from fish 
		where playerid = any($1)
		group by string, string2, playerid
		order by string asc`,
		validPlayers)
	if err != nil {
		return Profiles, err
	}

	ChatCountsYear, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[Frick])
	if err != nil {
		return Profiles, err
	}

	for _, chatyear := range ChatCountsYear {

		// Add the playerid to the map first and get their name, their verified status and their twitchid
		if _, ok := Profiles[chatyear.PlayerID]; !ok {

			Profiles[chatyear.PlayerID] = &PlayerProfile{
				PlayerID:       chatyear.PlayerID,
				CountYear:      make(map[string]int),
				ChatCounts:     make(map[string]int),
				ChatCountsYear: make(map[string]map[string]int),
			}

			Profiles[chatyear.PlayerID].Name, _, Profiles[chatyear.PlayerID].Verified.Bool, Profiles[chatyear.PlayerID].TwitchID, err = PlayerStuff(chatyear.PlayerID, params, pool)
			if err != nil {
				return Profiles, err
			}

			if Profiles[chatyear.PlayerID].TwitchID == 0 {
				logs.Logs().Error().
					Str("Player", Profiles[chatyear.PlayerID].Name).
					Int("PlayerID", chatyear.PlayerID).
					Msg("Player does not have a twitchID in the DB!")
			}

			Profiles[chatyear.PlayerID].PlayerID = chatyear.PlayerID
		}

		if Profiles[chatyear.PlayerID].ChatCountsYear[chatyear.String] == nil {
			Profiles[chatyear.PlayerID].ChatCountsYear[chatyear.String] = make(map[string]int)
		}
		Profiles[chatyear.PlayerID].ChatCountsYear[chatyear.String][chatyear.String2] = chatyear.Int

		// Calculate the total count, the count per year and the count per chat
		Profiles[chatyear.PlayerID].Count = Profiles[chatyear.PlayerID].Count + chatyear.Int
		Profiles[chatyear.PlayerID].CountYear[chatyear.String] = Profiles[chatyear.PlayerID].CountYear[chatyear.String] + chatyear.Int
		Profiles[chatyear.PlayerID].ChatCounts[chatyear.String2] = Profiles[chatyear.PlayerID].ChatCounts[chatyear.String2] + chatyear.Int
	}

	// // their 10 biggest fish, can put the limit into the config to change it ?
	// rows, err = pool.Query(context.Background(),
	// 	`SELECT weight, fishtype as type, fishname as typename, bot, chat, date, catchtype, fishid, chatid
	// 	FROM fish
	// 	WHERE playerid = $1
	// 	ORDER BY weight desc
	// 	LIMIT 10`, playerID)
	// if err != nil {
	// 	return Profile, err
	// }

	// Profile.BiggestFish, err = pgx.CollectRows(rows, pgx.RowToStructByNameLax[data.FishInfo])
	// if err != nil {
	// 	return Profile, err
	// }

	// // their last fish
	// rows, err = pool.Query(context.Background(),
	// 	`SELECT weight, fishtype as type, fishname as typename, bot, chat, date, catchtype, fishid, chatid
	// 	FROM fish
	// 	WHERE playerid = $1
	// 	ORDER BY date desc
	// 	LIMIT 10`, playerID)
	// if err != nil {
	// 	return Profile, err
	// }

	// Profile.LastFish, err = pgx.CollectRows(rows, pgx.RowToStructByNameLax[data.FishInfo])
	// if err != nil {
	// 	return Profile, err
	// }

	// The last seen bag
	rows, err = pool.Query(context.Background(),
		`select bag, bot, chat, date, b.playerid
		from bag b
		join
		(
		select max(date) as max_date, playerid
		from bag
		where playerid = any($1)
		group by playerid
		) bag on b.playerid = bag.playerid and b.date = bag.max_date`,
		validPlayers)
	if err != nil {
		return Profiles, err
	}

	bags, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[data.FishInfo])
	if err != nil && err != pgx.ErrNoRows {
		return Profiles, err
	}

	for _, lastBag := range bags {

		// If a player never did +bag this will just be nothing
		Profiles[lastBag.PlayerID].Bag = lastBag

		// Count the items in their bag
		Profiles[lastBag.PlayerID].BagCounts = make(map[string]int)
		for n, ItemInBag := range Profiles[lastBag.PlayerID].Bag.Bag {

			var isashiny bool
			for _, shiny := range allShinies {
				if ItemInBag == shiny {

					isashiny = true

					// Change the item to the shiny emoji
					shinyEmoji := fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/shiny/%s.png)", ItemInBag, ItemInBag)

					Profiles[lastBag.PlayerID].Bag.Bag[n] = shinyEmoji

					Profiles[lastBag.PlayerID].BagCounts[shinyEmoji]++
				}
			}

			// Dont count the item again if it is a shiny
			if !isashiny {
				Profiles[lastBag.PlayerID].BagCounts[ItemInBag]++
			}
		}
	}

	// The first seen bag which had the ✉️ letter in it
	rows, err = pool.Query(context.Background(),
		`select bag, bot, chat, date, b.playerid
		from bag b
		join
		(
		select min(date) as min_date, playerid
		from bag
		where playerid = any($1)
		and '✉️' = any(bag)
		group by playerid
		) bag on b.playerid = bag.playerid and b.date = bag.min_date`,
		validPlayers)
	if err != nil {
		return Profiles, err
	}

	letterbag, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[data.FishInfo])
	if err != nil && err != pgx.ErrNoRows {
		return Profiles, err
	}

	for _, bag := range letterbag {
		Profiles[bag.PlayerID].HasLetter = true
	}

	// The fish type caught count per year per chat per catchtype
	rows, err = pool.Query(context.Background(), `
		select count(*) as int,
		fishname as string,
		chat as string2,
		to_char(date_trunc('year', date), 'YYYY') as string3,
		catchtype as string5,
		playerid
		from fish
		where playerid = any($1)
		group by string, string2, string3, string5, playerid`,
		validPlayers)
	if err != nil {
		return Profiles, err
	}

	FishTypesCaughtCountYearChat, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[Frick])
	if err != nil {
		return Profiles, err
	}

	// i dotn think there is a more readablew way to do this orm
	// also if fish was caught in multiple ways the count doesnt add up on board
	for _, chatyear := range FishTypesCaughtCountYearChat {

		if Profiles[chatyear.PlayerID].CountCatchtype == nil {
			Profiles[chatyear.PlayerID].CountCatchtype = make(map[string]int)
			Profiles[chatyear.PlayerID].CountCatchtypeChat = make(map[string]map[string]int)
			Profiles[chatyear.PlayerID].FishTypesCaughtCountCatchtype = make(map[string]map[string]int)
			Profiles[chatyear.PlayerID].FishTypesCaughtCountCatchtypeChat = make(map[string]map[string]map[string]int)

			Profiles[chatyear.PlayerID].FishTypesCaughtCount = make(map[string]int)
			Profiles[chatyear.PlayerID].FishTypesCaughtCountChat = make(map[string]map[string]int)
			Profiles[chatyear.PlayerID].FishTypesCaughtCountYear = make(map[string]map[string]int)
			Profiles[chatyear.PlayerID].FishTypesCaughtCountYearChat = make(map[string]map[string]map[string]int)
			Profiles[chatyear.PlayerID].FishTypesCaughtCountYearChatCatchtype = make(map[string]map[string]map[string]map[string]int)
		}

		if Profiles[chatyear.PlayerID].FishTypesCaughtCountYearChatCatchtype[chatyear.String] == nil {
			Profiles[chatyear.PlayerID].FishTypesCaughtCountYearChatCatchtype[chatyear.String] = make(map[string]map[string]map[string]int)
		}

		if Profiles[chatyear.PlayerID].FishTypesCaughtCountYearChatCatchtype[chatyear.String][chatyear.String3] == nil {
			Profiles[chatyear.PlayerID].FishTypesCaughtCountYearChatCatchtype[chatyear.String][chatyear.String3] = make(map[string]map[string]int)
		}

		if Profiles[chatyear.PlayerID].FishTypesCaughtCountYearChatCatchtype[chatyear.String][chatyear.String3][chatyear.String2] == nil {
			Profiles[chatyear.PlayerID].FishTypesCaughtCountYearChatCatchtype[chatyear.String][chatyear.String3][chatyear.String2] = make(map[string]int)
		}

		Profiles[chatyear.PlayerID].FishTypesCaughtCountYearChatCatchtype[chatyear.String][chatyear.String3][chatyear.String2][chatyear.String5] = chatyear.Int

		if Profiles[chatyear.PlayerID].FishTypesCaughtCountYearChat[chatyear.String] == nil {
			Profiles[chatyear.PlayerID].FishTypesCaughtCountYearChat[chatyear.String] = make(map[string]map[string]int)
		}
		if Profiles[chatyear.PlayerID].FishTypesCaughtCountYearChat[chatyear.String][chatyear.String3] == nil {
			Profiles[chatyear.PlayerID].FishTypesCaughtCountYearChat[chatyear.String][chatyear.String3] = make(map[string]int)
		}
		Profiles[chatyear.PlayerID].FishTypesCaughtCountYearChat[chatyear.String][chatyear.String3][chatyear.String2] = chatyear.Int

		Profiles[chatyear.PlayerID].FishTypesCaughtCount[chatyear.String] = Profiles[chatyear.PlayerID].FishTypesCaughtCount[chatyear.String] + chatyear.Int

		if Profiles[chatyear.PlayerID].FishTypesCaughtCountChat[chatyear.String] == nil {
			Profiles[chatyear.PlayerID].FishTypesCaughtCountChat[chatyear.String] = make(map[string]int)
		}
		Profiles[chatyear.PlayerID].FishTypesCaughtCountChat[chatyear.String][chatyear.String2] = Profiles[chatyear.PlayerID].FishTypesCaughtCountChat[chatyear.String][chatyear.String2] + chatyear.Int

		if Profiles[chatyear.PlayerID].FishTypesCaughtCountYear[chatyear.String] == nil {
			Profiles[chatyear.PlayerID].FishTypesCaughtCountYear[chatyear.String] = make(map[string]int)
		}
		Profiles[chatyear.PlayerID].FishTypesCaughtCountYear[chatyear.String][chatyear.String3] = Profiles[chatyear.PlayerID].FishTypesCaughtCountYear[chatyear.String][chatyear.String3] + chatyear.Int

		Profiles[chatyear.PlayerID].CountCatchtype[chatyear.String5] = Profiles[chatyear.PlayerID].CountCatchtype[chatyear.String5] + chatyear.Int

		if Profiles[chatyear.PlayerID].CountCatchtypeChat[chatyear.String5] == nil {
			Profiles[chatyear.PlayerID].CountCatchtypeChat[chatyear.String5] = make(map[string]int)
		}

		Profiles[chatyear.PlayerID].CountCatchtypeChat[chatyear.String5][chatyear.String2] = Profiles[chatyear.PlayerID].CountCatchtypeChat[chatyear.String5][chatyear.String2] + chatyear.Int

		if Profiles[chatyear.PlayerID].FishTypesCaughtCountCatchtype[chatyear.String] == nil {
			Profiles[chatyear.PlayerID].FishTypesCaughtCountCatchtype[chatyear.String] = make(map[string]int)
		}

		Profiles[chatyear.PlayerID].FishTypesCaughtCountCatchtype[chatyear.String][chatyear.String5] = Profiles[chatyear.PlayerID].FishTypesCaughtCountCatchtype[chatyear.String][chatyear.String5] + chatyear.Int

		if Profiles[chatyear.PlayerID].FishTypesCaughtCountCatchtypeChat[chatyear.String] == nil {
			Profiles[chatyear.PlayerID].FishTypesCaughtCountCatchtypeChat[chatyear.String] = make(map[string]map[string]int)
		}

		if Profiles[chatyear.PlayerID].FishTypesCaughtCountCatchtypeChat[chatyear.String][chatyear.String5] == nil {
			Profiles[chatyear.PlayerID].FishTypesCaughtCountCatchtypeChat[chatyear.String][chatyear.String5] = make(map[string]int)
		}

		Profiles[chatyear.PlayerID].FishTypesCaughtCountCatchtypeChat[chatyear.String][chatyear.String5][chatyear.String2] = Profiles[chatyear.PlayerID].FishTypesCaughtCountCatchtypeChat[chatyear.String][chatyear.String5][chatyear.String2] + chatyear.Int
	}

	// all their fish seen; could get this from the fishtypescaughtcount maps
	// but this is also sorting them by name, so i dont need to sort them later
	rows, err = pool.Query(context.Background(),
		`select array_agg(distinct(fishname)) as string4, playerid
		from fish 
		where playerid = any($1)
		group by playerid`,
		validPlayers)
	if err != nil {
		return Profiles, err
	}

	fishseen, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[Frick])
	if err != nil && err != pgx.ErrNoRows {
		return Profiles, err
	}

	for _, fishu := range fishseen {

		Profiles[fishu.PlayerID].FishSeen = fishu.String4
	}

	// // the fish they never caught
	// err = pool.QueryRow(context.Background(),
	// 	`select array_agg(fishname)
	// 	from
	// 	(
	// 	select distinct(fishname) from fishinfo
	// 	except
	// 	select distinct(fishname) from fish where playerid = $1
	// 	order by fishname asc)`, playerID).Scan(&Profile.FishNotSeen)
	// if err != nil {
	// 	return Profile, err
	// }

	// The fishseen per chat count
	rows, err = pool.Query(context.Background(), `
		select count(distinct(fishname)) as int,
		chat as string,
		playerid
		from fish
		where playerid = any($1)
		group by string, playerid`,
		validPlayers)
	if err != nil {
		return Profiles, err
	}

	CountChat, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[Frick])
	if err != nil {
		return Profiles, err
	}

	for _, dada := range CountChat {
		if Profiles[dada.PlayerID].FishSeenChat == nil {
			Profiles[dada.PlayerID].FishSeenChat = make(map[string]int)
		}
		Profiles[dada.PlayerID].FishSeenChat[dada.String] = dada.Int
	}

	// For biggest and smallest ignore the fish which i dont see the weight of in the catch message (squirrels and release bonus fish)
	// Also for biggest and smallest im ordering by date desc, so that as the rows are being read, if someone caught that weight multiple times
	// it should always end up printing the oldest one with that weight on the profile
	queryBiggestFishChat := ConstructFishQuery(
		",MAX(weight) AS max_weight, chat",
		"AND catchtype != 'release' AND catchtype != 'squirrel'",
		",chat",
		"AND f.weight = sub.max_weight AND f.chat = sub.chat",
		"AND f.catchtype != 'release' AND f.catchtype != 'squirrel'",
		"ORDER BY date desc")

	Profiles, err = QueryMapStringFishInfo(pool, Profiles, queryBiggestFishChat, validPlayers, "biggest", true)
	if err != nil {
		return Profiles, err
	}

	// If first / last catch was a mouth bonus catch dont select the mouth catch,
	// So that there arent two fish with max / min date
	queryFirstFishChat := ConstructFishQuery(
		",MIN(date) AS min_date, chat",
		"",
		",chat",
		"AND f.date = sub.min_date AND f.chat = sub.chat",
		"AND f.catchtype != 'mouth'",
		"")

	Profiles, err = QueryMapStringFishInfo(pool, Profiles, queryFirstFishChat, validPlayers, "first", true)
	if err != nil {
		return Profiles, err
	}

	queryLastFishChat := ConstructFishQuery(
		",MAX(date) AS max_date, chat",
		"",
		",chat",
		"AND f.date = sub.max_date AND f.chat = sub.chat",
		"AND f.catchtype != 'mouth'",
		"")

	Profiles, err = QueryMapStringFishInfo(pool, Profiles, queryLastFishChat, validPlayers, "last", true)
	if err != nil {
		return Profiles, err
	}

	// Get the biggest, smallest, last and first fish per fishtype
	queryBiggestFishPerType := ConstructFishQuery(
		",fishname, MAX(weight) AS max_weight",
		"AND catchtype != 'release' AND catchtype != 'squirrel'",
		",fishname",
		"AND f.weight = sub.max_weight AND f.fishname = sub.fishname",
		"AND f.catchtype != 'release' AND f.catchtype != 'squirrel'",
		"ORDER BY date desc")

	Profiles, err = QueryMapStringFishInfo(pool, Profiles, queryBiggestFishPerType, validPlayers, "biggest", false)
	if err != nil {
		return Profiles, err
	}

	querySmallestFishPerType := ConstructFishQuery(
		",fishname, MIN(weight) AS min_weight",
		"AND catchtype != 'release' AND catchtype != 'squirrel'",
		",fishname",
		"AND f.weight = sub.min_weight AND f.fishname = sub.fishname",
		"AND f.catchtype != 'release' AND f.catchtype != 'squirrel'",
		"ORDER BY date desc")

	Profiles, err = QueryMapStringFishInfo(pool, Profiles, querySmallestFishPerType, validPlayers, "smallest", false)
	if err != nil {
		return Profiles, err
	}

	// If someones last catch of a type was a mouth catch and the fish in the mouth and the other catch are of the same type
	// this will select two fishes, doesnt rly happen a lot anyways so idc (?)
	queryLastFishPerType := ConstructFishQuery(
		",MAX(date) AS max_date, fishname",
		"",
		",fishname",
		"AND f.date = sub.max_date AND f.fishname = sub.fishname",
		"",
		"")

	Profiles, err = QueryMapStringFishInfo(pool, Profiles, queryLastFishPerType, validPlayers, "last", false)
	if err != nil {
		return Profiles, err
	}

	queryFirstFishPerType := ConstructFishQuery(
		",MIN(date) AS min_date, fishname",
		"",
		",fishname",
		"AND f.date = sub.min_date AND f.fishname = sub.fishname",
		"",
		"")

	Profiles, err = QueryMapStringFishInfo(pool, Profiles, queryFirstFishPerType, validPlayers, "first", false)
	if err != nil {
		return Profiles, err
	}

	return Profiles, nil
}

func QueryMapStringFishInfo(pool *pgxpool.Pool, Profiles map[int]*PlayerProfile, query string, validPlayers []int, whatmap string, chat bool) (map[int]*PlayerProfile, error) {

	rows, err := pool.Query(context.Background(), query, validPlayers)
	if err != nil {
		return Profiles, err
	}

	Fishes, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[data.FishInfo])
	if err != nil && err != pgx.ErrNoRows {
		return Profiles, err
	}

	switch whatmap {
	case "biggest":
		for _, fish := range Fishes {
			if chat {
				if Profiles[fish.PlayerID].BiggestFishChat == nil {
					Profiles[fish.PlayerID].BiggestFishChat = make(map[string]data.FishInfo)
				}

				Profiles[fish.PlayerID].BiggestFishChat[fish.Chat] = fish
			} else {
				if Profiles[fish.PlayerID].BiggestFishPerType == nil {
					Profiles[fish.PlayerID].BiggestFishPerType = make(map[string]data.FishInfo)
				}

				Profiles[fish.PlayerID].BiggestFishPerType[fish.TypeName] = fish
			}
		}
	case "smallest":
		for _, fish := range Fishes {
			if chat {
				// im not selecting the smallest fish per chat
				logs.Logs().Warn().Msg("No smallest fish per chat!")
			} else {
				if Profiles[fish.PlayerID].SmallestFishPerType == nil {
					Profiles[fish.PlayerID].SmallestFishPerType = make(map[string]data.FishInfo)
				}

				Profiles[fish.PlayerID].SmallestFishPerType[fish.TypeName] = fish
			}
		}
	case "first":
		for _, fish := range Fishes {
			if chat {
				if Profiles[fish.PlayerID].FirstFishChat == nil {
					Profiles[fish.PlayerID].FirstFishChat = make(map[string]data.FishInfo)
				}

				Profiles[fish.PlayerID].FirstFishChat[fish.Chat] = fish
			} else {
				if Profiles[fish.PlayerID].FirstCaughtFishPerType == nil {
					Profiles[fish.PlayerID].FirstCaughtFishPerType = make(map[string]data.FishInfo)
				}

				Profiles[fish.PlayerID].FirstCaughtFishPerType[fish.TypeName] = fish
			}
		}
	case "last":
		for _, fish := range Fishes {
			if chat {
				if Profiles[fish.PlayerID].LastFishChat == nil {
					Profiles[fish.PlayerID].LastFishChat = make(map[string]data.FishInfo)
				}

				Profiles[fish.PlayerID].LastFishChat[fish.Chat] = fish
			} else {
				if Profiles[fish.PlayerID].LastCaughtFishPerType == nil {
					Profiles[fish.PlayerID].LastCaughtFishPerType = make(map[string]data.FishInfo)
				}

				Profiles[fish.PlayerID].LastCaughtFishPerType[fish.TypeName] = fish
			}
		}
	default:
		logs.Logs().Warn().
			Str("MAP", whatmap).
			Msg("QueryMapStringFishInfo WRONG MAP!")
	}

	return Profiles, nil
}

func ConstructFishQuery(innerSelect string, ignoreCatchtypeInside string, groupByInside string, idk string, ignoreCatchtypeOutside string, orderDateOutside string) string {

	return fmt.Sprintf(` 
		SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
			SELECT playerid %s
			FROM fish
			WHERE playerid = any($1)
			%s
			GROUP BY playerid %s
		) AS sub
		ON f.playerid = sub.playerid %s %s
		WHERE f.playerid = any($1)
		%s`, innerSelect, ignoreCatchtypeInside, groupByInside, idk, ignoreCatchtypeOutside, orderDateOutside)

}
