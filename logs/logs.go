package logs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"gofish_test/other"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type URLSet struct {
	URLs        []string `json:"urls"`
	Logs        string   `json:"logs"`
	LogsHost    string   `json:"logs_host"`
	LogsHostOld string   `json:"logs_host_old"`
}

type Config struct {
	URLSets map[string]URLSet `json:"url_sets"`
}

// RunLogs runs the logs program with the provided setNames, numMonths, and monthYear.
func RunLogs(setNames string, numMonths int, monthYear string) {
	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		os.Exit(1)
	}

	// Construct the absolute path to the config file
	configFilePath := filepath.Join(wd, "config.json")

	// Load the config from the constructed file path
	config := loadConfig(configFilePath)

	// Check if the first argument is "all"
	if setNames == "all" {
		// Run all URL sets with the specified number of months or month/year
		for setName, setInfo := range config.URLSets {
			urls := other.CreateURL(setName, numMonths, monthYear)
			fetchMatchingLines(setInfo, urls)
		}
		return
	}

	// Loop through the setNames
	for _, setName := range strings.Split(setNames, ",") {
		// Check if the specified URL set exists
		setInfo, ok := config.URLSets[setName]
		if !ok {
			fmt.Printf("URL set '%s' not found\n", setName)
			continue
		}

		// Call runURLSet function with the provided arguments
		urls := other.CreateURL(setName, numMonths, monthYear)
		fetchMatchingLines(setInfo, urls)
	}
}

func fetchMatchingLines(setInfo URLSet, urls []string) {
	// Get the directory of the current source file
	_, currentFilePath, _, _ := runtime.Caller(0)
	currentFileDir := filepath.Dir(currentFilePath)

	// Construct the absolute path to the logs directory relative to the current source file
	logsDir := filepath.Join(currentFileDir, "")

	// Check if the log file path is absolute
	var logFilePath string
	if filepath.IsAbs(setInfo.Logs) {
		logFilePath = setInfo.Logs
	} else {
		// Construct the absolute path to the log file
		logFilePath = filepath.Join(logsDir, setInfo.Logs)
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

	// Read existing content from the output file
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
		fmt.Printf("New results appended to %s\n", setInfo.Logs)
	} else {
		fmt.Printf("No new results to append to %s\n", setInfo.Logs)
	}
}

func loadConfig(filename string) Config {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("Error opening config file:", err)
		os.Exit(1)
	}
	defer file.Close()

	var config Config
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		fmt.Println("Error parsing config file:", err)
		os.Exit(1)
	}

	return config
}
