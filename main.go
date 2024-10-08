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
	debug := flag.Bool("debug", false, "Debug some stuff")
	program := flag.String("p", "", "Program name: boards, data, renamed, verified, pattern")

	// For both boards and data
	mode := flag.String("mm", "", "Modes are different for each program")
	chatNames := flag.String("s", "", "Comma-separated list of chat names")

	// Flags for data program
	numMonths := flag.Int("m", 1, "Number of past months for url")
	db := flag.String("db", "", "Database to update: fish (f) and tournament results (t)")
	// This flag is also used for the boards as "date"
	monthYear := flag.String("dt", "", "Specific month and year for data (yyyy/mm). For the boards, this needs to be yyyy-mm-dd")

	// Flags for boards program
	title := flag.String("title", "", "Pass a custom title to the board.")
	leaderboard := flag.String("l", "", "Comma-separated list of leaderboards")
	date2 := flag.String("dt2", "", "Second date for the leaderboards. If you want to get boards for a time period")
	path := flag.String("path", "", "Give the board a custom name. But you should only do one board at a time with this. Else it will get overwritten.")

	// For the rename player script
	renamePairs := flag.String("rename", "", "Comma-separated list of oldName:newName pairs")

	flag.Parse()

	logs.InitializeLogger(*debug)

	if *mode != "" && !isValidModeForProgram(*program, *mode) {
		logs.Logs().Warn().
			Str("Mode", *mode).
			Str("Program", *program).
			Msg("Invalid mode specified")
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
		logs.Logs().Info().Msg("To run in debug mode, set -debug to true.")
		logs.Logs().Info().Msg("Mode 'check': For type,weight,fishweek boards. Only logs new or updated records")
		logs.Logs().Info().Msg("Mode 'a': For data. Adds every fish caught to FishData instead of just the new ones and inserts the missing fish into the db")
		return

	case "boards":
		logs.Logs().Info().
			Str("Boards", *leaderboard).
			Str("Chats", *chatNames).
			Str("Program", *program).
			Str("Date", *monthYear).
			Str("Date2", *date2).
			Str("Title", *title).
			Str("Mode", *mode).
			Str("Path", *path).
			Msg("Start")

		leaderboards.Leaderboards(*leaderboard, *chatNames, *monthYear, *date2, *path, *title, *mode)

	case "data":
		logs.Logs().Info().
			Int("Months", *numMonths).
			Str("Program", *program).
			Str("Chats", *chatNames).
			Str("Date", *monthYear).
			Str("Mode", *mode).
			Str("DB", *db).
			Msg("Start")

		data.GetData(*chatNames, *db, *numMonths, *monthYear, *mode)

	case "pattern":
		logs.Logs().Info().
			Str("Program", *program).
			Msg("Start")
		scripts.RunPattern()

	case "renamed":
		logs.Logs().Info().
			Str("Rename pairs", *renamePairs).
			Str("Program", *program).
			Msg("Start")
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

	case "renamedfish":
		logs.Logs().Info().
			Str("Rename pairs", *renamePairs).
			Str("Program", *program).
			Msg("Start")
		namePairs, err := scripts.ProcessRenamePairs(*renamePairs)
		if err != nil {
			logs.Logs().Error().Err(err).Msg("Error processing fishname rename pairs")
			return
		}
		err = scripts.UpdateFishNames(namePairs)
		if err != nil {
			logs.Logs().Error().Err(err).Msg("Error updating fish names")
			return
		}

	case "verified":
		logs.Logs().Info().
			Str("Program", *program).
			Msg("Start")
		scripts.VerifiedPlayers()

	case "updatetwitchids":
		logs.Logs().Info().
			Str("Program", *program).
			Str("Mode", *mode).
			Msg("Start")

		scripts.UpdateTwitchIDs(*mode)

	case "mergetwitchids":
		logs.Logs().Info().
			Str("Program", *program).
			Msg("Start")

		scripts.MergePlayers()

	default:
		logs.Logs().Warn().
			Str("Program", *program).
			Msg("Invalid program specified")
		return
	}
}

func isValidModeForProgram(program, mode string) bool {

	validModes := map[string]map[string]bool{
		"boards":          {"check": true},
		"updatetwitchids": {"ble": true},
		"data":            {"a": true},
	}

	// Check if the provided mode is valid for the specified program
	if validPrograms, exists := validModes[program]; exists {
		return validPrograms[mode]
	}
	return false
}
