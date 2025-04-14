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

	TenBiggestFish []data.FishInfo
	LastTenFish    []data.FishInfo
	FirstFish      data.FishInfo

	FishSeen             []string
	FishTypesCaughtCount map[string]int
	BiggestFishPerType   map[string]data.FishInfo
	SmallestFishPerType  map[string]data.FishInfo
	// what else
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
	board := params.LeaderboardType
	chatName := params.ChatName
	pool := params.Pool

	var Profile PlayerProfile

	Profile.PlayerID = playerID

	// Get the data for each player

	// For this I already have PlayerStuff function in leaderboardutils ?
	// add twitchid to that function or?
	rows, err := pool.Query(context.Background(), `
	select name, twitchid from playerdata
	where playerid = $1`, playerID)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error querying database")
		return Profile, err
	}
	defer rows.Close()

	for rows.Next() {

		if err := rows.Scan(&Profile.Name, &Profile.TwitchID); err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error scanning row")
			return Profile, err
		}

		if !Profile.TwitchID.Valid {
			logs.Logs().Error().
				Str("Player", Profile.Name).
				Int("PlayerID", Profile.PlayerID).
				Msg("Player does not have a twitchID in the DB!")
		}

	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error iterating over rows")
		return Profile, err
	}

	rows, err = pool.Query(context.Background(), `
		select count(*), array_agg(distinct(fishname)) from fish
		where playerid = $1`, playerID)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error querying database")
		return Profile, err
	}
	defer rows.Close()

	for rows.Next() {

		if err := rows.Scan(&Profile.Count, &Profile.FishSeen); err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error scanning row")
			return Profile, err
		}

	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error iterating over rows")
		return Profile, err
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

	_, err = fmt.Fprintf(file, "# Player: %s, TwitchID: %d", Profile.Name, Profile.TwitchID.Int64)
	if err != nil {
		return err
	}

	return nil
}
