package leaderboards

import (
	"context"
	"database/sql"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"
)

type PlayerProfile struct {
	Name     string
	PlayerID int
	TwitchID sql.NullInt64
	Verified sql.NullBool

	Count          int
	CountYear      map[string]int
	ChatCounts     map[string]int
	ChatCountsYear map[string]map[string]int
	CountCatchtype map[string]int
	// could also have the count catchtype per year per chat ?

	BiggestFish []data.FishInfo
	LastFish    []data.FishInfo // Last and first fish per chat ?
	FirstFish   data.FishInfo

	FishSeen     []string
	FishNotSeen  []string
	FishSeenChat map[string][]string

	FishTypesCaughtCount                  map[string]int
	FishTypesCaughtCountChat              map[string]map[string]int
	FishTypesCaughtCountYear              map[string]map[string]int
	FishTypesCaughtCountYearChat          map[string]map[string]map[string]int
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

	// instead of always updating it for all players
	//  only select the players who fished in the last seven days above the limit and update it for them only ?
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

	// This can just be a normal sync waitgroup
	// im not doing anythign with an error
	g := new(errgroup.Group)

	// Get the players profile and print it for each player
	for _, validPlayer := range validPlayers {
		g.Go(func() error {
			playerProfile, err := GetAPlayerProfile(params, validPlayer)
			if err == nil {
				err = PrintPlayerProfile(playerProfile, FishWithEmoji)
			}
			return err
		})
	}

	// this will be the first non nil error
	if err := g.Wait(); err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error with player profiles")
		return
	}

	logs.Logs().Info().
		Msg("Done printing player profiles")

}

func GetValidPlayers(params LeaderboardParams, limit int) ([]int, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	pool := params.Pool

	var validPlayers []int

	// Query for all players above the countlimit
	rows, err := pool.Query(context.Background(), `
		select playerid from fish
		group by playerid
		having count(*) >= $1`, limit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error querying database")
		return validPlayers, err
	}
	defer rows.Close()

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

func GetAllFishNames(params LeaderboardParams) ([]string, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	pool := params.Pool

	var fishes []string

	rows, err := pool.Query(context.Background(), `
		select fishname from fishinfo
		group by fishname`)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error querying database")
		return fishes, err
	}
	defer rows.Close()

	for rows.Next() {

		var fishy string

		if err := rows.Scan(&fishy); err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error scanning row")
			return fishes, err
		}

		fishes = append(fishes, fishy)
	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error iterating over rows")
		return fishes, err
	}

	return fishes, nil
}

func GetAPlayerProfile(params LeaderboardParams, playerID int) (PlayerProfile, error) {
	pool := params.Pool

	var Profile PlayerProfile

	Profile.PlayerID = playerID

	// Get the data for each player

	// add date and date 2 to the queries so that they are like the leaderboards
	// so that they only count the data for the tournament week ? or

	// For this I already have PlayerStuff function in utilsleaderboard ?
	// add twitchid to that function or?
	err := pool.QueryRow(context.Background(),
		"select name, twitchid, verified from playerdata where playerid = $1",
		playerID).Scan(&Profile.Name, &Profile.TwitchID, &Profile.Verified)
	if err != nil {
		return Profile, err
	}

	if !Profile.TwitchID.Valid {
		logs.Logs().Error().
			Str("Player", Profile.Name).
			Int("PlayerID", Profile.PlayerID).
			Msg("Player does not have a twitchID in the DB!")
	}

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
	}

	// The count per year per chat
	rows, err := pool.Query(context.Background(), `
		select count(*) as int, 
		to_char(date_trunc('year', date), 'YYYY') as string,
		chat as string2
		from fish 
		where playerid = $1
		group by string, string2
		order by string asc
		`,
		playerID)
	if err != nil {
		return Profile, err
	}

	ChatCountsYear, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[Frick])
	if err != nil {
		return Profile, err
	}

	Profile.CountYear = make(map[string]int)
	Profile.ChatCounts = make(map[string]int)
	Profile.ChatCountsYear = make(map[string]map[string]int)

	for _, chatyear := range ChatCountsYear {
		// Need to initialize both maps, else nil panic
		if Profile.ChatCountsYear[chatyear.String] == nil {
			Profile.ChatCountsYear[chatyear.String] = make(map[string]int)
		}
		Profile.ChatCountsYear[chatyear.String][chatyear.String2] = chatyear.Int

		// Calculate the total count, the count per year and the count per chat
		Profile.Count = Profile.Count + chatyear.Int
		Profile.CountYear[chatyear.String] = Profile.CountYear[chatyear.String] + chatyear.Int
		Profile.ChatCounts[chatyear.String2] = Profile.ChatCounts[chatyear.String2] + chatyear.Int
	}

	// The count per catchtype
	rows, err = pool.Query(context.Background(), `
		select count(*) as int, 
		catchtype as string 
		from fish 
		where playerid = $1
		group by string
		`,
		playerID)
	if err != nil {
		return Profile, err
	}

	CountCatchtype, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[Frick])
	if err != nil {
		return Profile, err
	}

	Profile.CountCatchtype = make(map[string]int)

	for _, catch := range CountCatchtype {
		Profile.CountCatchtype[catch.String] = catch.Int
	}

	// A players first ever fish
	rows, err = pool.Query(context.Background(),
		`SELECT weight, fishtype as type, fishname as typename, bot, chat, date, catchtype, fishid, chatid
		FROM fish 
		WHERE playerid = $1
		ORDER BY date asc
		LIMIT 1`, playerID)
	if err != nil {
		return Profile, err
	}

	Profile.FirstFish, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[data.FishInfo])
	if err != nil {
		return Profile, err
	}

	// their 10 biggest fish, can put the limit into the config to change it ?
	rows, err = pool.Query(context.Background(),
		`SELECT weight, fishtype as type, fishname as typename, bot, chat, date, catchtype, fishid, chatid
		FROM fish 
		WHERE playerid = $1
		ORDER BY weight desc
		LIMIT 10`, playerID)
	if err != nil {
		return Profile, err
	}

	Profile.BiggestFish, err = pgx.CollectRows(rows, pgx.RowToStructByNameLax[data.FishInfo])
	if err != nil {
		return Profile, err
	}

	// their last fish
	rows, err = pool.Query(context.Background(),
		`SELECT weight, fishtype as type, fishname as typename, bot, chat, date, catchtype, fishid, chatid
		FROM fish 
		WHERE playerid = $1
		ORDER BY date desc
		LIMIT 10`, playerID)
	if err != nil {
		return Profile, err
	}

	Profile.LastFish, err = pgx.CollectRows(rows, pgx.RowToStructByNameLax[data.FishInfo])
	if err != nil {
		return Profile, err
	}

	// their last seen bag
	rows, err = pool.Query(context.Background(),
		`SELECT bag, bot, chat, date
		FROM bag
		WHERE playerid = $1
		AND date = (
			SELECT MAX(date)
			FROM bag
			WHERE playerid = $1
			)`, playerID)
	if err != nil {
		return Profile, err
	}

	// checking errnorows because not everyone does +bag
	// also check for shiny in bag!
	Profile.Bag, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[data.FishInfo])
	if err != nil && err != pgx.ErrNoRows {
		return Profile, err
	} else if err == pgx.ErrNoRows {
		Profile.Bag.Bag = []string{"Player never did +bag!"}
	}

	Profile.BagCounts = make(map[string]int)
	for _, ItemInBag := range Profile.Bag.Bag {
		Profile.BagCounts[ItemInBag]++
	}

	// The count per year per chat
	rows, err = pool.Query(context.Background(), `
		select count(*) as int, 
		fishname as string,
		chat as string2,
		to_char(date_trunc('year', date), 'YYYY') as string3,
		catchtype as string5
		from fish 
		where playerid = $1
		group by string, string2, string3, string5
		order by int desc
		`,
		playerID)
	if err != nil {
		return Profile, err
	}

	FishTypesCaughtCountYearChat, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[Frick])
	if err != nil {
		return Profile, err
	}

	Profile.FishTypesCaughtCount = make(map[string]int)
	Profile.FishTypesCaughtCountChat = make(map[string]map[string]int)
	Profile.FishTypesCaughtCountYear = make(map[string]map[string]int)
	Profile.FishTypesCaughtCountYearChat = make(map[string]map[string]map[string]int)
	Profile.FishTypesCaughtCountYearChatCatchtype = make(map[string]map[string]map[string]map[string]int)

	for _, chatyear := range FishTypesCaughtCountYearChat {
		// Need to initialize all frickin maps, else nil panic)
		if Profile.FishTypesCaughtCountYearChatCatchtype[chatyear.String] == nil {
			Profile.FishTypesCaughtCountYearChatCatchtype[chatyear.String] = make(map[string]map[string]map[string]int)
		}

		if Profile.FishTypesCaughtCountYearChatCatchtype[chatyear.String][chatyear.String3] == nil {
			Profile.FishTypesCaughtCountYearChatCatchtype[chatyear.String][chatyear.String3] = make(map[string]map[string]int)
		}

		if Profile.FishTypesCaughtCountYearChatCatchtype[chatyear.String][chatyear.String3][chatyear.String2] == nil {
			Profile.FishTypesCaughtCountYearChatCatchtype[chatyear.String][chatyear.String3][chatyear.String2] = make(map[string]int)
		}

		Profile.FishTypesCaughtCountYearChatCatchtype[chatyear.String][chatyear.String3][chatyear.String2][chatyear.String5] = chatyear.Int

		if Profile.FishTypesCaughtCountYearChat[chatyear.String] == nil {
			Profile.FishTypesCaughtCountYearChat[chatyear.String] = make(map[string]map[string]int)
		}
		if Profile.FishTypesCaughtCountYearChat[chatyear.String][chatyear.String3] == nil {
			Profile.FishTypesCaughtCountYearChat[chatyear.String][chatyear.String3] = make(map[string]int)
		}
		Profile.FishTypesCaughtCountYearChat[chatyear.String][chatyear.String3][chatyear.String2] = chatyear.Int

		Profile.FishTypesCaughtCount[chatyear.String] = Profile.FishTypesCaughtCount[chatyear.String] + chatyear.Int

		if Profile.FishTypesCaughtCountChat[chatyear.String] == nil {
			Profile.FishTypesCaughtCountChat[chatyear.String] = make(map[string]int)
		}
		Profile.FishTypesCaughtCountChat[chatyear.String][chatyear.String2] = Profile.FishTypesCaughtCountChat[chatyear.String][chatyear.String2] + chatyear.Int

		if Profile.FishTypesCaughtCountYear[chatyear.String] == nil {
			Profile.FishTypesCaughtCountYear[chatyear.String] = make(map[string]int)
		}
		Profile.FishTypesCaughtCountYear[chatyear.String][chatyear.String3] = Profile.FishTypesCaughtCountYear[chatyear.String][chatyear.String3] + chatyear.Int
	}

	// all their fish seen; could get this from the fishtypescaughtcount maps
	// but this is also sorting them by name, so i dont need to sort them later
	err = pool.QueryRow(context.Background(),
		"select array_agg(distinct(fishname)) from fish where playerid = $1",
		playerID).Scan(&Profile.FishSeen)
	if err != nil {
		return Profile, err
	}

	// the fish they never caught
	err = pool.QueryRow(context.Background(),
		`select array_agg(fishname)
		from
		(
		select distinct(fishname) from fishinfo
		except
		select distinct(fishname) from fish where playerid = $1
		order by fishname asc)`, playerID).Scan(&Profile.FishNotSeen)
	if err != nil {
		return Profile, err
	}

	// The fishseen per chat; can get this from FishTypesCaughtCountChat
	rows, err = pool.Query(context.Background(), `
		select array_agg(distinct(fishname)) as string4,
		chat as string
		from fish 
		where playerid = $1
		group by string
		order by string4 asc
		`,
		playerID)
	if err != nil {
		return Profile, err
	}

	CountYear, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[Frick])
	if err != nil {
		return Profile, err
	}

	Profile.FishSeenChat = make(map[string][]string)

	for _, dada := range CountYear {
		Profile.FishSeenChat[dada.String] = dada.String4
	}

	// first biggest fish per type
	rows, err = pool.Query(context.Background(),
		`SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
			SELECT fishname, MAX(weight) AS max_weight
			FROM fish
			WHERE playerid = $1
			AND catchtype != 'release'
			AND catchtype != 'squirrel'
			GROUP BY fishname
		) AS sub
		ON f.fishname = sub.fishname AND f.weight = sub.max_weight
		WHERE f.playerid = $1
		ORDER BY date asc`, playerID)
	if err != nil {
		return Profile, err
	}

	BiggestFishPerType, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[data.FishInfo])
	if err != nil && err != pgx.ErrNoRows {
		return Profile, err
	}

	Profile.BiggestFishPerType = make(map[string]data.FishInfo)

	for _, fish := range BiggestFishPerType {
		Profile.BiggestFishPerType[fish.TypeName] = fish
	}

	// first smallest fish per type
	rows, err = pool.Query(context.Background(),
		`SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
			SELECT fishname, MIN(weight) AS min_weight
			FROM fish
			WHERE playerid = $1
			AND catchtype != 'release'
			AND catchtype != 'squirrel'
			GROUP BY fishname
		) AS sub
		ON f.fishname = sub.fishname AND f.weight = sub.min_weight
		WHERE f.playerid = $1
		ORDER BY date asc`, playerID)
	if err != nil {
		return Profile, err
	}

	SmallestFishPerType, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[data.FishInfo])
	if err != nil && err != pgx.ErrNoRows {
		return Profile, err
	}

	Profile.SmallestFishPerType = make(map[string]data.FishInfo)

	for _, fish := range SmallestFishPerType {
		Profile.SmallestFishPerType[fish.TypeName] = fish
	}

	// first time player caught a fish type
	rows, err = pool.Query(context.Background(),
		`SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
			SELECT fishname, MAX(date) AS max_date
			FROM fish
			WHERE playerid = $1
			GROUP BY fishname
		) AS sub
		ON f.fishname = sub.fishname AND f.date = sub.max_date
		WHERE f.playerid = $1
		ORDER BY date asc`, playerID)
	if err != nil {
		return Profile, err
	}

	LastCaughtFishPerType, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[data.FishInfo])
	if err != nil {
		return Profile, err
	}

	Profile.LastCaughtFishPerType = make(map[string]data.FishInfo)

	for _, fish := range LastCaughtFishPerType {
		Profile.LastCaughtFishPerType[fish.TypeName] = fish
	}

	// last time a player caught a fish type
	rows, err = pool.Query(context.Background(),
		`SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
			SELECT fishname, MIN(date) AS min_date
			FROM fish
			WHERE playerid = $1
			GROUP BY fishname
		) AS sub
		ON f.fishname = sub.fishname AND f.date = sub.min_date
		WHERE f.playerid = $1
		ORDER BY date asc`, playerID)
	if err != nil {
		return Profile, err
	}

	FirstCaughtFishPerType, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[data.FishInfo])
	if err != nil {
		return Profile, err
	}

	Profile.FirstCaughtFishPerType = make(map[string]data.FishInfo)

	for _, fish := range FirstCaughtFishPerType {
		Profile.FirstCaughtFishPerType[fish.TypeName] = fish
	}

	return Profile, nil
}

func PrintPlayerProfile(Profile PlayerProfile, EmojisForFish map[string]string) error {

	filePath := filepath.Join("leaderboards", "global", "players", fmt.Sprintf("%d", Profile.TwitchID.Int64)+".md")

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, _ = fmt.Fprintf(file, "# %s", Profile.Name)

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

	_, _ = fmt.Fprintln(file, "\n\nFish caught per year per chat")

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

	_, _ = fmt.Fprintln(file, "\n| --- | Catchtype | Count |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|")

	rank = 1

	sortedCatchtypes := sortMapString(Profile.CountCatchtype, "countdesc")

	for _, catch := range sortedCatchtypes {
		var catchtype string
		switch catch {
		default:
			catchtype = catch
		case "normal":
			catchtype = "Normal"
		case "egg":
			catchtype = "Eggs hatched"
		case "jumped":
			catchtype = "Jumped bonus"
		case "release":
			catchtype = "Release bonus"
		case "mouth":
			catchtype = "Mouth bonus"
		case "squirrel":
			catchtype = "Squirrels"
		case "squirrelfail":
			catchtype = "Squirrel fail" // add a description for this
		}
		_, _ = fmt.Fprintf(file, "\n| %d | %s | %d |",
			rank,
			catchtype,
			Profile.CountCatchtype[catch])
		rank++
	}

	_, _ = fmt.Fprintln(file, "\n## First, biggest and last fish") // Make it show catchtype ?

	_, _ = fmt.Fprintln(file, "\nFirst ever fish caught")

	_, _ = fmt.Fprintln(file, "\n| Fish | Weight in lbs | Date in UTC | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|")

	_, _ = fmt.Fprintf(file, "\n| %s %s | %.2f | %s | %s |",
		Profile.FirstFish.Type,
		Profile.FirstFish.TypeName,
		Profile.FirstFish.Weight,
		Profile.FirstFish.Date.Format("2006-01-02 15:04:05"),
		fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", Profile.FirstFish.Chat, Profile.FirstFish.Chat))

	_, _ = fmt.Fprintln(file, "\n\nTheir biggest fish caught")

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

	_, _ = fmt.Fprintln(file, "\n\nTheir last fish caught")

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

	// print one block for each fish type
	for _, fish := range Profile.FishSeen {

		_, _ = fmt.Fprintf(file, "\n| %s %s | Total caught | %d |",
			EmojisForFish[fish],
			fish,
			Profile.FishTypesCaughtCount[fish])

		_, _ = fmt.Fprintln(file, "\n|-------|-------|-------|")

		_, _ = fmt.Fprintf(file, "\n| %s | Year | Count | Chat|\n", EmojisForFish[fish])

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

		_, _ = fmt.Fprintf(file, "\n| %s | Weight in lbs | Date in UTC | Chat |\n", EmojisForFish[fish])

		_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|")

		_, _ = fmt.Fprintf(file, "\n| First caught | %.2f | %s | %s |",
			Profile.FirstCaughtFishPerType[fish].Weight,
			Profile.FirstCaughtFishPerType[fish].Date.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)",
				Profile.FirstCaughtFishPerType[fish].Chat,
				Profile.FirstCaughtFishPerType[fish].Chat))

		_, _ = fmt.Fprintf(file, "\n| Last caught | %.2f | %s | %s |",
			Profile.LastCaughtFishPerType[fish].Weight,
			Profile.LastCaughtFishPerType[fish].Date.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)",
				Profile.LastCaughtFishPerType[fish].Chat,
				Profile.LastCaughtFishPerType[fish].Chat))

		_, _ = fmt.Fprintf(file, "\n| Biggest | %.2f | %s | %s |",
			Profile.BiggestFishPerType[fish].Weight,
			Profile.BiggestFishPerType[fish].Date.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)",
				Profile.BiggestFishPerType[fish].Chat,
				Profile.BiggestFishPerType[fish].Chat))

		_, _ = fmt.Fprintf(file, "\n| Smallest | %.2f | %s | %s |",
			Profile.SmallestFishPerType[fish].Weight,
			Profile.SmallestFishPerType[fish].Date.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)",
				Profile.SmallestFishPerType[fish].Chat,
				Profile.SmallestFishPerType[fish].Chat))

		_, _ = fmt.Fprintln(file)
	}

	// show what fish they never caught
	_, _ = fmt.Fprintln(file, "\n## Fish they have never seen")

	for _, fish := range Profile.FishNotSeen {

		_, _ = fmt.Fprintf(file, "\n* %s %s", EmojisForFish[fish], fish)

	}

	_, _ = fmt.Fprintf(file, "\n\nIn total %d fish never seen", len(Profile.FishNotSeen))

	// add something to say if they caught all the treasures

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
