package leaderboards

import (
	"gofish/logs"
	"gofish/utils"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// struct to scan all the board data into
type BoardData struct {
	// for the player
	ProfileLink string `json:"profile_link,omitempty"`
	Player      string `json:"player,omitempty"`
	PlayerID    int    `json:"playerid,omitempty"`
	TwitchID    int    `json:"twitchid,omitempty"`
	Verified    bool   `json:"verified,omitempty"`

	// for fishes
	Weight    float64   `json:"weight,omitempty"`
	Bot       string    `json:"bot,omitempty"`
	FishType  string    `json:"fishtype,omitempty"`
	FishName  string    `json:"fishname,omitempty"`
	CatchType string    `json:"catchtype,omitempty"`
	Chat      string    `json:"chat,omitempty"`
	ChatPfp   string    `json:"chatpfp,omitempty"`
	FishId    int       `json:"fishid,omitempty"`
	ChatId    int       `json:"chatid,omitempty"`
	Date      time.Time `json:"date,omitempty"`

	// for mouth fishes
	WeightMouth   float64 `json:"weightmouth,omitempty"`
	FishTypeMouth string  `json:"fishtypemouth,omitempty"`
	FishNameMouth string  `json:"fishnamemouth,omitempty"`

	TotalWeight float64 `json:"totalweight,omitempty"`

	// idk
	Count       int                `json:"count,omitempty"`
	ChatCounts  map[string]int     `json:"chatcounts,omitempty"`
	ChatWeights map[string]float64 `json:"chatweights,omitempty"`

	// for chats board
	ActiveFishers int `json:"activefishers,omitempty"`
	UniqueFishers int `json:"uniquefishers,omitempty"`
	UniqueFish    int `json:"uniquefish,omitempty"`

	// for trophy board
	Trophies int `json:"trophies,omitempty"`
	Silver   int `json:"silver,omitempty"`
	Bronze   int `json:"bronze,omitempty"`

	Rank int `json:"rank,omitempty"`
}

// to store info about a leaderboard and its functions
type LeaderboardConfig struct {
	Name       string
	hasGlobal  bool
	GlobalOnly bool
	Tournament bool

	Function func(LeaderboardParams)

	GetFunction      func(LeaderboardParams) (map[string]BoardData, error)
	GetFunctionInt   func(LeaderboardParams, int) (map[int]BoardData, error)
	GetTitleFunction func(LeaderboardParams) string
	GetQueryFunction func(LeaderboardParams) string
	// a map, because some boards need multiple queries
	GetQueryFunctionMap func(LeaderboardParams) map[string]string

	// to not update profiles with all the other boards
	// profiles should be run after them because that is using the json data from the other boards
	// if run with the other boards, the data for the board records will be old and wrong
	IsProfiles bool
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
	existingboards := map[string]LeaderboardConfig{
		"trophy": {
			hasGlobal:  false,
			GlobalOnly: false,
			Tournament: true,
			Function:   processTrophy,
		},

		"fishweek": {
			hasGlobal:           false,
			GlobalOnly:          false,
			Tournament:          true,
			Function:            processCount,
			GetFunctionInt:      getCount,
			GetTitleFunction:    countBoardTitles,
			GetQueryFunctionMap: countBoardSql,
		},
		"uniquefish": {
			hasGlobal:           true,
			GlobalOnly:          false,
			Tournament:          false,
			Function:            processCount,
			GetFunctionInt:      getCount,
			GetTitleFunction:    countBoardTitles,
			GetQueryFunctionMap: countBoardSql,
		},
		"count": {
			hasGlobal:           true,
			GlobalOnly:          false,
			Tournament:          false,
			Function:            processCount,
			GetFunctionInt:      getCount,
			GetTitleFunction:    countBoardTitles,
			GetQueryFunctionMap: countBoardSql,
		},

		"typelast": {
			hasGlobal:        true,
			GlobalOnly:       false,
			Tournament:       false,
			Function:         processType,
			GetFunction:      getTypeRecords,
			GetTitleFunction: typeBoardTitles,
			GetQueryFunction: typeBoardSql,
		},
		"typefirst": {
			hasGlobal:        true,
			GlobalOnly:       false,
			Tournament:       false,
			Function:         processType,
			GetFunction:      getTypeRecords,
			GetTitleFunction: typeBoardTitles,
			GetQueryFunction: typeBoardSql,
		},
		"typesmall": {
			hasGlobal:        true,
			GlobalOnly:       false,
			Tournament:       false,
			Function:         processType,
			GetFunction:      getTypeRecords,
			GetTitleFunction: typeBoardTitles,
			GetQueryFunction: typeBoardSql,
		},
		"type": {
			hasGlobal:        true,
			GlobalOnly:       false,
			Tournament:       false,
			Function:         processType,
			GetFunction:      getTypeRecords,
			GetTitleFunction: typeBoardTitles,
			GetQueryFunction: typeBoardSql,
		},

		"records": {
			hasGlobal:  true,
			GlobalOnly: false,
			Tournament: false,
			Function:   processChannelRecords,
		},

		"weight": {
			hasGlobal:  true,
			GlobalOnly: false,
			Tournament: false,
			Function:   processWeight,
		},
		"weight2": {
			hasGlobal:  true,
			GlobalOnly: false,
			Tournament: false,
			Function:   processWeight2,
		},
		"weighttotal": {
			hasGlobal:  true,
			GlobalOnly: false,
			Tournament: false,
			Function:   processWeightTotal,
		},
		"weightmouth": {
			hasGlobal:  true,
			GlobalOnly: true,
			Tournament: false,
			Function:   processWeightMouth,
		},

		"averageweight": {
			hasGlobal:  true,
			GlobalOnly: true,
			Tournament: false,
			Function:   processAverageWeight,
		},
		"rare": {
			hasGlobal:  true,
			GlobalOnly: true,
			Tournament: false,
			Function:   RunCountFishTypesGlobal,
		},
		"chats": {
			hasGlobal:  true,
			GlobalOnly: true,
			Tournament: false,
			Function:   RunChatStatsGlobal,
		},
		"shiny": {
			hasGlobal:  true,
			GlobalOnly: true,
			Tournament: false,
			Function:   processShinies,
		},

		"profiles": {
			hasGlobal:  true,
			GlobalOnly: true,
			Tournament: false,
			Function:   GetPlayerProfiles,

			IsProfiles: true,
		},
		"wrapped": {
			hasGlobal:  true,
			GlobalOnly: true,
			Tournament: false,
			Function:   GetWrapped,

			IsProfiles: true,
		}}

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
				if board.GlobalOnly && !board.IsProfiles {
					processLeaderboard(config, params, boardname, board)
				}
			}

		case "all":
			for boardname, board := range existingboards {

				if !board.IsProfiles {
					processLeaderboard(config, params, boardname, board)
				}
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
