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
	PlayerID int
	TwitchID sql.NullInt64
	Name     string

	Count          int
	CountYear      map[string]int
	ChatCounts     map[string]int
	ChatCountsYear map[string]map[string]int

	BiggestFish []data.FishInfo
	LastFish    []data.FishInfo
	FirstFish   data.FishInfo

	FishSeen             []string
	FishTypesCaughtCount map[string]int
	BiggestFishPerType   map[string]data.FishInfo
	SmallestFishPerType  map[string]data.FishInfo

	Bag       data.FishInfo
	BagCounts map[string]int
}

func GetPlayerProfiles(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	config := params.Config
	limit := params.Limit

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

	g := new(errgroup.Group)

	// Get the players profile and print it for each player
	for _, validPlayer := range validPlayers {
		// "The first call to return a non-nil error cancels the group's context, if the group was created by calling WithContext."
		// context ?
		g.Go(func() error {
			playerProfile, err := GetAPlayerProfile(params, validPlayer)
			if err == nil {
				err = PrintPlayerProfile(playerProfile)
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

func GetAPlayerProfile(params LeaderboardParams, playerID int) (PlayerProfile, error) {
	pool := params.Pool

	var Profile PlayerProfile

	Profile.PlayerID = playerID

	// Get the data for each player

	// add date and date 2 to the queries so that they are like the leaderboards
	// so that they only count the data for the tournament week ? or

	// For this I already have PlayerStuff function in leaderboardutils ?
	// add twitchid to that function or?
	err := pool.QueryRow(context.Background(),
		"select name, twitchid from playerdata where playerid = $1",
		playerID).Scan(&Profile.Name, &Profile.TwitchID)
	if err != nil {
		return Profile, err
	}

	if !Profile.TwitchID.Valid {
		logs.Logs().Error().
			Str("Player", Profile.Name).
			Int("PlayerID", Profile.PlayerID).
			Msg("Player does not have a twitchID in the DB!")
	}

	err = pool.QueryRow(context.Background(),
		"select count(*), array_agg(distinct(fishname)) from fish where playerid = $1",
		playerID).Scan(&Profile.Count, &Profile.FishSeen)
	if err != nil {
		return Profile, err
	}

	// A players first ever fish
	rows, err := pool.Query(context.Background(),
		`SELECT weight, fishtype as type, fishname as typename, bot, chat, date, catchtype, fishid, chatid
		FROM fish 
		WHERE playerid = $1
		AND date = (
			SELECT MIN(date)
			FROM fish
			WHERE playerid = $1
			)`, playerID)
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

	// their 10 last fish
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

	// ...

	return Profile, nil
}

func PrintPlayerProfile(Profile PlayerProfile) error {

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

	_, _ = fmt.Fprintf(file, "\n* Fish Caught %d", Profile.Count)

	_, _ = fmt.Fprintln(file, "\n## Data for fishes")

	_, _ = fmt.Fprintln(file, "\nFirst ever fish caught")

	_, _ = fmt.Fprintln(file, "\n| --- | Fish | Weight in lbs | Date in UTC | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|-------|")

	_, _ = fmt.Fprintf(file, "\n| --- | %s %s | %.2f | %s | %s |",
		Profile.FirstFish.Type,
		Profile.FirstFish.TypeName,
		Profile.FirstFish.Weight,
		Profile.FirstFish.Date.Format("2006-01-02 15:04:05"),
		fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", Profile.FirstFish.Chat, Profile.FirstFish.Chat))

	_, _ = fmt.Fprintln(file, "\n\nTheir biggest fish caught")

	_, _ = fmt.Fprintln(file, "\n| Rank | Fish | Weight in lbs | Date in UTC | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|-------|")

	rank := 1

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

	_, _ = fmt.Fprintln(file, "\n| Rank | Fish | Weight in lbs | Date in UTC | Chat |")

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

	_, _ = fmt.Fprintln(file, "\n## Data about their fish seen")

	// ...

	_, _ = fmt.Fprintln(file, "\n## Their last seen bag")

	_, _ = fmt.Fprintln(file, "\n| Bag | Date in UTC | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|")

	_, _ = fmt.Fprintf(file, "\n| %s | %s | %s |",
		Profile.Bag.Bag,
		Profile.Bag.Date.Format("2006-01-02 15:04:05"),
		fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", Profile.Bag.Chat, Profile.Bag.Chat))

	return nil
}
