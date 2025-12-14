package leaderboards

import (
	"context"
	"database/sql"
	"gofish/logs"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// for the profiles
func ReturnFishSliceQueryValidPlayers(params LeaderboardParams, query string, validPlayers []int) ([]ProfileFish, error) {
	date2 := params.Date2
	date := params.Date
	pool := params.Pool

	rows, err := pool.Query(context.Background(), query, validPlayers, date, date2)
	if err != nil {
		return []ProfileFish{}, err
	}

	fishy, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[ProfileFish])
	if err != nil && err != pgx.ErrNoRows {
		return []ProfileFish{}, err
	}

	return fishy, nil
}

// for some leaderboards
func ReturnFishSliceQuery(params LeaderboardParams, query string, chat bool) ([]BoardData, error) {
	chatName := params.ChatName
	date2 := params.Date2
	date := params.Date
	pool := params.Pool

	var rows pgx.Rows
	var err error

	if chat {
		rows, err = pool.Query(context.Background(), query, chatName, date, date2)
		if err != nil {
			return []BoardData{}, err
		}

	} else {
		rows, err = pool.Query(context.Background(), query, date, date2)
		if err != nil {
			return []BoardData{}, err
		}
	}

	fishy, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[BoardData])
	if err != nil && err != pgx.ErrNoRows {
		return []BoardData{}, err
	}

	return fishy, nil
}

// Store the players in a map, for their verified status, their current name and when they started fishing
// useful when updating all the leaderboards at once; firstfishdate is ignored on some boards where you already get date
// the first fishdate only matters if the player fished on supi, would be better to get min(date) from fish for the player
// firstfishdate is set for when the player first was added and is never updated afterwards,
// so the player might have fished earlier in a chat which wasnt covered for example or during a downtime
// and also, firstfishdate could also be when the player first did + bag
func PlayerStuff(playerID int, params LeaderboardParams, pool *pgxpool.Pool) (string, time.Time, bool, int, error) {

	var name string
	var firstfishdate time.Time
	var verified sql.NullBool
	var twitchID sql.NullInt64

	if _, ok := params.Players[playerID]; !ok {
		err := pool.QueryRow(context.Background(),
			"SELECT name, firstfishdate, verified, twitchid FROM playerdata WHERE playerid = $1",
			playerID).Scan(&name, &firstfishdate, &verified, &twitchID)
		if err != nil {
			logs.Logs().Error().Err(err).
				Int("PlayerID", playerID).
				Str("Chat", params.ChatName).
				Str("Board", params.LeaderboardType).
				Msg("Error retrieving player name for id")
			return name, firstfishdate, verified.Bool, 0, err
		}

		if twitchID.Valid {
			params.Players[playerID] = PlayerInfo{
				CurrentName: name,
				Date:        firstfishdate,
				Verified:    verified.Bool,
				TwitchID:    int(twitchID.Int64),
			}
		} else {
			params.Players[playerID] = PlayerInfo{
				CurrentName: name,
				Date:        firstfishdate,
				Verified:    verified.Bool,
				TwitchID:    0,
			}
		}
	}

	return params.Players[playerID].CurrentName, params.Players[playerID].Date, params.Players[playerID].Verified, params.Players[playerID].TwitchID, nil
}

// because some fish had different emotes on supibot, i always get the latest emoji from fishinfo
func FishStuff(fishName string, params LeaderboardParams) (string, error) {
	pool := params.Pool

	var emoji string

	if _, ok := params.FishTypes[fishName]; !ok {
		err := pool.QueryRow(context.Background(), "SELECT fishtype FROM fishinfo WHERE fishname = $1", fishName).Scan(&emoji)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("FishName", fishName).
				Str("Chat", params.ChatName).
				Str("Board", params.LeaderboardType).
				Msg("Error retrieving fish type for fish name")
			return emoji, err
		}

		params.FishTypes[fishName] = emoji

	} else {
		emoji = params.FishTypes[fishName]
	}

	return emoji, nil
}

// get all the fish from the db
func GetAllFishNames(params LeaderboardParams) ([]string, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	date2 := params.Date2
	date := params.Date
	pool := params.Pool

	var fishes []string

	err := pool.QueryRow(context.Background(), `
	select array_agg(distinct fishname) 
	from fish
	where date < $1
	and date > $2`, date, date2).Scan(&fishes)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error querying database for all fish")
		return fishes, err
	}

	return fishes, nil
}

// return fish from fishinfo with a specific tag
func ReturnFishTags(params LeaderboardParams, tag string) ([]string, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	pool := params.Pool

	var fish []string

	err := pool.QueryRow(context.Background(), `select array_agg(fishname) from fishinfo where $1 = any(tags)`, tag).Scan(&fish)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Str("Tag", tag).
			Msg("Error querying database for tag")
		return fish, err
	}

	if len(fish) == 0 {
		logs.Logs().Warn().
			Str("Chat", chatName).
			Str("Board", board).
			Str("Tag", tag).
			Msg("No fish with that tag in db!")
	}

	return fish, nil
}

func GetAllShinies(params LeaderboardParams) ([]string, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	pool := params.Pool

	var shinies []string

	rows, err := pool.Query(context.Background(), `
		select shiny from fishinfo where shiny != '{}'`)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error querying database")
		return shinies, err
	}
	defer rows.Close()

	for rows.Next() {

		// maybe bread will add multiple shinies for same fish at one point
		// thats why shiny is an array in the fishinfo table
		var shiny []string

		if err := rows.Scan(&shiny); err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error scanning row")
			return shinies, err
		}

		shinies = append(shinies, shiny...)
	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error iterating over rows")
		return shinies, err
	}

	return shinies, nil
}

func CatchtypeNames(pool *pgxpool.Pool) (map[string]string, error) {

	CatchtypeNames := make(map[string]string)

	rows, err := pool.Query(context.Background(), `
		select distinct(catchtype) from fish`)
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error querying database")
		return map[string]string{}, err
	}
	defer rows.Close()

	for rows.Next() {

		var catchtype string

		if err := rows.Scan(&catchtype); err != nil {
			logs.Logs().Error().Err(err).
				Msg("Error scanning row")
			return map[string]string{}, err
		}

		switch catchtype {
		default:
			logs.Logs().Error().
				Str("Catchtype", catchtype).
				Msg("NO NAME FOR CATCHTYPE!!!!")

		case "normal":
			CatchtypeNames[catchtype] = "Normal"

		case "egg":
			CatchtypeNames[catchtype] = "Hatched egg"

		case "release":
			CatchtypeNames[catchtype] = "Release bonus"

		case "releasepumpkin":
			CatchtypeNames[catchtype] = "Pumpkin release bonus"

		case "jumped":
			CatchtypeNames[catchtype] = "Jumped bonus"

		case "mouth":
			CatchtypeNames[catchtype] = "Mouth bonus"

		case "squirrel":
			CatchtypeNames[catchtype] = "Squirrel"

		case "squirrelfail":
			CatchtypeNames[catchtype] = "Squirrel fail"
			// the squirrels i added manually because bread forgor to update game and you werent supposed to catch them

		case "sonnythrow":
			CatchtypeNames[catchtype] = "Sonny throw"

		case "giftbell":
			CatchtypeNames[catchtype] = "Bell gift"

		case "giftwinter2024":
			CatchtypeNames[catchtype] = "Winter present (2024)"

		}
	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error iterating over rows")
		return map[string]string{}, err
	}

	return CatchtypeNames, nil
}
