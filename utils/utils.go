package utils

import (
	"bufio"
	"math"
	"os"
	"strings"
	"time"

	"gofish/logs"
)

func Confirm(prompt string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	logs.Logs().Warn().Msg(prompt)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}
		input = strings.TrimSpace(strings.ToLower(input))
		switch input {
		case "y":
			return true, nil
		case "n":
			return false, nil
		default:
			logs.Logs().Warn().Msgf("Invalid input '%s'. Use 'y' or 'n'", input)
		}
	}
}

func ScanAndReturn() (string, error) {

	scanner := bufio.NewScanner(os.Stdin)

	scanner.Scan()
	err := scanner.Err()
	if err != nil {
		return "", err
	}
	response := scanner.Text()

	return response, nil
}

func ParseDate(dateStr string) (time.Time, error) {
	// Parse the date string into a time.Time object
	date, err := time.Parse("2006-01-2 15:04:05", dateStr)
	if err != nil {
		date, err = time.Parse("2006-01-2", dateStr)
		// this is only for the "chats" leaderboard to get the active fishers
		if err != nil {
			return time.Time{}, err
		}
	}
	return date, nil
}

func ParseDateInLoc(dateStr string, loc *time.Location) (time.Time, error) {
	// Parse the date string into a time.Time object with a location
	date, err := time.ParseInLocation("2006-01-2 15:04:05", dateStr, loc)
	if err != nil {
		return time.Time{}, err
	}
	return date, nil
}

func RoundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}
