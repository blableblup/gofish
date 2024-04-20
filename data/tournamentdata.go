package data

import (
	"bufio"
	"fmt"
	"gofish/utils"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func GetTournamentData(config utils.Config, chatNames string, numMonths int, monthYear string) {

	switch chatNames {
	case "all":
		for chatName, chat := range config.Chat {
			if !chat.CheckEnabled {
				if chatName != "global" {
					fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
				}
				continue
			}

			fmt.Printf("Checking tournament results for chat '%s'.\n", chatName)
			urls := utils.CreateURL(chatName, numMonths, monthYear)
			fetchMatchingLines(chatName, urls)
		}
	case "":
		fmt.Println("Please specify chat names.")
	default:
		specifiedchatNames := strings.Split(chatNames, ",")
		for _, chatName := range specifiedchatNames {
			chat, ok := config.Chat[chatName]
			if !ok {
				fmt.Printf("Chat '%s' not found in config.\n", chatName)
				continue
			}
			if !chat.CheckEnabled {
				if chatName != "global" {
					fmt.Printf("Skipping chat '%s' because check_enabled is false.\n", chatName)
				}
				continue
			}

			fmt.Printf("Checking tournament results for chat '%s'.\n", chatName)
			urls := utils.CreateURL(chatName, numMonths, monthYear)
			fetchMatchingLines(chatName, urls)
		}
	}
}

func fetchMatchingLines(chatName string, urls []string) {

	logFilePath := filepath.Join("data", chatName, "tournamentlogs.txt")

	if err := os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
		fmt.Println("Error creating directory:", err)
		return
	}

	// Fetch matching lines from each URL
	matchingLines := make([]string, 0)
	for _, url := range urls {
		response, err := http.Get(url)
		if err != nil {
			fmt.Println("Error fetching URL:", err)
			continue
		}
		defer response.Body.Close()

		body, err := io.ReadAll(response.Body)
		if err != nil {
			fmt.Println("Error reading response body:", err)
			continue
		}

		lines := strings.Split(string(body), "\n")
		for _, line := range lines {
			if strings.Contains(line, "The results are in") ||
				strings.Contains(line, "The results for last week are in") ||
				strings.Contains(line, "Last week...") {
				matchingLines = append(matchingLines, strings.TrimSpace(line))
			}
		}
		fmt.Println("Finished checking for matching lines in", url)
	}

	// Ensure directory exists
	err := os.MkdirAll(filepath.Dir(logFilePath), 0755)
	if err != nil {
		fmt.Println("Error creating folder:", err)
		return
	}

	file, err := os.OpenFile(logFilePath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		return
	}
	defer file.Close()

	existingLines := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "You caught") {
			parts := strings.SplitN(line, "You caught", 2)
			if len(parts) > 1 {
				existingLines[strings.TrimSpace(parts[1])] = struct{}{}
			}
		}
	}

	// Extract and compare new results to ensure uniqueness
	newResults := make([]string, 0)
	for _, line := range matchingLines {
		if strings.Contains(line, "You caught") {
			parts := strings.SplitN(line, "You caught", 2)
			if len(parts) > 1 {
				newLine := strings.TrimSpace(parts[1])
				if _, exists := existingLines[newLine]; !exists {
					newResults = append(newResults, line)
					existingLines[newLine] = struct{}{} // Update existing lines with new ones
				}
			}
		}
	}

	// Append only the unique new results to the output file
	if len(newResults) > 0 {
		file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println("Error opening log file for appending:", err)
			return
		}
		defer file.Close()

		for _, line := range newResults {
			if _, err := file.WriteString(line + "\n"); err != nil {
				fmt.Println("Error appending to log file:", err)
				return
			}
		}
		fmt.Printf("New results appended to %s\n", logFilePath)
	} else {
		fmt.Printf("No new results to append to %s\n", logFilePath)
	}
}
