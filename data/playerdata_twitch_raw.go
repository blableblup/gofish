package data

import (
	"fmt"
	"gofish/logs"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

func CheckRawLogsForPlayer(player string, url string) (int, bool, error) {

	resp, err := http.Get(url)
	if err != nil {
		logs.Logs().Error().
			Str("URL", url).
			Str("Player", player).
			Msg("Error getting raw logs page")
		return 0, false, err
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode != 404 {
			logs.Logs().Error().
				Str("URL", url).
				Str("Player", player).
				Int("HTTP Code", resp.StatusCode).
				Msg("Unexpected HTTP status code for raw logs page")
			return 0, false, nil
		} else {
			logs.Logs().Error().
				Str("URL", url).
				Str("Player", player).
				Int("HTTP Code", resp.StatusCode).
				Msg("No logs for player raw logs page????")
			return 0, false, nil
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logs.Logs().Error().
			Str("URL", url).
			Str("Player", player).
			Msg("Error reading raw logs response body")

	}
	resp.Body.Close()

	page := string(body)

	// not looking for display name because that can be in other character thingys
	// idk what this is though
	whatToLookFor := fmt.Sprintf(":%s!%s@%s.tmi.twitch.tv", player, player, player)

	userIDRegex := regexp.MustCompile(`user-id=(\d+);`)

	lines := strings.SplitSeq(page, "\n")

	var foundTwitchID bool
	var twitchID int

	for line := range lines {
		line = strings.ToLower(line)

		if strings.Contains(line, whatToLookFor) {
			twitchIDString := userIDRegex.FindStringSubmatch(line)

			twitchID, err = strconv.Atoi(twitchIDString[1])
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("URL", url).
					Str("Player", player).
					Str("TwitchID", twitchIDString[1]).
					Msg("Error converting found twitchid to string in raw page!!!")
				return twitchID, foundTwitchID, err
			}
			foundTwitchID = true
			break
		}
	}

	return twitchID, foundTwitchID, nil
}
