package scripts

import (
	"encoding/json"
	"fmt"
	"gofish/logs"
	"gofish/utils"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
)

// To get all the different justlog instances for a chat from https://logs.zonian.dev/instances
func GetInstances() {

	config := utils.LoadConfig()

	instancesapi, err := doApiToLogsZonian()
	if err != nil {
		return
	}

	// If a lot of instances are down it wont find all the channels
	logs.Logs().Info().
		Int("Down", instancesapi.InstancesStats.Down).
		Int("Up", instancesapi.InstancesStats.Count).
		Msg("Status of API")

	// Get all the different instances which have the chats
	instanceswhichhavechannel := make(map[string][]string)
	for instance, channels := range instancesapi.Instances {

		if len(channels) == 0 {
			logs.Logs().Warn().
				Str("Instance", instance).
				Msg("Instance is down")
			continue
		}

		for _, channel := range channels {
			for chatName, chat := range config.Chat {
				if chatName == "global" || chatName == "default" {
					continue
				}

				if channel.Name == chatName && channel.UserID == chat.TwitchID {
					instanceswhichhavechannel[chatName] = append(instanceswhichhavechannel[chatName], instance)
				}
			}
		}
	}

	for chatName, chat := range config.Chat {
		if chatName == "global" || chatName == "default" {
			continue
		}

		// Get the chats instances from the config
		configinstancesslice := chat.LogsInstances

		// Skip chats with no instances in the API
		if len(instanceswhichhavechannel[chatName]) == 0 {
			logs.Logs().Warn().
				Str("Chat", chatName).
				Msg("No instances found for chat")
			continue
		}

		// Warn if the instance from the config cant be found in the API
		// This will always log for logs.joinuv and the private instance for vaia
		for _, existinginstance := range configinstancesslice {
			if !slices.Contains(instanceswhichhavechannel[chatName], strings.TrimPrefix(existinginstance.URL, "https://")) {
				logs.Logs().Warn().
					Str("Chat", chatName).
					Str("Instance", existinginstance.URL).
					Msg("Instance found in config not in API")
			}
		}

		// Find the instances which arent already in the config
		for _, instance := range instanceswhichhavechannel[chatName] {
			instanceisnew := true
			channeloptedoutofinstance := false

			for _, existinginstance := range configinstancesslice {
				if strings.Contains(existinginstance.URL, instance) {
					instanceisnew = false
					break
				}
			}

			if instanceisnew {

				// Find how far back the logs go to find the "logs_added" for the instance
				timevar := time.Now().UTC()

				// Loop through the urls starting from current month
				i := 0
				monthsinarowwhich404d := 0
				lastmonthwhichdidnt404 := "no logs added found"
				for {
					firstOfMonth := time.Date(timevar.Year(), timevar.Month()-time.Month(i), 1, 0, 0, 0, 0, time.UTC)
					year, month, _ := firstOfMonth.Date()

					url := fmt.Sprintf("https://%s/channel/%s/user/gofishgame/%d/%d", instance, chatName, year, month)
					response, err := http.Get(url)
					if err != nil {
						logs.Logs().Error().Err(err).
							Msg("Error making request")
					}

					if response.StatusCode != http.StatusOK {
						if response.StatusCode == 404 {
							monthsinarowwhich404d++
						}
						// Skip instances from which the channel opted out of
						if response.StatusCode == 403 {
							logs.Logs().Warn().
								Str("Chat", chatName).
								Str("Instance", instance).
								Msg("Channel opted out of instance")
							channeloptedoutofinstance = true
							break
						}
					} else {
						monthsinarowwhich404d = 0
						lastmonthwhichdidnt404 = fmt.Sprintf("%d/%d", year, month)
					}

					// So if there is 404 12 months in a row use the last month which didnt as logs added
					// If the channel had gofish but noone fished there for over a year this wont work though
					// If the channel has gofish but the instance had the chat added later this also wont work if the bot didnt type anything since the instance was added
					// Need to set logs added manually in those cases
					if monthsinarowwhich404d >= 12 {
						break
					}

					i++
				}

				if channeloptedoutofinstance {
					continue
				}

				logs.Logs().Info().
					Str("Chat", chatName).
					Str("Instance", instance).
					Str("LogsAdded", lastmonthwhichdidnt404).
					Msg("New instance found for chat")

				NewInstance := utils.Instance{
					URL:       fmt.Sprintf("https://%s", instance),
					LogsAdded: lastmonthwhichdidnt404,
				}

				configinstancesslice = append(configinstancesslice, NewInstance)
			}
		}

		// Update the instances in the config for the chat
		if blaaa, ok := config.Chat[chatName]; ok {

			blaaa.LogsInstances = configinstancesslice

			config.Chat[chatName] = blaaa
		}
	}

	logs.Logs().Info().
		Msg("Done checking the api")

	// Rewrite the config file
	file, err := os.Create("config" + ".json")
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error opening config file")
		return
	}
	defer file.Close()

	bytes, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error updating config file json")
		return
	}

	_, err = fmt.Fprintf(file, "%s", bytes)
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error writing config file")
		return
	}

	logs.Logs().Info().
		Msg("Updated config")
}

func doApiToLogsZonian() (LogsZonianChannelAPI, error) {

	var instancesapi LogsZonianChannelAPI

	response, err := http.Get("https://logs.zonian.dev/instances")
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error fetching instances")
		return instancesapi, err
	}

	if response.StatusCode != http.StatusOK {
		logs.Logs().Error().
			Int("HTTP Code", response.StatusCode).
			Msg("Unexpected HTTP status code")
		return instancesapi, fmt.Errorf("unexpected HTTP status code")
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error reading response body")
		return instancesapi, err
	}
	response.Body.Close()

	err = json.Unmarshal(body, &instancesapi)
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error unmarshalling json")
		return instancesapi, err
	}

	return instancesapi, nil
}

type LogsZonianChannelAPI struct {
	InstancesStats struct {
		Count int `json:"count"`
		Down  int `json:"down"`
	} `json:"instancesStats"`
	Instances map[string][]Channel `json:"instances"`
}

type Channel struct {
	Name   string `json:"name"`
	UserID string `json:"userID"`
}
