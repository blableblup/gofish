package data

import (
	"context"
	"gofish/logs"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PlayerData struct {
	PlayerID int

	confirmedDates []Dates
	// this is array, because a player can use one of their names multiple times
}

type Dates struct {
	HighestDate time.Time
	LowestDate  time.Time
}

func ConfirmWhoIsWho(fishes []FishInfo, pool *pgxpool.Pool) (map[string][]PlayerData, error) {

	confirmedPlayers := make(map[string][]PlayerData)

	// check the logs page like this: https://logs.joinuv.com/channel/breadworms/2025/10/26/?json=true
	// and then search for the players name to get their user-id

	// need to open that page until every player has their twitchid confirmed

	// OLDER LOGS in logs ivr have the names renamed instead of their historic name at that time !!!

	playersFishingDate := GetAllThePlayerNamesAndWhenTheyFished(fishes)

	for player := range playersFishingDate {

	}

	// can confirm for last 6 months atleast that that player is actually this player
	// since names arent available for others for 6 months after renaming

	// and if someone hasnt been fishing for 6 months and fishes again
	// need to check twitchid again

	// and then check for player with that twitchid in db

	// and rename them if name is different from current name
	// ?

	// for current name: can check api.ivr ?
	// but need to only rename player if they actually caught a fish after they renamed ? IDK

	// right now im only renaming a player if they caught a fish / did + bag

	return confirmedPlayers, nil
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

	var lastDay, nextDay time.Time

	for _, fish := range fishes {

		// idk
		oneday := (time.Hour * 24)

		nextDay = fish.Date.Truncate(oneday)

		if nextDay.Equal(lastDay) {
			continue
		} else {
			days = append(days, nextDay)
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
