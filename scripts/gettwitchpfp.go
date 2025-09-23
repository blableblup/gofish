package scripts

import (
	"fmt"
	"gofish/logs"
	"gofish/playerdata"
	"gofish/utils"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Save the pfps for channels in the config which dont have a file with their name in /images/players
// or save them for all the chats again if mode is "all"
// i rescale the pfps afterwards to make them 25x25
func GetTwitchPFPs(mode string) {

	config := utils.LoadConfig()

	for chatName := range config.Chat {

		if chatName == "global" || chatName == "default" {
			continue
		}

		filePath := filepath.Join("images", "players", fmt.Sprintf("%s.png", chatName))

		var file *os.File
		var err error
		if mode != "all" {
			// Check if the file exists
			// "O_EXCL   int = syscall.O_EXCL   // used with O_CREATE, file must not exist."
			// will error if file exists
			file, err = os.OpenFile(filePath, os.O_CREATE|os.O_EXCL, 0666)
			if err != nil {
				continue
			}
		} else {
			file, err = os.OpenFile(filePath, os.O_CREATE, 0666)
			if err != nil {
				logs.Logs().Error().Err(err).
					Msg("Error creating file")
				return
			}
		}
		defer file.Close()

		// Get the pfp link from api, make the request, save the pfp to the file
		pfp, err := playerdata.GetTwitchPFP(chatName)
		if err != nil {
			continue
		}

		response, err := http.Get(pfp)
		if err != nil {
			logs.Logs().Error().Err(err).
				Msg("Error making request")
			return
		}
		defer response.Body.Close()

		_, err = io.Copy(file, response.Body)
		if err != nil {
			logs.Logs().Error().Err(err).
				Msg("Error saving pfp to file")
			return
		}

		logs.Logs().Info().
			Str("Chat", chatName).
			Msg("Saved pfp")

	}
}
