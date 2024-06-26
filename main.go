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
	numMonths := flag.Int("m", 1, "Number of past months")
	debug := flag.Bool("debug", false, "Debug some stuff")
	mode := flag.String("mm", "", "Modes are different for each program")
	chatNames := flag.String("s", "", "Comma-separated list of chat names")
	leaderboard := flag.String("l", "", "Comma-separated list of leaderboards")
	program := flag.String("p", "", "Program name: boards, data, trnm, logs, pattern")
	db := flag.String("db", "", "Database to update, fish (f) and tournament results (t)")
	renamePairs := flag.String("rename", "", "Comma-separated list of oldName:newName pairs")
	date2 := flag.String("dt2", "", "Second date for the leaderboards. If you want to get boards for a time period")
	monthYear := flag.String("dt", "", "Specific month and year for data (yyyy/mm). For the boards, this needs to be yyyy-mm-dd")
	path := flag.String("path", "", "Give the board a custom name. But you should only do one board at a time with this. Else it will get overwritten.")

	flag.Parse()

	logs.InitializeLogger(*debug)

	if *mode != "" && !isValidModeForProgram(*program, *mode) {
		logs.Logs().Warn().Str("Program", *program).Str("Mode", *mode).Msg("Invalid mode specified")
		return
	}

	switch *program {
	case "":
		logs.Logs().Warn().Msg("No program specified. Use '-p help' for help")
		return

	case "help":
		logs.Logs().Info().Msg("Leaderboards: fishweek, trophy, weight, type, count, rare, stats. Global boards: weight, type, count")
		logs.Logs().Info().Msg("Usage: -p boards [-s <chat names> <all> <global>] [-l <leaderboards>] [-mm <mode>]")
		logs.Logs().Info().Msg("Usage: -p data [-db <database>] [-m <months>] [-dt <date>] [-mm <mode>]")
		logs.Logs().Info().Msg("Usage: If no month or time period is specified it checks the current month")
		logs.Logs().Info().Msg("Usage: -p renamed [-rename <oldName:newName>]")
		return

	case "boards":
		logs.Logs().Info().Str("Program", *program).Str("Mode", *mode).Str("Boards", *leaderboard).Str("Chats", *chatNames).Str("Path", *path).Str("Date", *monthYear).Str("Date2", *date2).Msg("Start")

		leaderboards.Leaderboards(*leaderboard, *chatNames, *monthYear, *date2, *path, *mode)
		// Modes: "check", only prints new / updated type and weight records

	case "data":
		logs.Logs().Info().Str("Program", *program).Str("Mode", *mode).Str("Chats", *chatNames).Str("DB", *db).Int("Months", *numMonths).Str("Date", *monthYear).Msg("Start")

		data.GetData(*chatNames, *db, *numMonths, *monthYear, *mode)
		// Modes: "a" for fishdatafetch.
		// Adds every fish caught to FishData instead of just the new ones and inserts the missing fish into the db.

	case "pattern":
		logs.Logs().Info().Str("Program", *program).Msg("Start")
		scripts.RunPattern()

	case "renamed":
		logs.Logs().Info().Str("Rename pairs", *renamePairs).Str("Program", *program).Msg("Start")
		namePairs, err := scripts.ProcessRenamePairs(*renamePairs)
		if err != nil {
			logs.Logs().Error().Err(err).Msg("Error processing rename pairs")
			return
		}
		err = scripts.UpdatePlayerNames(namePairs)
		if err != nil {
			logs.Logs().Error().Err(err).Msg("Error updating player names")
			return
		}

	case "verified":
		logs.Logs().Info().Str("Program", *program).Msg("Start")
		scripts.VerifiedPlayers()

	default:
		logs.Logs().Warn().Str("Program", *program).Msg("Invalid program specified")
		return
	}
}

func isValidModeForProgram(program, mode string) bool {

	validModes := map[string]map[string]bool{
		"boards": {"check": true},
		"data":   {"a": true, "insertall": true},
	}

	// Check if the provided mode is valid for the specified program
	if validPrograms, exists := validModes[program]; exists {
		return validPrograms[mode]
	}
	return false
}
