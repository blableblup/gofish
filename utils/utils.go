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
		if input == "y" {
			return true, nil
		} else if input == "n" {
			return false, nil
		} else {
			logs.Logs().Warn().Msgf("Invalid input '%s'. Use 'y' or 'n'", input)
		}
	}
}

func ParseDate(dateStr string) (time.Time, error) {
	// Parse the date string into a time.Time object
	date, err := time.Parse("2006-01-2 15:04:05", dateStr)
	if err != nil {
		date, err = time.Parse("2006-01-2", dateStr)
		if err != nil {
			return time.Time{}, err
		}
	}
	return date, nil
}

func RoundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}
