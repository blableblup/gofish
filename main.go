package main

// Made with chatgpt

import (
	"flag"
	"fmt"
	"gofish/data"
	"gofish/leaderboards"
)

func main() {
	// Define command line flags
	program := flag.String("p", "", "Program name: trnm, wght, logs, count, global")
	chatNames := flag.String("s", "", "Comma-separated list of chat names")
	leaderboard := flag.String("l", "", "Leaderboard name")
	mode := flag.String("mm", "", "Modes are different for each program")
	numMonths := flag.Int("m", 1, "Number of past months")
	monthYear := flag.String("d", "", "Specific month and year (yyyy/mm)")

	// Parse command line flags
	flag.Parse()

	// Validate program name
	if *program == "" {
		fmt.Println("Usage: go run main.go -p <program> [-s <chat names>] [-l <leaderboard>] [-m <months>] [-d <date>] [-m <mode>]")
		// If no leaderboard is specified it updates all available leaderboards of the program (for the global program a leaderboard has to be specified)
		// If no month or time period is specified it checks the current month
		return
	}

	// Validate leaderboard name for the specified program
	if *leaderboard != "" && !isValidLeaderboardForProgram(*program, *leaderboard) {
		fmt.Println("Invalid leaderboard specified for the program or the program doesn't have a leaderboard.")
		return
	}

	// Validate mode name for the specified program
	if *mode != "" && !isValidModeForProgram(*program, *mode) {
		fmt.Println("Invalid mode specified for the program or the program doesn't have different modes.")
		return
	}

	// Call the appropriate function based on the program name
	switch *program {
	case "global":
		fmt.Println("Running global program...")
		leaderboards.RunGlobal(*leaderboard)

	case "trnm":
		fmt.Println("Running tournaments program...")
		leaderboards.RunTournaments(*chatNames, *leaderboard)

	case "wght":
		fmt.Print("Running typeweight program")
		if *mode != "" {
			fmt.Printf(" in mode '%s'", *mode)
		}
		fmt.Println("...")
		leaderboards.RunTypeWeight(*chatNames, *leaderboard, *numMonths, *monthYear, *mode)
		// Modes: "c", only prints new/updated records

	case "count":
		fmt.Println("Running totalcount program...")
		leaderboards.RunTotalcount(*chatNames, *leaderboard, *numMonths, *monthYear)

	case "logs":
		fmt.Println("Running logs program...")
		data.RunLogs(*chatNames, *numMonths, *monthYear)

	// case "test1":
	// 	fmt.Println("Running test1 program...")
	// 	other.RunTest1()

	default:
		fmt.Println("Invalid program specified.")
		return
	}
}

// Function to validate leaderboard name for the specified program
func isValidLeaderboardForProgram(program, leaderboard string) bool {
	// Define valid leaderboards for each program
	validLeaderboards := map[string]map[string]bool{
		"trnm":   {"trophy": true, "fishw": true, "": true},
		"wght":   {"weight": true, "type": true, "": true},
		"count":  {"count": true, "": true},
		"global": {"weight": true, "type": true, "all": true},
	}

	// Check if the provided leaderboard is valid for the specified program
	if validPrograms, exists := validLeaderboards[program]; exists {
		return validPrograms[leaderboard]
	}
	return false
}

// Function to validate mode name for the specified program
func isValidModeForProgram(program, mode string) bool {
	// Define valid modes for each program
	validModes := map[string]map[string]bool{
		"wght": {"c": true},
	}

	// Check if the provided mode is valid for the specified program
	if validPrograms, exists := validModes[program]; exists {
		return validPrograms[mode]
	}
	return false
}
