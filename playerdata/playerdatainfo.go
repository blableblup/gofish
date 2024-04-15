package playerdata

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ReadCheaters reads cheaters from a text file
func ReadCheaters() []string {
	cheaters := make([]string, 0)

	// Get the directory of the source file
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)

	// Construct the path to the cheaters.txt file relative to the source file directory
	cheatersPath := filepath.Join(dir, "cheaters.txt")

	file, err := os.Open(cheatersPath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return cheaters
	}
	defer file.Close()

	// Read each line from the text file
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// Append the line to the list of cheaters after stripping any leading or trailing whitespace
		cheaters = append(cheaters, strings.TrimSpace(scanner.Text()))
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading file:", err)
		return cheaters
	}

	return cheaters
}

// ReadVerifiedPlayers reads verified players from a text file
func ReadVerifiedPlayers() []string {
	verifiedPlayers := make([]string, 0)

	// Get the directory of the source file
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)

	// Construct the path to the verified.txt file relative to the source file directory
	verifiedPath := filepath.Join(dir, "verified.txt")

	file, err := os.Open(verifiedPath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return verifiedPlayers
	}
	defer file.Close()

	// Read each line from the text file
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// Append the line to the list of verified players after stripping any leading or trailing whitespace
		verifiedPlayers = append(verifiedPlayers, strings.TrimSpace(scanner.Text()))
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading file:", err)
		return verifiedPlayers
	}

	return verifiedPlayers
}

// ReadRenamedChatters reads renamed chatters from a CSV file
func ReadRenamedChatters() map[string]string {
	renamedChatters := make(map[string]string)

	// Get the directory of the source file
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)

	// Construct the path to the renamed.csv file relative to the source file directory
	csvPath := filepath.Join(dir, "renamed.csv")

	file, err := os.Open(csvPath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return renamedChatters
	}
	defer file.Close()

	reader := csv.NewReader(file)
	for {
		row, err := reader.Read()
		if err != nil {
			break
		}
		if len(row) == 2 {
			oldPlayer := row[0]
			newPlayer := row[1]
			renamedChatters[oldPlayer] = newPlayer
		}
	}

	return renamedChatters
}
