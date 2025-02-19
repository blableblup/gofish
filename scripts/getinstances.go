package scripts

import (
	"encoding/json"
	"fmt"
	"gofish/logs"
	"gofish/utils"
	"io"
	"net/http"
	"os"
	"strings"
)

// To get all the different justlog instances for a chat from https://logs.zonian.dev/instances
func GetInstances() {

	config := utils.LoadConfig()

	instancesapi, err := doApiToLogsZonian()
	if err != nil {
		return
	}

	for chatName, chat := range config.Chat {
		if !chat.CheckFData {
			if chatName != "global" && chatName != "default" {
				logs.Logs().Warn().
					Str("Chat", chatName).
					Msg("Skipping chat because checkfdata is false")
			}
			continue
		}

		// Get all the different instances which have the chat
		var instanceswhichhavechannel []string
		for instance, channels := range instancesapi.Instances {
			for _, channel := range channels {
				if channel.Name == chatName {
					instanceswhichhavechannel = append(instanceswhichhavechannel, instance)
				} // Can also check the twitchid channel.UserID, need to also store twitchid in config file!
			}
		}

		// Find the instances which arent already in the config
		// Can maybe also find instances which had the chat originally but then removed the chat (?) And then update the config ?
		// This will also find instances for a channel even if the channel has opted out of being logged !
		configinstancesslice := chat.LogsInstances
		for _, instance := range instanceswhichhavechannel {
			instanceisnew := true

			for _, existinginstance := range configinstancesslice {
				if strings.Contains(existinginstance.URL, instance) {
					instanceisnew = false
					break
				}
			}
			if instanceisnew {
				logs.Logs().Info().
					Str("Chat", chatName).
					Str("Instance", instance).
					Msg("New instance found for chat")

				NewInstance := utils.Instance{
					URL:       fmt.Sprintf("https://%s", instance),
					LogsAdded: "add this manually",
					// could check which url returns 404 first and then use the last one that didnt as logsadded ?
					// but checking it manually probably better, there could also be months in which gofishgame didnt type anything in the small chats
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
	err = writeConfigAgain(config)
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error updating config file")
		return
	}

	logs.Logs().Info().
		Msg("Updated config")
}

// This writes the config chats ordered by their name instead of how i had them ordered by when they added gofish
// And also "default" and "global" wont be at the bottom
func writeConfigAgain(config utils.Config) error {

	file, err := os.Create("config" + ".json")
	if err != nil {
		return err
	}
	defer file.Close()

	bytes, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(file, "%s", bytes)

	return nil
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
