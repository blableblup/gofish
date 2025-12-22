package leaderboards

import (
	"context"
	"database/sql"
	"fmt"
	"gofish/logs"
	"gofish/utils"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/jackc/pgx/v5"
)

type Wrapped struct {
	Year string `json:"Year"`

	Name     string       `json:"Name"`
	PlayerID int          `json:"-"`
	TwitchID int          `json:"-"`
	Verified sql.NullBool `json:"-"`

	BiggestFish        []ProfileFish
	MostCaughtFish     []CaughtFish
	RarestFish         []RareFish
	FishSeen           []string `json:"-"`
	FishSeenCount      int
	FishSeenPercentile float64
	Count              *TotalChatStructPercentile
	FishLocations      []Location
}

type TotalChatStructPercentile struct {
	Total      int
	Percentile float64
	ChatCounts map[string]*TotalChatStructPercentile `json:"ChatCounts,omitempty"`
}

type CaughtFish struct {
	Fish  string
	Count int
}

type RareFish struct {
	Fish         string
	CountYear    int
	CountAllTime int
}

type Location struct {
	Location   string
	Percentage float64
}

func GetWrapped(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	limit := params.Limit

	// anyone who caught more than 1 fish gets their wrapped
	// unless specified else
	var countlimit int
	var err error
	if limit == "" {
		countlimit = 1
	} else {
		countlimit, err = strconv.Atoi(limit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Limit", limit).
				Msg("Error converting limit to int")
			return
		}
	}

	validPlayers, err := GetValidPlayersWrapped(params, countlimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error getting valid players")
		return
	}

	allFishes, err := GetAllFishNames(params)
	if err != nil {
		return
	}

	FishWithEmoji := make(map[string]string)

	for _, fish := range allFishes {
		FishWithEmoji[fish], err = FishStuff(fish, params)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Str("Fish", fish).
				Msg("Error getting fish emoji")
			return
		}
	}

	// get the year from date param
	date := params.Date

	datetime, err := utils.ParseDate(date)
	if err != nil {
		return
	}

	year := fmt.Sprintf("%d", datetime.Year())

	Wrappeds, err := GetTheWrappeds(params, FishWithEmoji, validPlayers, year)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error getting wrappeds")
		return
	}

	var twitchIDsWrappeds []int

	wg := new(sync.WaitGroup)
	mu := new(sync.Mutex)

	for _, validPlayer := range validPlayers {
		wg.Go(func() {

			if Wrappeds[validPlayer].TwitchID != 0 {
				err = PrintWrapped(Wrappeds[validPlayer], FishWithEmoji)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Chat", chatName).
						Str("Board", board).
						Int("PlayerID", validPlayer).
						Msg("Error printing wrapped")
				} else {
					mu.Lock()
					twitchIDsWrappeds = append(twitchIDsWrappeds, Wrappeds[validPlayer].TwitchID)
					mu.Unlock()
				}
			}
		})
	}

	wg.Wait()

	err = writeRaw(filepath.Join("leaderboards", "global", "wrappeds", year, "twitchIDsList"), twitchIDsWrappeds)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error writing twitchIDsList file")
		return
	}

	logs.Logs().Info().
		Str("Chat", chatName).
		Str("Board", board).
		Str("Date", params.Date).
		Str("Date2", params.Date2).
		Msg("Done updating wrappeds")

}

func GetValidPlayersWrapped(params LeaderboardParams, limit int) ([]int, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	date2 := params.Date2
	date := params.Date
	pool := params.Pool

	var validPlayers []int
	var rows pgx.Rows
	var err error

	query := `
		select f.playerid from fish f
		join 
		(
		select playerid, count(*) from fish
		where date < $2
		and date >= $3
		group by playerid
		) bla on bla.playerid = f.playerid
		group by f.playerid, bla.count
		having bla.count >= $1`

	rows, err = pool.Query(context.Background(), query, limit, date, date2)
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
