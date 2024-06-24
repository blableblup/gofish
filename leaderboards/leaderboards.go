package leaderboards

import (
	"gofish/data"
	"gofish/logs"
	"gofish/utils"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
)

func Leaderboards(leaderboards string, chatNames string, mode string) {

	config := utils.LoadConfig()

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
		case "fishweek":
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
		case "stats":
			RunChatStatsGlobal(params)
		// case "countday":
		// 	RunCountDay(params)

		case "all":
			RunChatStatsGlobal(params)
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
			logs.Logs().Info().Str("Leaderboard", leaderboard).Msg("＞︿＜ Invalid leaderboard specified")
		}
	}
}

func processLeaderboard(config utils.Config, params LeaderboardParams, processFunc func(LeaderboardParams)) {

	specifiedchatNames := strings.Split(params.ChatName, ",")
	for _, chatName := range specifiedchatNames {

		switch chatName {
		case "all":

			// Process all chats
			for chatName, chat := range config.Chat {
				if !chat.BoardsEnabled {
					if chatName != "global" && chatName != "default" {
						logs.Logs().Warn().Str("Chat", chatName).Msg("Skipping chat because board_enabled is false")
					}
					continue
				}

				logs.Logs().Info().Str("Chat", chatName).Str("Board", params.LeaderboardType).Msg("Checking leaderboard for chat")
				params.ChatName = chatName
				params.Chat = chat
				processFunc(params)
			}

		case "global":
			processGlobalLeaderboard(params)
		case "":
			logs.Logs().Warn().Msg("Please specify chat names")
		default:

			// Process the specified chat
			chat, ok := config.Chat[chatName]
			if !ok {
				logs.Logs().Warn().Str("Chat", chatName).Msg("Chat not found in config")
				continue
			}
			if !chat.BoardsEnabled {
				if chatName != "global" && chatName != "default" {
					logs.Logs().Warn().Str("Chat", chatName).Msg("Skipping chat because board_enabled is false")
				}
				continue
			}

			logs.Logs().Info().Str("Chat", chatName).Str("Board", params.LeaderboardType).Msg("Checking leaderboard for chat")
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
		logs.Logs().Warn().Str("Board", params.LeaderboardType).Msg("（︶^︶） There is no global leaderboard for that board")
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
