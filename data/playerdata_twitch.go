package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"gofish/logs"
	"io"
	"net/http"
	"strconv"
)

// custom error idk
var ErrNoPlayerFound = errors.New("no player found")

// what can be id or name
func MakeApiRequestForPlayerToApiIVR(player string, twitchID int, what string) ([]map[string]interface{}, error) {

	var userdata []map[string]interface{}

	var url string

	// maybe instead of getting the same userdata two times
	// store it somewhere ?
	switch what {

	case "id":
		url = fmt.Sprintf("https://api.ivr.fi/v2/twitch/user?id=%d", twitchID)

	case "name":
		url = fmt.Sprintf("https://api.ivr.fi/v2/twitch/user?login=%s", player)

	default:
		logs.Logs().Fatal().Msg("MakeApiRequestForPlayerToApiIVR")
	}

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

func GetTwitchID(player string) (int, error) {

	userdata, err := MakeApiRequestForPlayerToApiIVR(player, 0, "name")
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

func GetCurrentName(twitchID int) (string, error) {

	userdata, err := MakeApiRequestForPlayerToApiIVR("", twitchID, "id")
	if err != nil {
		return "", err
	}

	name := userdata[0]["login"].(string)

	return name, nil
}

func GetTwitchPFP(player string) (string, error) {
	userdata, err := MakeApiRequestForPlayerToApiIVR(player, 0, "name")
	if err != nil {
		return "", err
	}

	pfp := userdata[0]["logo"].(string)

	return pfp, nil
}
