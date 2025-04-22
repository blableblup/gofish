package leaderboards

import (
	"context"
	"database/sql"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"gofish/utils"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PlayerProfile struct {
	Name     string
	PlayerID int
	TwitchID int
	Verified sql.NullBool

	HasSeenTreasures bool // make this show when they caught the treasures for the first time
	HasLetter        bool // also make it show when they had it in their bag for first time
	HasShiny         bool

	Count              int
	CountYear          map[string]int
	ChatCounts         map[string]int
	ChatCountsYear     map[string]map[string]int
	CountCatchtype     map[string]int
	CountCatchtypeChat map[string]map[string]int

	BiggestFish     []data.FishInfo
	LastFish        []data.FishInfo
	FirstFish       data.FishInfo
	BiggestFishChat map[string]data.FishInfo
	LastFishChat    map[string]data.FishInfo
	FirstFishChat   map[string]data.FishInfo

	FishSeen     []string
	FishNotSeen  []string
	FishSeenChat map[string]int

	FishTypesCaughtCount                  map[string]int
	FishTypesCaughtCountChat              map[string]map[string]int
	FishTypesCaughtCountYear              map[string]map[string]int
	FishTypesCaughtCountYearChat          map[string]map[string]map[string]int
	FishTypesCaughtCountCatchtype         map[string]map[string]int
	FishTypesCaughtCountCatchtypeChat     map[string]map[string]map[string]int
	FishTypesCaughtCountYearChatCatchtype map[string]map[string]map[string]map[string]int

	BiggestFishPerType     map[string]data.FishInfo
	SmallestFishPerType    map[string]data.FishInfo
	FirstCaughtFishPerType map[string]data.FishInfo
	LastCaughtFishPerType  map[string]data.FishInfo

	Bag       data.FishInfo
	BagCounts map[string]int
}

func GetPlayerProfiles(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	config := params.Config
	limit := params.Limit
	pool := params.Pool

	var countlimit int
	var err error
	if limit == "" {
		// This can only be run as "global" so always go with default
		countlimit = config.Chat["default"].PlayerCountLimit
	} else {
		countlimit, err = strconv.Atoi(limit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Limit", limit).
				Str("Board", board).
				Msg("Error converting custom limit to int")
			return
		}
	}

	validPlayers, err := GetValidPlayers(params, countlimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error getting valid players")
		return
	}

	// I need to get the newest emoji per fishname
	// because there were fish which had different emojis in older logs (like ⛸ / ⛸️ for ice skate)
	// and i didnt change that
	// so get all the fish first and then use the utils function
	allFishes, err := GetAllFishNames(params)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error getting all fish names")
		return
	}

	FishWithEmoji := make(map[string]string)

	for _, fish := range allFishes {
		FishWithEmoji[fish], err = FishStuff(fish, params, pool)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Str("Fish", fish).
				Msg("Error getting fish emoji")
			return
		}
	}

	// get the names for the different type of ways you can catch fish
	Catchtypenames := CatchtypeNames()

	// Get the players profile and print it for each player

	logs.Logs().Info().
		Int("Amount of players", len(validPlayers)).
		Msg("Updating player profiles")

	playerProfiles, err := GetThePlayerProfiles(params, validPlayers)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error getting player profiles")
	}

	wg := new(sync.WaitGroup)

	for _, validPlayer := range validPlayers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			err = PrintPlayerProfile(playerProfiles[validPlayer], FishWithEmoji, Catchtypenames)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Chat", chatName).
					Str("Board", board).
					Int("PlayerID", validPlayer).
					Msg("Error printing player profile")
			}
		}()
	}

	wg.Wait()

	logs.Logs().Info().
		Str("Chat", chatName).
		Str("Board", board).
		Msg("Done updating player profiles")

}

func GetValidPlayers(params LeaderboardParams, limit int) ([]int, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	date2 := params.Date2
	date := params.Date
	mode := params.Mode
	pool := params.Pool

	var validPlayers []int
	var rows pgx.Rows
	var err error

	// If nothing else was specified for date 2 and its the default date
	// have date 2 be 7 days before date1, so the players above the limit who fished in the last 7 days are selected
	// because ill update this with the other leaderboards on sunday
	if date2 == "2022-12-03" {
		datetime, err := utils.ParseDate(date)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Date", date).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error parsing date into time.Time for active fishers")
			return validPlayers, err
		}
		date2 = datetime.AddDate(0, 0, -7).Format("2006-01-2")
	}

	// select all the players
	queryall := `
		select playerid from fish
		group by playerid
		having count(*) >= $1`

	// only select the players who fished in that time period and have their total count above the limit
	queryrecent := `
	select f.playerid from fish f
		join 
		(
		select playerid from fish
		where date < $2
		and date >= $3
			group by playerid
		) bla on bla.playerid = f.playerid
		group by f.playerid
		having count(*) >= $1
	`

	if mode == "force" {

		rows, err = pool.Query(context.Background(), queryall, limit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error querying database")
			return validPlayers, err
		}
		defer rows.Close()

	} else {

		rows, err = pool.Query(context.Background(), queryrecent, limit, date, date2)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error querying database")
			return validPlayers, err
		}
		defer rows.Close()

	}

	for rows.Next() {

		var playerID int

		if err := rows.Scan(&playerID); err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error scanning row")
			return validPlayers, err
		}

		validPlayers = append(validPlayers, playerID)
	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error iterating over rows")
		return validPlayers, err
	}

	return validPlayers, nil
}

// add date and date 2 to the queries so that they are like the leaderboards
// so that they only count the data for the tournament week ?
func GetThePlayerProfiles(params LeaderboardParams, validPlayers []int) (map[int]*PlayerProfile, error) {
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

	// also check for shiny in bag!
	bags, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[data.FishInfo])
	if err != nil && err != pgx.ErrNoRows {
		return Profiles, err
	}

	for _, lastBag := range bags {

		// If a player never did +bag this will just be nothing
		Profiles[lastBag.PlayerID].Bag = lastBag

		// Count the items in their bag
		Profiles[lastBag.PlayerID].BagCounts = make(map[string]int)
		for _, ItemInBag := range Profiles[lastBag.PlayerID].Bag.Bag {
			Profiles[lastBag.PlayerID].BagCounts[ItemInBag]++
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

	// the queries here are mostly the same, can have a base query and then construct the others
	queryBiggestFishChat := `
		SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
			SELECT MAX(weight) AS max_weight, chat, playerid
			FROM fish
			WHERE playerid = any($1)
			AND catchtype != 'release'
			AND catchtype != 'squirrel'
			GROUP BY chat, playerid
		) AS sub
		ON f.weight = sub.max_weight AND f.chat = sub.chat AND f.playerid = sub.playerid
		WHERE f.playerid = any($1)`

	Profiles, err = QueryMapStringFishInfo(pool, Profiles, queryBiggestFishChat, validPlayers, "biggest", true)
	if err != nil {
		return Profiles, err
	}

	// if first / last catch was a mouth bonus catch dont select the mouth catch,
	// so that there arent two fish with max / min date
	queryFirstFishChat := `
	SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
	FROM fish f
	JOIN (
		SELECT MIN(date) AS min_date, chat, playerid
		FROM fish
		WHERE playerid = any($1)
		AND catchtype != 'mouth'
		GROUP BY chat, playerid
	) AS sub
	ON f.date = sub.min_date AND f.chat = sub.chat AND f.playerid = sub.playerid
	WHERE f.playerid = any($1)`

	Profiles, err = QueryMapStringFishInfo(pool, Profiles, queryFirstFishChat, validPlayers, "first", true)
	if err != nil {
		return Profiles, err
	}

	queryLastFishChat := `
	SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
	FROM fish f
	JOIN (
		SELECT MAX(date) AS max_date, chat, playerid
		FROM fish
		WHERE playerid = any($1)
		AND catchtype != 'mouth'
		GROUP BY chat, playerid
	) AS sub
	ON f.date = sub.max_date AND f.chat = sub.chat AND f.playerid = sub.playerid
	WHERE f.playerid = any($1)`

	Profiles, err = QueryMapStringFishInfo(pool, Profiles, queryLastFishChat, validPlayers, "last", true)
	if err != nil {
		return Profiles, err
	}

	// Get the biggest, smallest, last and first fish per fishtype
	// For biggest and smallest ignore the fish which i dont see the weight of in the catch message (squirrels and release bonus fish)
	queryBiggestFishPerType := `
		SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
			SELECT fishname, MAX(weight) AS max_weight, playerid
			FROM fish
			WHERE playerid = any($1)
			AND catchtype != 'release'
			AND catchtype != 'squirrel'
			GROUP BY fishname, playerid
		) AS sub
		ON f.fishname = sub.fishname AND f.weight = sub.max_weight AND f.playerid = sub.playerid
		WHERE f.playerid = any($1)`

	Profiles, err = QueryMapStringFishInfo(pool, Profiles, queryBiggestFishPerType, validPlayers, "biggest", false)
	if err != nil {
		return Profiles, err
	}

	querySmallestFishPerType := `
		SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
			SELECT fishname, MIN(weight) AS min_weight, playerid
			FROM fish
			WHERE playerid = any($1)
			AND catchtype != 'release'
			AND catchtype != 'squirrel'
			GROUP BY fishname, playerid
		) AS sub
		ON f.fishname = sub.fishname AND f.weight = sub.min_weight AND f.playerid = sub.playerid
		WHERE f.playerid = any($1)`

	Profiles, err = QueryMapStringFishInfo(pool, Profiles, querySmallestFishPerType, validPlayers, "smallest", false)
	if err != nil {
		return Profiles, err
	}

	queryLastFishPerType := `
		SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
			SELECT fishname, MAX(date) AS max_date, playerid
			FROM fish
			WHERE playerid = any($1)
			GROUP BY fishname, playerid
		) AS sub
		ON f.fishname = sub.fishname AND f.date = sub.max_date AND f.playerid = sub.playerid
		WHERE f.playerid = any($1)`

	Profiles, err = QueryMapStringFishInfo(pool, Profiles, queryLastFishPerType, validPlayers, "last", false)
	if err != nil {
		return Profiles, err
	}

	queryFirstFishPerType := `
		SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
			SELECT fishname, MIN(date) AS min_date, playerid
			FROM fish
			WHERE playerid = any($1)
			GROUP BY fishname, playerid
		) AS sub
		ON f.fishname = sub.fishname AND f.date = sub.min_date AND f.playerid = sub.playerid
		WHERE f.playerid = any($1)`

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

// can put code which is printing the same type of maps into their own function ?
func PrintPlayerProfile(Profile *PlayerProfile, EmojisForFish map[string]string, CatchtypeNames map[string]string) error {

	filePath := filepath.Join("leaderboards", "global", "players", fmt.Sprintf("%d", Profile.TwitchID)+".md")

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, _ = fmt.Fprintf(file, "# %s", Profile.Name)
	// something here to show that they caught all the treasures and have gotten a letter ?

	_, _ = fmt.Fprintln(file, "\n## Data for their fish caught")

	_, _ = fmt.Fprintf(file, "\n| Total fish caught | %d |", Profile.Count)

	_, _ = fmt.Fprintln(file, "\n|-------|-------|")

	_, _ = fmt.Fprintln(file, "\n\nFish caught per chat")

	_, _ = fmt.Fprintln(file, "\n| Rank | Chat | Count |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|")

	rank := 1 // make chats with same count have same rank

	sortedChatCounts := sortMapString(Profile.ChatCounts, "countdesc")

	for _, chat := range sortedChatCounts {
		_, _ = fmt.Fprintf(file, "\n| %s | %s %s | %d |",
			Ranks(rank),
			chat,
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat),
			Profile.ChatCounts[chat])
		rank++
	}

	_, _ = fmt.Fprintln(file, "\n\nFish caught per year")

	_, _ = fmt.Fprintln(file, "\n| --- | Year | Count | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|")

	rank = 1

	sortedYearCounts := sortMapString(Profile.CountYear, "nameasc")

	for _, year := range sortedYearCounts {
		_, _ = fmt.Fprintf(file, "\n| %d | %s | %d |",
			rank,
			year,
			Profile.CountYear[year])

		sortedChatCountsYear := sortMapString(Profile.ChatCountsYear[year], "countdesc")

		for _, chat := range sortedChatCountsYear {
			_, _ = fmt.Fprintf(file, " %s %d ",
				fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat),
				Profile.ChatCountsYear[year][chat])
		}
		_, _ = fmt.Fprint(file, "|")
		rank++
	}

	_, _ = fmt.Fprintln(file, "\n\nFish caught per catchtype")

	_, _ = fmt.Fprintln(file, "\n| --- | Catchtype | Count | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|")

	rank = 1

	sortedCatchtypes := sortMapString(Profile.CountCatchtype, "countdesc")

	for _, catch := range sortedCatchtypes {

		catchtype := CatchtypeNames[catch]

		_, _ = fmt.Fprintf(file, "\n| %d | %s | %d |",
			rank,
			catchtype,
			Profile.CountCatchtype[catch])

		sortedChatCounts := sortMapString(Profile.CountCatchtypeChat[catch], "countdesc")

		for _, chat := range sortedChatCounts {
			_, _ = fmt.Fprintf(file, " %s %d",
				fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat),
				Profile.CountCatchtypeChat[catch][chat])
		}

		_, _ = fmt.Fprint(file, "|")
		rank++
	}

	_, _ = fmt.Fprintln(file, "\n## First, biggest and last fish")
	// Make it show catchtype here ? and in the fish seen part

	_, _ = fmt.Fprintln(file, "\n\nFirst ever fish caught per chat")

	_, _ = fmt.Fprintln(file, "\n| --- | Fish | Weight in lbs | Date in UTC | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|-------|")

	rank = 1

	sortedChatDates := sortMapStringFishInfo(Profile.FirstFishChat, "dateasc")

	for _, chat := range sortedChatDates {
		_, _ = fmt.Fprintf(file, "\n| %d | %s %s | %.2f | %s | %s |",
			rank,
			EmojisForFish[Profile.FirstFishChat[chat].TypeName],
			Profile.FirstFishChat[chat].TypeName,
			Profile.FirstFishChat[chat].Weight,
			Profile.FirstFishChat[chat].Date.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat))
		rank++
	}

	_, _ = fmt.Fprintln(file, "\n\nLast fish caught per chat")

	_, _ = fmt.Fprintln(file, "\n| --- | Fish | Weight in lbs | Date in UTC | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|-------|")

	rank = 1

	sortedChatDates2 := sortMapStringFishInfo(Profile.LastFishChat, "dateasc")

	for _, chat := range sortedChatDates2 {
		_, _ = fmt.Fprintf(file, "\n| %d | %s %s | %.2f | %s | %s |",
			rank,
			EmojisForFish[Profile.LastFishChat[chat].TypeName],
			Profile.LastFishChat[chat].TypeName,
			Profile.LastFishChat[chat].Weight,
			Profile.LastFishChat[chat].Date.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat))
		rank++
	}

	_, _ = fmt.Fprintln(file, "\n\nBiggest fish caught per chat")

	_, _ = fmt.Fprintln(file, "\n| --- | Fish | Weight in lbs | Date in UTC | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|-------|")

	rank = 1

	sortedChatWeights := sortMapStringFishInfo(Profile.BiggestFishChat, "weightdesc")

	for _, chat := range sortedChatWeights {
		_, _ = fmt.Fprintf(file, "\n| %d | %s %s | %.2f | %s | %s |",
			rank,
			EmojisForFish[Profile.BiggestFishChat[chat].TypeName],
			Profile.BiggestFishChat[chat].TypeName,
			Profile.BiggestFishChat[chat].Weight,
			Profile.BiggestFishChat[chat].Date.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat))
		rank++
	}

	_, _ = fmt.Fprintln(file, "\n\nTheir overall biggest fish caught")

	_, _ = fmt.Fprintln(file, "\n| Rank | Fish | Weight in lbs | Date in UTC | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|-------|")

	rank = 1

	for _, Fish := range Profile.BiggestFish {
		_, _ = fmt.Fprintf(file, "\n| %s | %s %s | %.2f | %s | %s |",
			Ranks(rank),
			Fish.Type,
			Fish.TypeName,
			Fish.Weight,
			Fish.Date.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", Fish.Chat, Fish.Chat))
		rank++
	}

	_, _ = fmt.Fprintln(file, "\n\nTheir overall last fish caught")

	_, _ = fmt.Fprintln(file, "\n| --- | Fish | Weight in lbs | Date in UTC | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|-------|")

	rank = 1

	for _, Fish := range Profile.LastFish {
		_, _ = fmt.Fprintf(file, "\n| %d | %s %s | %.2f | %s | %s |",
			rank,
			Fish.Type,
			Fish.TypeName,
			Fish.Weight,
			Fish.Date.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", Fish.Chat, Fish.Chat))
		rank++
	}

	_, _ = fmt.Fprintln(file, "\n## Their fish seen")

	_, _ = fmt.Fprintf(file, "\n| Total fish seen | %d |", len(Profile.FishSeen))

	_, _ = fmt.Fprintln(file, "\n|-------|-------|")

	_, _ = fmt.Fprintln(file, "\nFish seen per chat")

	_, _ = fmt.Fprintln(file, "\n| Rank | Chat | Count |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|")

	rank = 1

	sortedChatCounts = sortMapString(Profile.FishSeenChat, "countdesc")

	for _, chat := range sortedChatCounts {
		_, _ = fmt.Fprintf(file, "\n| %s | %s %s | %d |",
			Ranks(rank),
			chat,
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat),
			Profile.FishSeenChat[chat])
		rank++
	}

	_, _ = fmt.Fprintln(file)

	_, _ = fmt.Fprintln(file, "\n## Data about each of their seen fish")

	// print one block for each fish type
	// show their total coutn caught, count per year per chat
	for _, fish := range Profile.FishSeen {

		_, _ = fmt.Fprintf(file, "\n| %s %s | Total caught | %d |",
			EmojisForFish[fish],
			fish,
			Profile.FishTypesCaughtCount[fish])

		_, _ = fmt.Fprintln(file, "\n|-------|-------|-------|")

		_, _ = fmt.Fprintf(file, "\n| %s | Year | Count | Chat |\n", EmojisForFish[fish])

		_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|")

		rank = 1

		for _, year := range sortedYearCounts {

			// Skip the fish not caught in that year
			if Profile.FishTypesCaughtCountYear[fish][year] == 0 {
				continue
			}

			_, _ = fmt.Fprintf(file, "\n| %d | %s | %d |",
				rank,
				year,
				Profile.FishTypesCaughtCountYear[fish][year])

			sortedChatCountsType := sortMapString(Profile.FishTypesCaughtCountYearChat[fish][year], "countdesc")

			for _, chat := range sortedChatCountsType {

				_, _ = fmt.Fprintf(file, " %s %d",
					fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat),
					Profile.FishTypesCaughtCountYearChat[fish][year][chat])
			}

			_, _ = fmt.Fprint(file, " |")

			rank++

		}
		_, _ = fmt.Fprintln(file)

		_, _ = fmt.Fprintf(file, "\n| %s | Catchtype | Count | Chat |\n", EmojisForFish[fish])

		_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|")

		rank = 1

		sortedCatchtypes := sortMapString(Profile.FishTypesCaughtCountCatchtype[fish], "countdesc")

		for _, catch := range sortedCatchtypes {

			catchtype := CatchtypeNames[catch]

			_, _ = fmt.Fprintf(file, "\n| %d | %s | %d |",
				rank,
				catchtype,
				Profile.FishTypesCaughtCountCatchtype[fish][catch])

			sortedChatCountsType := sortMapString(Profile.FishTypesCaughtCountCatchtypeChat[fish][catch], "countdesc")

			for _, chat := range sortedChatCountsType {

				_, _ = fmt.Fprintf(file, " %s %d",
					fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat),
					Profile.FishTypesCaughtCountCatchtypeChat[fish][catch][chat])
			}

			_, _ = fmt.Fprint(file, " |")

			rank++

		}
		_, _ = fmt.Fprintln(file)

		_, _ = fmt.Fprintf(file, "\n| %s | Weight in lbs | Date in UTC | Chat |\n", EmojisForFish[fish])

		_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|")

		MapsToUse := []map[string]data.FishInfo{Profile.FirstCaughtFishPerType, Profile.LastCaughtFishPerType, Profile.BiggestFishPerType, Profile.SmallestFishPerType}
		Stringy := []string{"First Caught", "Last caught", "Biggest", "Smallest"}

		for Inty, Mup := range MapsToUse {
			_, _ = fmt.Fprintf(file, "\n| %s | %.2f | %s | %s |",
				Stringy[Inty],
				Mup[fish].Weight,
				Mup[fish].Date.Format("2006-01-02 15:04:05"),
				fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)",
					Mup[fish].Chat,
					Mup[fish].Chat))
		}

		_, _ = fmt.Fprintln(file)
	}

	// show what fish they never caught
	_, _ = fmt.Fprintln(file, "\n## Fish they have never seen")

	for _, fish := range Profile.FishNotSeen {

		_, _ = fmt.Fprintf(file, "\n* %s %s", EmojisForFish[fish], fish)

	}

	_, _ = fmt.Fprintf(file, "\n\nIn total %d fish never seen", len(Profile.FishNotSeen))

	_, _ = fmt.Fprintln(file, "\n## Their last seen bag")

	_, _ = fmt.Fprintln(file, "\n| Bag | Date in UTC | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|")

	_, _ = fmt.Fprintf(file, "\n| %s | %s | %s |",
		Profile.Bag.Bag,
		Profile.Bag.Date.Format("2006-01-02 15:04:05"),
		fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", Profile.Bag.Chat, Profile.Bag.Chat))

	_, _ = fmt.Fprintln(file, "\n\nCount of items in that bag:")

	sortedBagCounts := sortMapString(Profile.BagCounts, "countdesc")

	for _, bagItem := range sortedBagCounts {
		_, _ = fmt.Fprintf(file, " [%s %d]",
			bagItem,
			Profile.BagCounts[bagItem])
	}

	return nil
}
