package main

// Originally made with chatgpt

import (
	"flag"
	"gofish/data"
	"gofish/leaderboards"
	"gofish/logs"
	"gofish/scripts"
)

func main() {
	// For the logger
	debug := flag.Bool("debug", false, "Debug some stuff")
	multi := flag.Bool("multi", false, "To also log to a file")
	pathlog := flag.String("pathlog", "", "If multi is selected, can give the file a different name")

	program := flag.String("p", "", "Program name: boards, data, renamed, verified, pattern")
	database := flag.String("database", "default", "What databse to connect to. Connects to whatever is named 'default' in sqlconfig.json by default.")

	// For both boards and data
	mode := flag.String("mode", "", "Modes are different for each program")
	chatNames := flag.String("chats", "", "Comma-separated list of chat names")

	// Flags for data program
	numMonths := flag.Int("months", 1, "Number of past months for url")
	db := flag.String("db", "", "Database to update: fish (f) and tournament results (t)")
	// To select another justlog instance for a chat if it has one
	logInstance := flag.String("instance", "0", "Can select another justlog instance for a chat")
	// This flag is also used for the boards as "date"
	monthYear := flag.String("dt", "", "Specific month and year for data (yyyy/mm). For the boards, this needs to be yyyy-mm-dd")

	// Flags for boards program
	title := flag.String("title", "", "Pass a custom title to the board.")
	limit := flag.String("limit", "", "Custom weight/count limit for the boards")
	leaderboard := flag.String("board", "", "Comma-separated list of leaderboards")
	date2 := flag.String("dt2", "", "Second date for the leaderboards. If you want to get boards for a time period")
	path := flag.String("path", "", "Give the board a custom name. But you should only do one board at a time with this. Else it will get overwritten.")

	// For the rename player script
	renamePairs := flag.String("rename", "", "Comma-separated list of oldName:newName pairs")

	flag.Parse()

	logs.InitializeLogger(*debug, *multi, *pathlog)

	// Connect to the selected database
	pool, err := data.Connect(*database)
	if err != nil {
		logs.Logs().Error().Err(err).
			Msg("Error connecting to the database")
		return
	}
	defer pool.Close()

	if *mode != "" && !isValidModeForProgram(*program, *mode) {
		logs.Logs().Warn().
			Str("Mode", *mode).
			Str("Program", *program).
			Msg("Invalid mode specified")
		return
	}

	switch *program {
	case "":
		logs.Logs().Warn().Msg("Missing -p !")
		return

	case "boards":
		logs.Logs().Info().
			Str("Boards", *leaderboard).
			Str("Database", *database).
			Str("Chats", *chatNames).
			Str("Program", *program).
			Str("Date", *monthYear).
			Str("Date2", *date2).
			Str("Limit", *limit).
			Str("Title", *title).
			Str("Mode", *mode).
			Str("Path", *path).
			Msg("Start")

		leaderboards.Leaderboards(pool, *leaderboard, *chatNames, *monthYear, *date2, *path, *title, *limit, *mode)

	case "data":
		logs.Logs().Info().
			Str("LogInstance", *logInstance).
			Str("Database", *database).
			Int("Months", *numMonths).
			Str("Program", *program).
			Str("Chats", *chatNames).
			Str("Date", *monthYear).
			Str("Mode", *mode).
			Str("DB", *db).
			Msg("Start")

		data.GetData(pool, *chatNames, *db, *numMonths, *monthYear, *logInstance, *mode)

	case "renamedfish":
		logs.Logs().Info().
			Str("Rename pairs", *renamePairs).
			Str("Database", *database).
			Str("Program", *program).
			Msg("Start")
		namePairs, err := scripts.ProcessRenamePairs(*renamePairs)
		if err != nil {
			logs.Logs().Error().Err(err).Msg("Error processing fishname rename pairs")
			return
		}
		err = scripts.UpdateFishNames(pool, namePairs)
		if err != nil {
			logs.Logs().Error().Err(err).Msg("Error updating fish names")
			return
		}

	case "verified":
		logs.Logs().Info().
			Str("Database", *database).
			Str("Program", *program).
			Msg("Start")
		scripts.VerifiedPlayers(pool)

	case "updatetwitchids":
		logs.Logs().Info().
			Str("Database", *database).
			Str("Program", *program).
			Str("Mode", *mode).
			Msg("Start")

		scripts.UpdateTwitchIDs(pool, *mode)

	case "mergetwitchids":
		logs.Logs().Info().
			Str("Program", *program).
			Msg("Start")

		scripts.MergePlayers(pool)

	case "pfps":
		logs.Logs().Info().
			Str("Program", *program).
			Str("Mode", *mode).
			Msg("Start")

		scripts.GetTwitchPFPs(*mode)

	default:
		logs.Logs().Warn().
			Str("Program", *program).
			Msg("Invalid program specified")
		return
	}
}

func isValidModeForProgram(program, mode string) bool {

	validModes := map[string]map[string]bool{
		"boards": {"check": true, "force": true},
		// "check" Only prints updated/new records for the weight/weight2 and smallest/biggest fish per type boards
		// "force" forces the leaderboard to update, even if there are no changes on it
		"updatetwitchids": {"ble": true},
		// Mode ble is updating the twitch ids for all players, even if they already have one
		// This could mess up data, by updating someones twitchid because someone else is now using their name
		"data": {"a": true},
		// "a" makes it so every single catch/bag/result is manually checked if it exists in the db
		// usually, you get the highest date for each chat in the db and then add all fish which come after that
		"pfps": {"all": true},
		// all gets the pfps for all the chats in the config even if they already have a file named after them in /images/players
	}

	// Check if the provided mode is valid for the specified program
	if validPrograms, exists := validModes[program]; exists {
		return validPrograms[mode]
	}
	return false
}
