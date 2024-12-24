package leaderboards

import (
	"gofish/logs"
	"gofish/utils"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Leaderboards(pool *pgxpool.Pool, leaderboards string, chatNames string, date string, date2 string, path string, title string, limit string, mode string) {

	config := utils.LoadConfig()

	leaderboardList := strings.Split(leaderboards, ",")

	// So that you can make "past" boards for a date or for a time period (like 2023 boards)
	// So if "date" is 2024-01-01 and "date2" is 2022-12-31 it will only count data for 2023
	// If "date" is 2024-05-10 it will only count data up to 2024-05-09
	// If "date2" is 2023-01-01 it will only count data after that date
	// By default, "date" is the next day. So that it considers all data
	if date == "" {
		currentDate := time.Now()
		nextDay := currentDate.AddDate(0, 0, 1)
		date = nextDay.Format("2006-01-02")
	}

	if date2 == "" {
		// This is one day before the first ever fish was caught (on justlog)
		oldDate := time.Date(2022, 12, 3, 0, 0, 0, 0, time.UTC)
		date2 = oldDate.Format("2006-01-02")
	}

	if !isValidDate(date) {
		logs.Logs().Error().
			Str("Date", date).
			Msg("Date is in the wrong format")
		return
	}

	if !isValidDate(date2) {
		logs.Logs().Error().
			Str("Date2", date2).
			Msg("Date2 is in the wrong format")
		return
	}

	params := LeaderboardParams{
		Pool:     pool,
		Mode:     mode,
		ChatName: chatNames,
		Config:   config,
		Date:     date,
		Date2:    date2,
		Path:     path,
		Title:    title,
		Limit:    limit,
	}

	// Rare, stats, shiny and averageweight are the only boards which are "global only"
	// They do not go to processLeaderboard, instead they directly go to their function
	// And they do not need a chat specified. Could change it so that chat needs to be global ?
	for _, leaderboard := range leaderboardList {
		params.LeaderboardType = leaderboard
		switch leaderboard {
		case "records":
			processLeaderboard(config, params, processChannelRecords)
		case "unique":
			processLeaderboard(config, params, processUniqueFish)
		case "typesmall":
			processLeaderboard(config, params, processTypeSmall)
		case "fishweek":
			processLeaderboard(config, params, processFishweek)
		case "trophy":
			processLeaderboard(config, params, processTrophy)
		case "weight2":
			processLeaderboard(config, params, processWeight2)
		case "weight":
			processLeaderboard(config, params, processWeight)
		case "count":
			processLeaderboard(config, params, processCount)
		case "type":
			processLeaderboard(config, params, processType)
		case "averageweight":
			processAverageWeight(params)
		case "rare":
			RunCountFishTypesGlobal(params)
		case "stats":
			RunChatStatsGlobal(params)
		case "shiny":
			processShinies(params)

		case "all":
			params.LeaderboardType = "shiny"
			processShinies(params)
			params.LeaderboardType = "stats"
			RunChatStatsGlobal(params)
			params.LeaderboardType = "averageweight"
			processAverageWeight(params)
			params.LeaderboardType = "rare"
			RunCountFishTypesGlobal(params)
			params.LeaderboardType = "type"
			processLeaderboard(config, params, processType)
			params.LeaderboardType = "count"
			processLeaderboard(config, params, processCount)
			params.LeaderboardType = "weight"
			processLeaderboard(config, params, processWeight)
			params.LeaderboardType = "weight2"
			processLeaderboard(config, params, processWeight2)
			params.LeaderboardType = "trophy"
			processLeaderboard(config, params, processTrophy)
			params.LeaderboardType = "fishweek"
			processLeaderboard(config, params, processFishweek)
			params.LeaderboardType = "typesmall"
			processLeaderboard(config, params, processTypeSmall)
			params.LeaderboardType = "unique"
			processLeaderboard(config, params, processUniqueFish)
			params.LeaderboardType = "records"
			processLeaderboard(config, params, processChannelRecords)
		default:
			logs.Logs().Info().
				Str("Leaderboard", leaderboard).
				Msg("＞︿＜ Invalid leaderboard specified")
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
					continue
				}

				logs.Logs().Info().
					Str("Chat", chatName).
					Str("Board", params.LeaderboardType).
					Msg("Checking leaderboard for chat")

				params.ChatName = chatName
				params.Chat = chat

				if chatName != "global" {
					params.Global = false
					processFunc(params)
				} else {
					params.Global = true
					processGlobalLeaderboard(params)
				}
			}

		case "":

			logs.Logs().Warn().
				Msg("Please specify chat names")

		default:

			// Process the specified chat
			chat, ok := config.Chat[chatName]
			if !ok {
				logs.Logs().Warn().
					Str("Chat", chatName).
					Msg("Chat not found in config")
				continue
			}
			if !chat.BoardsEnabled {
				if chatName != "default" {
					logs.Logs().Warn().
						Str("Chat", chatName).
						Msg("Skipping chat because board_enabled is false")
				}
				continue
			}

			logs.Logs().Info().
				Str("Chat", chatName).
				Str("Board", params.LeaderboardType).
				Msg("Checking leaderboard for chat")

			params.ChatName = chatName
			params.Chat = chat

			if chatName != "global" {
				params.Global = false
				processFunc(params)
			} else {
				params.Global = true
				processGlobalLeaderboard(params)
			}
		}
	}
}

func processGlobalLeaderboard(params LeaderboardParams) {

	switch params.LeaderboardType {
	case "weight":
		params.LeaderboardType += "global"
		processWeight(params)
	case "weight2":
		params.LeaderboardType += "global"
		processWeight2(params)
	case "count":
		params.LeaderboardType += "global"
		RunCountGlobal(params)
	case "type":
		params.LeaderboardType += "global"
		processType(params)
	case "typesmall":
		params.LeaderboardType += "global"
		processTypeSmall(params)
	case "unique":
		params.LeaderboardType += "global"
		processUniqueFish(params)
	case "records":
		params.LeaderboardType += "global"
		processChannelRecords(params)
	default:
		logs.Logs().Warn().
			Str("Board", params.LeaderboardType).
			Msg("（︶^︶） There is no global leaderboard for that board")
	}
}

type LeaderboardParams struct {
	Chat            utils.ChatInfo
	Pool            *pgxpool.Pool
	Config          utils.Config
	Date            string
	Date2           string
	Title           string
	Limit           string
	Path            string
	ChatName        string
	Mode            string
	LeaderboardType string
	Global          bool
}

func isValidDate(date string) bool {
	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}( \d{2}:\d{2}:\d{2})?$`)
	return re.MatchString(date)
}
