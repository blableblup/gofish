package main

import (
	"flag"
	"fmt"
	"gofish/leaderboards"
	"gofish/logs"
)

func main() {
	// Define command line flags
	program := flag.String("p", "", "Program name: trnm, wght, logs, count")
	setNames := flag.String("s", "", "Comma-separated list of set names")
	leaderboard := flag.String("l", "", "Leaderboard name")
	numMonths := flag.Int("m", 1, "Number of past months")
	monthYear := flag.String("d", "", "Specific month and year (yyyy/mm)")

	// Parse command line flags
	flag.Parse()

	// Validate program name
	if *program == "" {
		fmt.Println("Usage: go run main.go -p <program> [-s <set names>] [-l <leaderboard>] [-m <months>] [-d <date>]")
		// If no leaderboard is specified it updates all available leaderboards of the program
		// If no month or time period is specified it checks the current month
		return
	}

	// Validate leaderboard name for the specified program
	if *leaderboard != "" && !isValidLeaderboardForProgram(*program, *leaderboard) {
		fmt.Println("Invalid leaderboard specified for the program or the program doesn't have a leaderboard.")
		return
	}

	// Call the appropriate function based on the program name
	switch *program {
	case "trnm":
		fmt.Println("Running tournaments program...")
		leaderboards.RunTournaments(*setNames, *leaderboard)

	case "wght":
		fmt.Println("Running typeweight program...")
		leaderboards.RunTypeWeight(*setNames, *leaderboard, *numMonths, *monthYear)

	case "count":
		fmt.Println("Running totalcount program...")
		leaderboards.RunTotalcount(*setNames, *leaderboard, *numMonths, *monthYear)

	case "logs":
		fmt.Println("Running logs program...")
		logs.RunLogs(*setNames, *numMonths, *monthYear)

	default:
		fmt.Println("Invalid program specified.")
		return
	}
}

// Function to validate leaderboard name for the specified program
func isValidLeaderboardForProgram(program, leaderboard string) bool {
	// Define valid leaderboards for each program
	validLeaderboards := map[string]map[string]bool{
		"trnm":  {"trophy": true, "fishw": true, "": true}, // Valid leaderboards for trnm program
		"wght":  {"weight": true, "type": true, "": true},  // Valid leaderboards for wght program
		"count": {"count": true, "": true},                 // Valid leaderboards for count program
	}

	// Check if the provided leaderboard is valid for the specified program
	if validPrograms, exists := validLeaderboards[program]; exists {
		return validPrograms[leaderboard]
	}
	return false
}
