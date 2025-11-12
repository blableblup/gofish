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

	// IDEA:
	// check the logs page like this: https://logs.joinuv.com/channel/breadworms/2025/10/26/?json=true
	// OLDER LOGS in logs ivr have the names renamed instead of their historic name at that time !!!
	// and then search for the players name to get their user-id
	// so that i always have someones twitch id

	// OR: since this doesnt actually happen a lot that i cant find the id for someone in the api
	// i could just stop the program and check the logs page manually
	// and then copy the id out of the logs ?

	playersFishingDate := GetAllThePlayerNamesAndWhenTheyFished(fishes)

	// this doesnt work for players whose name was also used by other players
	// need to check for the 6 month gap in the fishing dates somewhere ???ßßß
	for player := range playersFishingDate {

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

			playerID, err = AddNewPlayer(twitchID, player, time.Time{}, "", pool)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Player", player).
					Msg("Error adding new player to DB")
				return confirmedPlayers, err
			}
		}

		dataPlayer := PlayerData{
			TwitchID: twitchID,
			PlayerID: playerID,
		}

		confirmedPlayers[player] = append(confirmedPlayers[player], dataPlayer)
	}

	// can confirm for last 6 months atleast that that player is actually this player
	// since names arent available for others for 6 months after renaming
	// so subtract 6 months from highestdate in dates

	return confirmedPlayers, nil
}

func CheckForTwitchIDInDB(twitchID int, pool *pgxpool.Pool) (PlayerDataInDB, bool, error) {

	var DBData PlayerDataInDB

	err := pool.QueryRow(context.Background(),
		"SELECT name, playerid, twitchid, oldnames FROM playerdata WHERE twitchid = $1",
		twitchID).Scan(&DBData.Name, &DBData.PlayerID, &DBData.TwitchID, &DBData.OldNames)

	if err != pgx.ErrNoRows {

		return DBData, false, err

	} else if err == pgx.ErrNoRows {

		return DBData, false, nil
	}

	return DBData, true, nil
}

func GetAllThePlayerNamesAndWhenTheyFished(fishes []FishInfo) map[string][]time.Time {

	playersFishingDate := make(map[string][]time.Time)
	playersFishingFish := make(map[string][]FishInfo)

	// first add the player to the map and get the fish that name caught
	for _, fish := range fishes {

		playersFishingFish[fish.Player] = append(playersFishingFish[fish.Player], fish)
	}

	for player := range playersFishingDate {

		playersFishingDate[player] = AllTheDaysAPlayerFished(playersFishingFish[player])
	}

	return playersFishingDate
}

func AllTheDaysAPlayerFished(fishes []FishInfo) []time.Time {

	var days []time.Time

	oneday := (time.Hour * 24)

	var lastDay time.Time

	for _, fish := range fishes {

		day := fish.Date.Truncate(oneday)

		if day.Equal(lastDay) {
			continue
		} else {
			days = append(days, day)
			lastDay = day
		}

	}

	return days
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
