package data

import (
	"context"
	"gofish/logs"
	"gofish/utils"
	"strconv"
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

	logs.Logs().Info().Msg("Going over all the players for playerids.....")

	confirmedPlayers := make(map[string][]PlayerData)

	playersFishingDate, firstFishChats, err := GetAllThePlayerNamesAndWhenTheyFished(fishes, pool)
	if err != nil {
		return confirmedPlayers, err
	}

	for player := range playersFishingDate {

		// this still doesnt work if a player name was used by different players in the past
		// all the fish will just go to the fisher who is currently using that name
		// but since im not rechecking ALL the logs anytime soon
		// this shouldnt be bad ?
		if len(playersFishingDate[player]) > 1 {
			logs.Logs().Warn().
				Str("Player", player).
				Interface("Dates", playersFishingDate[player]).
				Msg("PLAYer has more than 1 dates!!!!!")
			// idk
		}

		// range over the dates here idk

		// try to get their twitchID
		var twitchID int
		userdata, err := MakeApiRequestForPlayerToApiIVR(player, 0, "name")
		if err != nil && err != ErrNoPlayerFound {

			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error doing api ivr thing")
			return confirmedPlayers, err

		} else if err == ErrNoPlayerFound {

			logs.Logs().Warn().
				Str("Player", player).
				Interface("Fishing dates", playersFishingDate[player]).
				Msg("Cant get twitchID for player!!!!!")

			logs.Logs().Warn().Msg("WHAT IS THEIR TWITCHID? TYPE IT BELOW: ")
			// check the logs page manually idk
			// like this : https://logs.joinuv.com/channel/breadworms/2025/10/26/?json=true

			twitchIDString, err := utils.ScanAndReturn()
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Player", player).
					Msg("Error reading the thing for twitchID")
				return confirmedPlayers, err
			}

			twitchID, err = strconv.Atoi(twitchIDString)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("twitchID", twitchIDString).
					Msg("Error converting twitchID to int")
				return confirmedPlayers, err
			}

		} else if err == nil {

			twitchID, err = GetTwitchID(userdata)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Player", player).
					Msg("Error getting twitchID for player")
				return confirmedPlayers, err
			}
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

			// only rename if the player name is in api idk
			if len(userdata) != 0 {
				currentName := GetCurrentName(userdata)

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
			}

			playerID = DBData.PlayerID

		} else {

			var firstFishDate time.Time

			// idk if this is more than 1 dates
			for _, dates := range playersFishingDate[player] {
				firstFishDate = dates.LowestDate
			}
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
		// and idk if this is more than 1 date
		var confirmedDateEdited []Dates

		for _, confirmedDate := range playersFishingDate[player] {

			newDates := Dates{
				HighestDate: confirmedDate.HighestDate.Add(time.Hour * 4032),
				LowestDate:  confirmedDate.LowestDate.Add(time.Hour * -4032),
			}

			confirmedDateEdited = append(confirmedDateEdited, newDates)
		}

		dataPlayer := PlayerData{
			TwitchID:       twitchID,
			PlayerID:       playerID,
			confirmedDates: confirmedDateEdited,
		}

		confirmedPlayers[player] = append(confirmedPlayers[player], dataPlayer)
	}

	logs.Logs().Info().
		Int("Amount of players", len(confirmedPlayers)).
		Msg("Finished..")

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
