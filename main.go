package main

// Made with chatgpt

import (
	"flag"
	"fmt"
	"gofish/data"
	"gofish/leaderboards"
	"gofish/scripts"
)

func main() {
	// Define command line flags
	program := flag.String("p", "", "Program name: trnm, wght, logs, count, global,pattern")
	chatNames := flag.String("s", "", "Comma-separated list of chat names")
	leaderboard := flag.String("l", "", "Comma separated list of leaderboards")
	mode := flag.String("mm", "", "Modes are different for each program")
	numMonths := flag.Int("m", 1, "Number of past months")
	monthYear := flag.String("dt", "", "Specific month and year (yyyy/mm)")
	db := flag.String("db", "", "Database to update, fish (f) and tournament results (t)")
	renamePairs := flag.String("rename", "", "Comma-separated list of oldName:newName pairs")

	// Parse command line flags
	flag.Parse()

	// Validate program name
	if *program == "" {
		fmt.Println("Usage: go run main.go -p data [-db <database>] [-m <months>] [-d <date>] [-m <mode>]")
		// If no month or time period is specified it checks the current month
		fmt.Println("Usage: go run main.go -p boards [-s <chat names>] [-l <leaderboards>] [-m <mode>]")
		fmt.Println("Usage: go run main.go -p renamed [-rename <oldName:newName>]")
		fmt.Println("Usage: go run main.go -p global [-l <leaderboards>]")
		return
	}

	// Validate mode name for the specified program
	if *mode != "" && !isValidModeForProgram(*program, *mode) {
		fmt.Println("Invalid mode specified for the program or the program doesn't have different modes.")
		return
	}

	// Call the appropriate function based on the program name
	switch *program {
	case "boards":
		fmt.Printf("Running %s program...\n", *program)
		leaderboards.Leaderboards(*leaderboard, *chatNames, *mode)

	case "global":
		fmt.Printf("Running %s program...\n", *program)
		leaderboards.RunGlobal(*leaderboard)

	case "logs":
		fmt.Printf("Running %s program...\n", *program)
		data.RunLogs(*chatNames, *numMonths, *monthYear)

	case "data":
		fmt.Printf("Running %s program", *program)
		if *mode != "" {
			fmt.Printf(" in mode '%s'", *mode)
		}
		fmt.Println("...")
		data.GetData(*chatNames, *db, *numMonths, *monthYear, *mode)
		// Modes: "a", this adds every fish caught to FishData instead of just the new ones.
		// Useful for if there is a new catchtype and the database was already updated.

	case "pattern":
		fmt.Printf("Running %s program...\n", *program)
		scripts.RunPattern()

	case "renamed":
		fmt.Printf("Running %s program...\n", *program)
		namePairs, err := scripts.ProcessRenamePairs(*renamePairs)
		if err != nil {
			fmt.Println("Error processing rename pairs:", err)
			return
		}
		err = scripts.UpdatePlayerNames(namePairs)
		if err != nil {
			fmt.Println("Error updating player names:", err)
			return
		}

	case "verified":
		fmt.Printf("Running %s program...\n", *program)
		scripts.VerifiedPlayers()

	default:
		fmt.Println("Invalid program specified.")
		return
	}
}

// Function to validate mode name for the specified program
func isValidModeForProgram(program, mode string) bool {
	// Define valid modes for each program
	validModes := map[string]map[string]bool{
		"wght":   {"c": true},
		"boards": {"c": true},
		"data":   {"a": true},
	}

	// Check if the provided mode is valid for the specified program
	if validPrograms, exists := validModes[program]; exists {
		return validPrograms[mode]
	}
	return false
}
