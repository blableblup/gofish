package leaderboards

import (
	"gofish/data"
	"gofish/logs"
	"gofish/utils"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
)

func Leaderboards(leaderboards string, chatNames string, mode string) {
	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		logs.Logs().Fatal().Err(err).Msg("Error getting current working directory")
	}

	configFilePath := filepath.Join(wd, "config.json")
	config := utils.LoadConfig(configFilePath)

	leaderboardList := strings.Split(leaderboards, ",")

	pool, err := data.Connect()
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error connecting to the database")
		return
	}
	defer pool.Close()

	params := LeaderboardParams{
		Pool:     pool,
		Mode:     mode,
		ChatName: chatNames,
		Config:   config,
	}

	for _, leaderboard := range leaderboardList {
		params.LeaderboardType = leaderboard
		switch leaderboard {
		case "fishw":
			processLeaderboard(config, params, processFishweek)
		case "trophy":
			processLeaderboard(config, params, processTrophy)
		case "weight":
			processLeaderboard(config, params, processWeight)
		case "count":
			processLeaderboard(config, params, processCount)
		case "type":
			processLeaderboard(config, params, processType)
		case "rare":
			RunCountFishTypesGlobal(params)
		// case "countday":
		// 	RunCountDay(params)

		case "all":
			logs.Logs().Info().Msg("Updating all leaderboards...")
			RunCountFishTypesGlobal(params)
			params.LeaderboardType = "type"
			processLeaderboard(config, params, processType)
			params.LeaderboardType = "count"
			processLeaderboard(config, params, processCount)
			params.LeaderboardType = "weight"
			processLeaderboard(config, params, processWeight)
			params.LeaderboardType = "trophy"
			processLeaderboard(config, params, processTrophy)
			params.LeaderboardType = "fishweek"
			processLeaderboard(config, params, processFishweek)
		default:
			logs.Logs().Info().Msgf("＞︿＜ Invalid leaderboard specified: %s", leaderboard)
		}
	}
}

func processLeaderboard(config utils.Config, params LeaderboardParams, processFunc func(LeaderboardParams)) {
	switch params.ChatName {
	case "all":
		// Process all chats
		for chatName, chat := range config.Chat {
			if !chat.BoardsEnabled {
				if chatName != "global" && chatName != "default" {
					logs.Logs().Warn().Msgf("Skipping chat '%s' because board_enabled is false", chatName)
				}
				continue
			}

			logs.Logs().Info().Msgf("Checking leaderboard '%s' for chat '%s'", params.LeaderboardType, chatName)
			params.ChatName = chatName
			params.Chat = chat
			processFunc(params)
		}
	case "global":
		processGlobalLeaderboard(params)
	case "":
		logs.Logs().Info().Msg("Please specify chat names")
	default:
		// Process specified chat names
		specifiedchatNames := strings.Split(params.ChatName, ",")
		for _, chatName := range specifiedchatNames {
			chat, ok := config.Chat[chatName]
			if !ok {
				logs.Logs().Warn().Msgf("Chat '%s' not found in config.\n", chatName)
				continue
			}
			if !chat.BoardsEnabled {
				if chatName != "global" && chatName != "default" {
					logs.Logs().Warn().Msgf("Skipping chat '%s' because board_enabled is false", chatName)
				}
				continue
			}

			logs.Logs().Info().Msgf("Checking leaderboard '%s' for chat '%s'", params.LeaderboardType, chatName)
			params.ChatName = chatName
			params.Chat = chat
			processFunc(params)
		}
	}
}

func processGlobalLeaderboard(params LeaderboardParams) {

	switch params.LeaderboardType {
	case "weight":
		RunWeightGlobal(params)
	case "count":
		RunCountGlobal(params)
	case "type":
		RunTypeGlobal(params)
	default:
		logs.Logs().Info().Msgf("（︶^︶） There is no global leaderboard for that board '%s'", params.LeaderboardType)
	}
}

type LeaderboardParams struct {
	Chat            utils.ChatInfo
	Pool            *pgxpool.Pool
	Config          utils.Config
	ChatName        string
	Mode            string
	LeaderboardType string
}
