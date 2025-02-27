package playerdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"gofish/logs"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

// Custom error for not finding a player in both the apis
var ErrNoPlayerFound = errors.New("no player found")

// Get the userdata for a single player
func MakeApiRequestForPlayerToApiIVR(player string) ([]map[string]interface{}, error) {

	var userdata []map[string]interface{}
	url := fmt.Sprintf("https://api.ivr.fi/v2/twitch/user?login=%s", player)
	response, err := http.Get(url)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("URL", url).
			Str("Player", player).
			Msg("Error fetching twitch id for player")
		return userdata, err
	}

	if response.StatusCode != http.StatusOK {
		logs.Logs().Error().
			Str("URL", url).
			Str("Player", player).
			Int("HTTP Code", response.StatusCode).
			Msg("Unexpected HTTP status code")
		return userdata, fmt.Errorf("unexpected HTTP status code")
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		logs.Logs().Error().
			Str("URL", url).
			Str("Player", player).
			Msg("Error reading response body")
		return userdata, err
	}
	response.Body.Close()

	err = json.Unmarshal(body, &userdata)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("URL", url).
			Str("Player", player).
			Msg("Error unmarshalling json")
		return userdata, err
	}

	if len(userdata) == 0 {
		logs.Logs().Error().
			Str("URL", url).
			Str("Player", player).
			Msg("No player found")
		return userdata, ErrNoPlayerFound
	}

	return userdata, nil
}

// Try to get the twitch id for a name
func GetTwitchID(player string) (int, error) {

	userdata, err := MakeApiRequestForPlayerToApiIVR(player)
	if err != nil {
		return 0, err
	}

	id, err := strconv.Atoi(userdata[0]["id"].(string))
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("ID", userdata[0]["id"].(string)).
			Str("Player", player).
			Msg("Error converting id to int")
		return 0, err
	}

	return id, nil
}

// Get the pfp
func GetTwitchPFP(player string) (string, error) {
	userdata, err := MakeApiRequestForPlayerToApiIVR(player)
	if err != nil {
		return "", err
	}

	pfp := userdata[0]["logo"].(string)

	return pfp, nil
}

// This checks that other website
func GetTwitchID2(player string) (int, error) {

	url := fmt.Sprintf("https://kunszg.com/api/user?username=%s", player)
	response, err := http.Get(url)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("URL", url).
			Str("Player", player).
			Msg("Error fetching twitch id for player")
		return 0, err
	}

	if response.StatusCode != http.StatusOK {
		if response.StatusCode != 429 {
			logs.Logs().Error().
				Str("URL", url).
				Str("Player", player).
				Int("HTTP Code", response.StatusCode).
				Msg("Unexpected HTTP status code")
			return 0, fmt.Errorf("unexpected HTTP status code")
		} else {
			// >.<
			logs.Logs().Warn().
				Str("URL", url).
				Str("Player", player).
				Msg("Too many requests")
			sleep := time.Second * 60
			time.Sleep(sleep)
			GetTwitchID2(player)
		}
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		logs.Logs().Error().
			Str("URL", url).
			Str("Player", player).
			Msg("Error reading response body")
		return 0, err
	}
	response.Body.Close()

	var userdata map[string]interface{}

	err = json.Unmarshal(body, &userdata)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("URL", url).
			Str("Player", player).
			Msg("Error unmarshalling json")
		return 0, err
	}

	if userdata["status"].(float64) == 404 {
		logs.Logs().Error().
			Str("URL", url).
			Str("Player", player).
			Msg("No player found")
		return 0, ErrNoPlayerFound
	}

	if userdata["status"].(float64) == 403 {
		logs.Logs().Error().
			Str("URL", url).
			Str("Player", player).
			Msg("User opted out from this endpoint")
		return 0, fmt.Errorf("user opted out from this endpoint")
	}

	// This website also stores the users name history
	// This doesnt rename the players though
	// And if the player had fished on multiple accounts,
	// there will be multiple entries in playerdata with the same twitchid
	// Thats why I also have mergetwitchid
	if userdata["currentUsername"].(string) != player {
		logs.Logs().Warn().
			Str("currentUsername", userdata["currentUsername"].(string)).
			Str("OldName", player).
			Msg("Player renamed")
	}

	id, err := strconv.Atoi(userdata["userid"].(string))
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("ID", userdata["userid"].(string)).
			Str("Player", player).
			Msg("Error converting id to int")
		return 0, err
	}

	return id, nil
}

// Check the official data from bread
func GetTwitchID3(player string) (int, error) {

	var userdata map[string]interface{}

	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	players_dump := filepath.Join(dir, "players_dump.json")

	file, err := os.ReadFile(players_dump)
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error reading players_dump")
		return 0, err
	}

	err = json.Unmarshal(file, &userdata)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Player", player).
			Msg("Error unmarshalling json from players_dump")
		return 0, err
	}

	for key, value := range userdata {

		playerData, ok := value.(map[string]interface{})
		if !ok {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error putting players data into map")
			return 0, fmt.Errorf("error putting players data into map")
		}

		if key == player {
			id, err := strconv.Atoi(playerData["twitchId"].(string))
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("ID", playerData["twitchId"].(string)).
					Str("Player", player).
					Msg("Error converting id to int")
				return 0, err
			}
			return id, nil
		}

		// Go through the duplicates if there are any
		if duplicates, found := playerData["duplicates"].([]interface{}); found {
			for _, duplicate := range duplicates {
				if duplicate.(string) == player {
					id, err := strconv.Atoi(playerData["twitchId"].(string))
					if err != nil {
						logs.Logs().Error().Err(err).
							Str("ID", playerData["twitchId"].(string)).
							Str("Player", player).
							Msg("Error converting id to int")
						return 0, err
					}
					logs.Logs().Warn().
						Str("Player", key).
						Str("Duplicate", duplicate.(string)).
						Msg("Player was merged and renamed")
					return id, nil
				}
			}
		}
	}

	logs.Logs().Error().
		Str("Player", player).
		Msg("Player not found in players_dump")

	deleted, err := CheckIfTheyWereDeleted(player)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Player", player).
			Msg("Error checking if player was deleted")
		return 0, err
	}

	if deleted {
		logs.Logs().Warn().
			Str("Player", player).
			Msg("Player was deleted")
		return 0, fmt.Errorf("player deleted")
	}

	logs.Logs().Error().
		Str("Player", player).
		Msg("Player not found in deleted_players")
	return 0, fmt.Errorf("player also not found in deleted_players")

}

func CheckIfTheyWereDeleted(player string) (bool, error) {

	var userdata []string

	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	deleted_players := filepath.Join(dir, "deleted_players.json")

	file, err := os.ReadFile(deleted_players)
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error reading deleted_players")
		return false, err
	}

	err = json.Unmarshal(file, &userdata)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Player", player).
			Msg("Error unmarshalling json from deleted_players")
		return false, err
	}

	for _, deletedplayer := range userdata {
		if deletedplayer == player {
			return true, nil
		}
	}

	return false, nil
}
