package data

import (
	"context"
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

	confirmedPlayers := make(map[string][]PlayerData)

	playersFishingDate, err := GetAllThePlayerNamesAndWhenTheyFished(fishes, pool)
	if err != nil {
		return confirmedPlayers, err
	}

	for player := range playersFishingDate {

		if len(playersFishingDate[player]) > 1 {
			logs.Logs().Warn().
				Str("Player", player).
				Interface("Dates", playersFishingDate[player]).
				Msg("PLAYer has more than 1 dates!!!!!")
			// idk
		}

		// range over the dates here idk

		// try to get their twitchID
		twitchID, err := GetTwitchID(player)
		if err != nil && err != ErrNoPlayerFound {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error getting twitchID for player")
			return confirmedPlayers, err
		} else if err == ErrNoPlayerFound {
			logs.Logs().Warn().
				Str("Player", player).
				Msg("Cannot get twitchID for player!!!!!")
			// check the logs page manually idk
			// like this : https://logs.joinuv.com/channel/breadworms/2025/10/26/?json=true
		}

		// check if there is already a player with that id in db
		// if yes, check name to see if need to rename
		// if no, add player
		var DBData PlayerDataInDB
		var TwitchIDExists bool

		DBData, TwitchIDExists, err = CheckForTwitchIDInDB(twitchID, pool)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error checking twitchID in DB for player")
			return confirmedPlayers, err
		}

		var playerID int

		if TwitchIDExists {

			var currentName string

			currentName, err = GetCurrentName(twitchID)
			if err != nil {
				logs.Logs().Error().Err(err).
					Int("twitchID", twitchID).
					Msg("Error getting current name for twitchID")
				return confirmedPlayers, err
			}

			if currentName != DBData.Name {

				// this would now always rename a player when checking the logs (in mode "a")
				// even when they havent fished or did +bag

				// before this i was only renaming the player if they actually fished
				err = RenamePlayer(currentName, DBData.Name, twitchID, DBData.PlayerID, pool)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Player", player).
						Int("twitchID", twitchID).
						Msg("Error renaming player")
					return confirmedPlayers, err
				}
			}

			playerID = DBData.PlayerID

		} else {

			var firstFishDate time.Time
			// var firstFishChat string
			// ?

			// idk if this is more than 1 dates
			for _, dates := range playersFishingDate[player] {
				firstFishDate = dates.LowestDate
			}

			playerID, err = AddNewPlayer(twitchID, player, firstFishDate, "", pool)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Player", player).
					Msg("Error adding new player to DB")
				return confirmedPlayers, err
			}
		}

		// add/remove 6 months from the highest / lowest date
		// tis is 7 * 24 * 4 * 6 idk 4032 hours

		dataPlayer := PlayerData{
			TwitchID:       twitchID,
			PlayerID:       playerID,
			confirmedDates: playersFishingDate[player],
		}

		confirmedPlayers[player] = append(confirmedPlayers[player], dataPlayer)
	}

	return confirmedPlayers, nil
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

func GetAllThePlayerNamesAndWhenTheyFished(fishes []FishInfo, pool *pgxpool.Pool) (map[string][]Dates, error) {

	playersFishingDate := make(map[string][]Dates)
	playersFishingFish := make(map[string][]FishInfo)
	var err error

	// first add the player to the map and get the fish that name caught
	for _, fish := range fishes {

		playersFishingFish[fish.Player] = append(playersFishingFish[fish.Player], fish)
	}

	for player := range playersFishingFish {

		playersFishingDate[player], err = AllTheDaysAPlayerFished(playersFishingFish[player], pool)
		if err != nil {
			return playersFishingDate, err
		}
	}

	return playersFishingDate, nil
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
		months, years, err := DiffBetweenTwoDates(lastDay, day, pool)
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

	// if the player never stopped fishing for more than 6 months in the ckecked data
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
