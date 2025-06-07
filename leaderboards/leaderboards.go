package leaderboards

import (
	"gofish/data"
	"gofish/logs"
	"gofish/utils"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// to store info about a leaderboard and its functions
type LeaderboardConfig struct {
	Name             string
	hasGlobal        bool
	GlobalOnly       bool
	Tournament       bool
	Function         func(LeaderboardParams)
	GetFunction      func(LeaderboardParams) (map[string]data.FishInfo, error)
	GetTitleFunction func(LeaderboardParams) string
	GetQueryFunction func(LeaderboardParams) string
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
	BoardInfo       LeaderboardConfig
	Global          bool
	Players         map[int]PlayerInfo
	FishTypes       map[string]string
	Catchtypenames  map[string]string
}

type PlayerInfo struct {
	CurrentName string
	TwitchID    int
	Verified    bool
	Date        time.Time
}

func Leaderboards(pool *pgxpool.Pool, leaderboards string, chatNames string, date string, date2 string, path string, title string, limit string, mode string) {

	config := utils.LoadConfig()

	// The date arguments for the leaderboards
	// The date format for the leaderboards is YYYY-MM-DD
	// but can also be YYYY-MM-DD HH:MM:SS
	// date can also be "today"
	// date and date2 are both > / < (unless specified else in the board)
	if date == "" {
		currentDate := time.Now().UTC()
		nextDay := currentDate.AddDate(0, 0, 1)
		date = nextDay.Format("2006-01-02")
	} else {
		if date == "today" {
			date = time.Now().UTC().Format("2006-01-02")
		} else {
			if !isValidDate(date) {
				logs.Logs().Error().
					Str("Date", date).
					Str("Format", "YYYY-MM-DD (HH:MM:SS)").
					Msg("Date is in the wrong format")
				return
			}
		}
	}

	if date2 == "" {
		// This is one day before the first ever fish was caught (on justlog)
		oldDate := time.Date(2022, 12, 3, 0, 0, 0, 0, time.UTC)
		date2 = oldDate.Format("2006-01-02")
	} else {
		if !isValidDate(date2) {
			logs.Logs().Error().
				Str("Date2", date2).
				Str("Format", "YYYY-MM-DD (HH:MM:SS)").
				Msg("Date2 is in the wrong format")
			return
		}
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
		Players:        make(map[int]PlayerInfo),
		FishTypes:      make(map[string]string),
	}

	// map of all the boards
	// averageweight, rare, stats, shiny are global only, dont need a chat specified, so if there is, they get skipped
	// i do -chats all -board chatboards and then -chats global -board globalboards on sunday, and -chats all -board tourney for tournaments later
	existingboards := map[string]LeaderboardConfig{
		"fishweek":   {hasGlobal: false, GlobalOnly: false, Tournament: true, Function: processFishweek},
		"trophy":     {hasGlobal: false, GlobalOnly: false, Tournament: true, Function: processTrophy},
		"records":    {hasGlobal: true, GlobalOnly: false, Tournament: false, Function: processChannelRecords},
		"uniquefish": {hasGlobal: true, GlobalOnly: false, Tournament: false, Function: processUniqueFish},

		"typefirst": {
			hasGlobal: true, GlobalOnly: false, Tournament: false, Function: processType,
			GetFunction: getTypeRecords, GetTitleFunction: typeBoardTitles, GetQueryFunction: typeBoardSql,
		},
		"typesmall": {
			hasGlobal: true, GlobalOnly: false, Tournament: false, Function: processType,
			GetFunction: getTypeRecords, GetTitleFunction: typeBoardTitles, GetQueryFunction: typeBoardSql,
		},
		"type": {hasGlobal: true, GlobalOnly: false, Tournament: false, Function: processType,
			GetFunction: getTypeRecords, GetTitleFunction: typeBoardTitles, GetQueryFunction: typeBoardSql,
		},

		"count":         {hasGlobal: true, GlobalOnly: false, Tournament: false, Function: processCount},
		"weight":        {hasGlobal: true, GlobalOnly: false, Tournament: false, Function: processWeight},
		"weight2":       {hasGlobal: true, GlobalOnly: false, Tournament: false, Function: processWeight2},
		"averageweight": {hasGlobal: true, GlobalOnly: true, Tournament: false, Function: processAverageWeight},
		"rare":          {hasGlobal: true, GlobalOnly: true, Tournament: false, Function: RunCountFishTypesGlobal},
		"chats":         {hasGlobal: true, GlobalOnly: true, Tournament: false, Function: RunChatStatsGlobal},
		"shiny":         {hasGlobal: true, GlobalOnly: true, Tournament: false, Function: processShinies},
		"profiles":      {hasGlobal: true, GlobalOnly: true, Tournament: false, Function: GetPlayerProfiles}}

	leaderboardList := strings.Split(leaderboards, ",")

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

				params.BoardInfo = board

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

			params.BoardInfo = board

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
