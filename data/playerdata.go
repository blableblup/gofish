package data

import (
	"context"
	"database/sql"
	"gofish/logs"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PlayerData struct {
	PlayerID int
	TwitchID int

	confirmedDates []Dates
	// this is array, because a player can use one of their names multiple times
	// but since confirmedPlayers is map[string][]PlayerData can also just append player again ?
	// i would need to range over the players and then append dates to that idk
}

type Dates struct {
	HighestDate time.Time
	LowestDate  time.Time
}

// all the data about the player in my playerdata table
type PlayerDataInDB struct {
	PlayerID int
	TwitchID int
	Name     string
	OldNames []string
}

func ConfirmWhoIsWho(fishes []FishInfo, pool *pgxpool.Pool) (map[string][]PlayerData, error) {

	logs.Logs().Info().Msg("Going over all the players for playerids.....")

	confirmedPlayers := make(map[string][]PlayerData)

	// playersfishingdate should include the urls with the dates
	// so that i now which logs page to open for which day for the raw page
	// so should be map[string]map[string][]Dates
	playersFishingDate, firstFishChats, err := GetAllThePlayerNamesAndWhenTheyFished(fishes, pool)
	if err != nil {
		return confirmedPlayers, err
	}

	// to not get rate limited from the thingy
	var playerCount int
	pause := time.Second * 60

	for player := range playersFishingDate {

		playerCount++
		if playerCount == 200 {
			playerCount = 0
			logs.Logs().Info().
				Msg("Pausing going over the players ....")
			time.Sleep(pause)
		}

		// the player has more than 1 dates, if there is a 6 month break in the fish data for them
		// and now need to find out if the name was used by other players before
		if len(playersFishingDate[player]) > 1 {
			logs.Logs().Warn().
				Str("Player", player).
				Interface("Dates", playersFishingDate[player]).
				Msg("Player name has more than 1 dates!")
		}

		for _, dates := range playersFishingDate[player] {

			// check now if highest date in the checked data is more than 6 months away
			months, years, err := DiffBetweenTwoDates(time.Now(), dates.HighestDate, pool)
			if err != nil {
				return confirmedPlayers, err
			}

			var twitchID int
			var playerInApi bool
			var userData []map[string]any

			if months >= 6 || years != 0 {

				// if it is more than 6 months away
				// check raw logs page

				// also: need to not rename a player again for their old names

				// ...

			} else {

				twitchID, userData, playerInApi, err = PlayerRecent(player, dates, pool)
				if err != nil {
					return confirmedPlayers, err
				}
			}

			// now check if there is already a player in the db with that twitchid

			DBData, TwitchIDExists, err := CheckForTwitchIDInDB(twitchID, pool)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Player", player).
					Msg("Error checking twitchID in DB for player")
				return confirmedPlayers, err
			}

			var playerID int

			if TwitchIDExists {

				if playerInApi {
					currentName := GetCurrentName(userData)

					if currentName != DBData.Name {

						err = RenamePlayer(currentName, DBData.Name, twitchID, DBData.PlayerID, pool)
						if err != nil {
							logs.Logs().Error().Err(err).
								Str("Player", player).
								Int("twitchID", twitchID).
								Msg("Error renaming player")
							return confirmedPlayers, err
						}
					}
				} else {
					// the player name could be an old name
					// since im now checking the ambience
					// someone could have not caught a fish with their old name and renamed
					// when checking old logs
					// ...
				}

				playerID = DBData.PlayerID

			} else {

				firstFishDate := dates.LowestDate

				// also: this firstfishdate wont be the exact date of their first fish
				// it will be the year month day witouth the hh:mm:ss
				// but idk im not using that for anything

				playerID, err = AddNewPlayer(twitchID, player, firstFishDate, firstFishChats[player], pool)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Player", player).
						Msg("Error adding new player to DB")
					return confirmedPlayers, err
				}
			}

			// add/remove 6 months from the highest / lowest date
			// tis is 7 * 24 * 4 * 6 idk 4032 hours
			datesEdited := Dates{
				HighestDate: dates.HighestDate.Add(time.Hour * 4032),
				LowestDate:  dates.LowestDate.Add(time.Hour * -4032),
			}

			dataPlayer := PlayerData{
				TwitchID:       twitchID,
				PlayerID:       playerID,
				confirmedDates: []Dates{datesEdited},
			}

			confirmedPlayers[player] = append(confirmedPlayers[player], dataPlayer)

		}
	}

	logs.Logs().Info().
		Int("Amount of players", len(confirmedPlayers)).
		Msg("Finished..")

	return confirmedPlayers, nil
}

func PlayerRecent(player string, dates Dates, pool *pgxpool.Pool) (int, []map[string]any, bool, error) {

	var twitchID int
	var playerInApi bool

	userdata, err := MakeApiRequestForPlayerToApiIVR(player, 0, "name")
	if err != nil && err != ErrNoPlayerFound {

		logs.Logs().Error().Err(err).
			Str("Player", player).
			Msg("Error doing api ivr thing")
		return 0, userdata, false, err

	} else if err == ErrNoPlayerFound {

		// if there is no player in the api currently using that name
		// check if the last time someone used that name in the db was more than 6 months ago

		// check for players who have that name as current name first
		playersNameDB, err := GetPlayersForPlayerName(player, pool)
		if err != nil {
			return 0, userdata, false, err
		}

		var foundAPlayer bool

		for _, playerName := range playersNameDB {

			lastSeen, _, err := PlayerDates(pool, playerName.PlayerID, player)
			if err != nil {
				return 0, userdata, false, err
			}

			months, years, err := DiffBetweenTwoDates(time.Now(), lastSeen, pool)
			if err != nil {
				return 0, userdata, false, err
			}

			if months < 6 && years == 0 {
				// if its not more than 6 months ago, it has to be this player
				twitchID = playerName.TwitchID
				foundAPlayer = true
				break
			}
		}

		if !foundAPlayer {
			// if no player found, check for players who have that name as an oldname
			playersOldNameDB, err := GetPlayersForOldName(player, pool)
			if err != nil {
				return 0, userdata, false, err
			}

			for _, oldPlayer := range playersOldNameDB {

				lastSeen, _, err := PlayerDates(pool, oldPlayer.PlayerID, player)
				if err != nil {
					return 0, userdata, false, err
				}

				months, years, err := DiffBetweenTwoDates(time.Now(), lastSeen, pool)
				if err != nil {
					return 0, userdata, false, err
				}

				if months < 6 && years == 0 {

					twitchID = oldPlayer.TwitchID
					break
				}
			}
		}

	} else if err == nil {

		// use twitchid of the player from the api if there is one
		twitchID, err = GetTwitchID(userdata)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error getting twitchID for player")
			return 0, userdata, false, err
		}
		playerInApi = true
	}

	if twitchID == 0 {
		// if there is still noone found for that name
		// check raw logs page
		logs.Logs().Error().
			Str("Player", player).
			Msg("No player found for recent player!")

		// ...
	}

	return twitchID, userdata, playerInApi, nil
}

func CheckForTwitchIDInDB(twitchID int, pool *pgxpool.Pool) (PlayerDataInDB, bool, error) {

	var DBData PlayerDataInDB

	err := pool.QueryRow(context.Background(),
		"SELECT name, playerid, twitchid, oldnames FROM playerdata WHERE twitchid = $1",
		twitchID).Scan(&DBData.Name, &DBData.PlayerID, &DBData.TwitchID, &DBData.OldNames)

	if err != nil {
		if err == pgx.ErrNoRows {
			return DBData, false, nil
		} else {
			return DBData, false, err
		}
	}

	return DBData, true, nil
}

func GetPlayersForPlayerName(player string, pool *pgxpool.Pool) ([]PlayerDataInDB, error) {

	var players []PlayerDataInDB

	rows, err := pool.Query(context.Background(),
		"SELECT name, playerid, twitchid, oldnames FROM playerdata WHERE name = $1",
		player)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Player", player).
			Msg("Error quering for player name in playerdata")
		return players, err
	}
	defer rows.Close()

	for rows.Next() {

		var DBData PlayerDataInDB

		if err := rows.Scan(&DBData.Name, &DBData.PlayerID, &DBData.TwitchID, &DBData.OldNames); err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error scanning row for player")
			return players, err
		}

		players = append(players, DBData)

	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error iterating over rows")
		return players, err
	}

	return players, nil
}

func GetPlayersForOldName(player string, pool *pgxpool.Pool) ([]PlayerDataInDB, error) {

	var players []PlayerDataInDB

	rows, err := pool.Query(context.Background(),
		"SELECT name, playerid, twitchid, oldnames FROM playerdata WHERE $1 = any(oldnames)",
		player)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Player", player).
			Msg("Error quering for player name in playerdata")
		return players, err
	}
	defer rows.Close()

	for rows.Next() {

		var DBData PlayerDataInDB

		if err := rows.Scan(&DBData.Name, &DBData.PlayerID, &DBData.TwitchID, &DBData.OldNames); err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error scanning row for player")
			return players, err
		}

		players = append(players, DBData)

	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error iterating over rows")
		return players, err
	}

	return players, nil
}

func PlayerDates(pool *pgxpool.Pool, playerID int, player string) (time.Time, time.Time, error) {

	// this can be weird, if the name was used by the persone multiple times
	// and someone else used that name between the other person
	// their time will overlap then
	var lastseen, firstseen sql.NullTime

	err := pool.QueryRow(context.Background(),
		`select max(max), min(min)
		from (
		select max(date), min(date) from ambience where playerid = $1 and player = $2
		union all
		select max(date), min(date) from bag where playerid = $1 and player = $2
		union all
		select max(date), min(date) from fish where playerid = $1 and player = $2
		) as all_dates
		`,
		playerID, player).Scan(&lastseen, &firstseen)
	if err != nil {
		logs.Logs().Error().Err(err).
			Int("PlayerID", playerID).
			Str("Player", player).
			Msg("Error querying DB for last and firstseen!!")
		return time.Time{}, time.Time{}, err
	}

	if !lastseen.Valid {
		logs.Logs().Error().
			Str("Player", player).
			Int("PlayerID", playerID).
			Msg("Cant find valid lastseen and firstseen for player!!!!")
		return time.Time{}, time.Time{}, nil
	}

	return lastseen.Time, firstseen.Time, nil
}

func GetAllThePlayerNamesAndWhenTheyFished(fishes []FishInfo, pool *pgxpool.Pool) (map[string][]Dates, map[string]string, error) {

	playersFishingDate := make(map[string][]Dates)
	playersFishingFish := make(map[string][]FishInfo)
	firstFishChats := make(map[string]string)
	var err error

	// first add the player to the map and get the fish that name caught
	for _, fish := range fishes {

		// the fish are sorted by date ascending, so this will be first fish chat
		// if the player name was used by different players in the checked data this can be wrong
		// if they are new
		if _, ok := firstFishChats[fish.Player]; !ok {
			firstFishChats[fish.Player] = fish.Chat
		}

		playersFishingFish[fish.Player] = append(playersFishingFish[fish.Player], fish)
	}

	for player := range playersFishingFish {

		playersFishingDate[player], err = AllTheDaysAPlayerFished(playersFishingFish[player], pool)
		if err != nil {
			return playersFishingDate, firstFishChats, err
		}
	}

	return playersFishingDate, firstFishChats, nil
}

func AllTheDaysAPlayerFished(fishes []FishInfo, pool *pgxpool.Pool) ([]Dates, error) {

	var days []Dates

	oneday := (time.Hour * 24)

	var highestDay, lowestDay, lastDay time.Time
	var resetAtleastOnce bool

	for _, fish := range fishes {

		day := fish.Date.Truncate(oneday)
		lastDay = day

		if len(days) == 0 {
			highestDay = day
			lowestDay = day
		} else {
			if day.Before(lowestDay) {
				lowestDay = day
			}
			if day.After(highestDay) {
				highestDay = day
			}
		}

		// if diff above 6 months; this could be a different player using that name
		months, years, err := DiffBetweenTwoDates(day, lastDay, pool)
		if err != nil {
			return days, err
		}

		if months > 6 || years > 0 {

			resetAtleastOnce = true

			newDays := Dates{
				HighestDate: highestDay,
				LowestDate:  lowestDay,
			}

			days = append(days, newDays)

			// reset the thing idk
			highestDay, lowestDay = day, day
		}

	}

	// if the player never stopped fishing for more than 6 months in the checked data
	// or if the thing was reset
	if len(days) == 0 || resetAtleastOnce {
		newDays := Dates{
			HighestDate: highestDay,
			LowestDate:  lowestDay,
		}

		days = append(days, newDays)
	}

	return days, nil
}

// years and months is positive if day / $1 is the higher day
func DiffBetweenTwoDates(day time.Time, day2 time.Time, pool *pgxpool.Pool) (int, int, error) {

	var months, years int

	err := pool.QueryRow(context.Background(),
		"select date_part('month', age($1, $2)), date_part('year', age($1, $2))",
		day, day2).Scan(&months, &years)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Date2", day2.Format("2006-01-2 15:04:05")).
			Str("Date1", day.Format("2006-01-2 15:04:05")).
			Msg("Error getting month difference for dates")
		return months, years, err
	}

	return months, years, nil

}

func AddNewPlayer(twitchid int, player string, firstFishDate time.Time, firstFishChat string, pool *pgxpool.Pool) (int, error) {

	// Add a new player and return their id
	// If a players twitchid cannot be found in the api, twitchid is null
	var playerID int
	if twitchid == 0 {
		err := pool.QueryRow(context.Background(), "INSERT INTO playerdata (name,  firstfishdate, firstfishchat) VALUES ($1, $2, $3) RETURNING playerid", player, firstFishDate, firstFishChat).Scan(&playerID)
		if err != nil {
			return 0, err
		}
		logs.Logs().Warn().
			Str("Date", firstFishDate.Format(time.RFC3339)).
			Str("Chat", firstFishChat).
			Str("TwitchID", "no TwitchID found").
			Str("Player", player).
			Int("PlayerID", playerID).
			Msg("Added new player to playerdata")
	} else {
		err := pool.QueryRow(context.Background(), "INSERT INTO playerdata (name, twitchid, firstfishdate, firstfishchat) VALUES ($1, $2, $3, $4) RETURNING playerid", player, twitchid, firstFishDate, firstFishChat).Scan(&playerID)
		if err != nil {
			return 0, err
		}
		logs.Logs().Info().
			Str("Date", firstFishDate.Format(time.RFC3339)).
			Str("Chat", firstFishChat).
			Int("TwitchID", twitchid).
			Str("Player", player).
			Int("PlayerID", playerID).
			Msg("Added new player to playerdata")
	}

	return playerID, nil
}

func RenamePlayer(newName string, oldName string, twitchid int, playerid int, pool *pgxpool.Pool) error {

	// Update the player in playerdata
	_, err := pool.Exec(context.Background(), `
			UPDATE playerdata
			SET name = $1, oldnames = array_append(oldnames, $2)
			WHERE twitchid = $3		
			`, newName, oldName, twitchid)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("OldName", oldName).
			Str("NewName", newName).
			Int("TwitchID", twitchid).
			Int("PlayerID", playerid).
			Msg("Error updating player data for name")
		return err
	}

	logs.Logs().Info().
		Str("OldName", oldName).
		Str("NewName", newName).
		Int("TwitchID", twitchid).
		Int("PlayerID", playerid).
		Msg("Renamed player")

	return nil
}
