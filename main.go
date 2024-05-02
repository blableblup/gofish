package main

// Made with chatgpt

import (
	"flag"
	"gofish/data"
	"gofish/leaderboards"
	"gofish/logs"
	"gofish/scripts"
)

func main() {
	numMonths := flag.Int("m", 1, "Number of past months")
	mode := flag.String("mm", "", "Modes are different for each program")
	chatNames := flag.String("s", "", "Comma-separated list of chat names")
	monthYear := flag.String("dt", "", "Specific month and year (yyyy/mm)")
	leaderboard := flag.String("l", "", "Comma-separated list of leaderboards")
	program := flag.String("p", "", "Program name: boards, data, trnm, logs, pattern")
	db := flag.String("db", "", "Database to update, fish (f) and tournament results (t)")
	renamePairs := flag.String("rename", "", "Comma-separated list of oldName:newName pairs")

	flag.Parse()

	if *program == "" {
		logs.Logs().Info().Msg("Usage: go run main.go -p boards [-s <chat names> <all> <global>] [-l <leaderboards>] [-mm <mode>]")
		logs.Logs().Info().Msg("Usage: go run main.go -p data [-db <database>] [-m <months>] [-dt <date>] [-mm <mode>]")
		logs.Logs().Info().Msg("Usage: If no month or time period is specified it checks the current month")
		logs.Logs().Info().Msg("Usage: go run main.go -p renamed [-rename <oldName:newName>]")
		return
	}

	if *mode != "" && !isValidModeForProgram(*program, *mode) {
		logs.Logs().Warn().Msg("Invalid mode specified for the program or the program doesn't have different modes")
		return
	}

	switch *program {
	case "boards":
		if *mode != "" {
			logs.Logs().Info().Msgf("Running %s program in mode '%s'...", *program, *mode)
		} else {
			logs.Logs().Info().Msgf("Running %s program...", *program)
		}
		leaderboards.Leaderboards(*leaderboard, *chatNames, *mode)
		// Modes: "check", only prints new / updated type and weight records

	case "data":
		if *mode != "" {
			logs.Logs().Info().Msgf("Running %s program in mode '%s'...", *program, *mode)
		} else {
			logs.Logs().Info().Msgf("Running %s program...", *program)
		}
		data.GetData(*chatNames, *db, *numMonths, *monthYear, *mode)
		// Modes: "a" for fishdatafetch.
		// Adds every fish caught to FishData instead of just the new ones and inserts the missing fish into the db.
		// Modes: "insertall" for tournamentdata.
		// Adds all the existing lines from the tournamentlogs to newResults, inserts the missing results into the db and then returns.

	case "pattern":
		logs.Logs().Info().Msgf("Running %s program...", *program)
		scripts.RunPattern()

	case "renamed":
		logs.Logs().Info().Msgf("Running %s program...", *program)
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
		logs.Logs().Info().Msgf("Running %s program...", *program)
		scripts.VerifiedPlayers()

	default:
		logs.Logs().Warn().Msg("Invalid program specified")
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
