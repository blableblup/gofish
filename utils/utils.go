package utils

import (
	"sort"
	"time"
)

// Function to sort a map by its values in descending order (int version)
func SortMapByValueDescInt(m map[string]int) []string {
	// Create a slice of keys from the map
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	// Sort the keys based on the corresponding values in the map
	sort.Slice(keys, func(i, j int) bool {
		return m[keys[i]] > m[keys[j]]
	})

	return keys
}

// Function to sort a map by its values in descending order
func SortMapByValueDesc(m map[string]float64) []string {
	// Create a slice of keys from the map
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	// Sort the keys based on the corresponding values in the map
	sort.Slice(keys, func(i, j int) bool {
		return m[keys[i]] > m[keys[j]]
	})

	return keys
}

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
		return time.Time{}, err
	}
	return date, nil
}
