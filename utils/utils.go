package utils

import (
	"time"
)

// Function to check if a slice contains a specific string
func Contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
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
