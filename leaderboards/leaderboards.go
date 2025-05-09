package leaderboards

import (
	"gofish/data"
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
		currentDate := time.Now().UTC()
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

	// Get the map with the name for the catchtypes
	Catchtypes, err := CatchtypeNames(pool)
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error getting names for catchtypes")
	}

	// Store the flags, the connection, the config and initialize the two maps for storing player and fish data
	params := LeaderboardParams{
		Pool:           pool,
		Mode:           mode,
		ChatName:       chatNames,
		Config:         config,
		Date:           date,
		Date2:          date2,
		Path:           path,
		Title:          title,
		Limit:          limit,
		Catchtypenames: Catchtypes,
		Players:        make(map[int]data.FishInfo),
		FishTypes:      make(map[string]string),
	}

	// map of all the boards
	// averageweight, rare, stats, shiny are global only, dont need a chat specified, so if there is, they get skipped
	// i do -chats all -board chatboards and then -chats global -board globalboards on sunday, and -chats all -board tourney for tournaments later
	existingboards := map[string]LeaderboardConfig{
		"fishweek":      {hasGlobal: false, GlobalOnly: false, Tournament: true, Function: processFishweek},
		"trophy":        {hasGlobal: false, GlobalOnly: false, Tournament: true, Function: processTrophy},
		"records":       {hasGlobal: true, GlobalOnly: false, Tournament: false, Function: processChannelRecords},
		"unique":        {hasGlobal: true, GlobalOnly: false, Tournament: false, Function: processUniqueFish},
		"typesmall":     {hasGlobal: true, GlobalOnly: false, Tournament: false, Function: processTypeSmall},
		"type":          {hasGlobal: true, GlobalOnly: false, Tournament: false, Function: processType},
		"count":         {hasGlobal: true, GlobalOnly: false, Tournament: false, Function: processCount},
		"weight":        {hasGlobal: true, GlobalOnly: false, Tournament: false, Function: processWeight},
		"weight2":       {hasGlobal: true, GlobalOnly: false, Tournament: false, Function: processWeight2},
		"averageweight": {hasGlobal: true, GlobalOnly: true, Tournament: false, Function: processAverageWeight},
		"rare":          {hasGlobal: true, GlobalOnly: true, Tournament: false, Function: RunCountFishTypesGlobal},
		"stats":         {hasGlobal: true, GlobalOnly: true, Tournament: false, Function: RunChatStatsGlobal},
		"shiny":         {hasGlobal: true, GlobalOnly: true, Tournament: false, Function: processShinies},
		"players":       {hasGlobal: true, GlobalOnly: true, Tournament: false, Function: GetPlayerProfiles}}

	for _, leaderboard := range leaderboardList {

		switch leaderboard {
		default:
			board, ok := existingboards[leaderboard]
			if !ok {
				logs.Logs().Warn().
					Str("Board", leaderboard).
					Msg("Board doesnt exist")
				continue
			}
			processLeaderboard(config, params, leaderboard, board)

		case "tourney":
			for boardname, board := range existingboards {
				if board.Tournament {
					processLeaderboard(config, params, boardname, board)
				}
			}

		case "nontourney":
			for boardname, board := range existingboards {
				if !board.Tournament {
					processLeaderboard(config, params, boardname, board)
				}
			}

		case "chatboards":
			for boardname, board := range existingboards {
				if !board.GlobalOnly && !board.Tournament {
					processLeaderboard(config, params, boardname, board)
				}
			}

		case "globalboards":
			for boardname, board := range existingboards {
				if board.GlobalOnly {
					processLeaderboard(config, params, boardname, board)
				}
			}

		case "all":
			for boardname, board := range existingboards {

				processLeaderboard(config, params, boardname, board)
			}
		}
	}
}

func processLeaderboard(config utils.Config, params LeaderboardParams, leaderboard string, board LeaderboardConfig) {

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
					Str("Board", leaderboard).
					Msg("Checking leaderboard for chat")

				params.LeaderboardType = leaderboard
				params.ChatName = chatName
				params.Chat = chat

				processFunc := board.Function

				if chatName != "global" {
					if board.GlobalOnly {
						logs.Logs().Warn().
							Str("Board", leaderboard).
							Str("Chat", chatName).
							Msg("Board is global only !")
						continue
					}
					params.Global = false
					processFunc(params)
				} else {
					if board.hasGlobal {
						params.Global = true
						processFunc(params)
					} else {
						logs.Logs().Warn().
							Str("Board", leaderboard).
							Msg("Board doesnt have global variant !")
						continue
					}
				}
			}

		case "":

			logs.Logs().Warn().
				Msg("Missing -chats !")

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
				Str("Board", leaderboard).
				Msg("Checking leaderboard for chat")

			params.LeaderboardType = leaderboard
			params.ChatName = chatName
			params.Chat = chat

			processFunc := board.Function

			if chatName != "global" {
				if board.GlobalOnly {
					logs.Logs().Warn().
						Str("Board", leaderboard).
						Str("Chat", chatName).
						Msg("Board is global only !")
					continue
				}
				params.Global = false
				processFunc(params)
			} else {
				if board.hasGlobal {
					params.Global = true
					processFunc(params)
				} else {
					logs.Logs().Warn().
						Str("Board", leaderboard).
						Msg("Board doesnt have global variant !")
					continue
				}
			}
		}
	}
}

type LeaderboardConfig struct {
	Name       string
	hasGlobal  bool
	GlobalOnly bool
	Tournament bool
	Function   func(LeaderboardParams)
}

// This is holding the flags (like -title and -path...)
// players is to store the info about the players
// to get their latest name, verified status and when they started fishing
// fishtypes is to store the latest emoji for a fishname
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
	Players         map[int]data.FishInfo
	FishTypes       map[string]string
	Catchtypenames  map[string]string
}

func isValidDate(date string) bool {
	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}( \d{2}:\d{2}:\d{2})?$`)
	return re.MatchString(date)
}
