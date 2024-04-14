package utils

import (
	"fmt"
	"sort"
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

// Define a mapping for equivalent fish types
var equivalentFishTypes = map[string]string{
	"🕷":          "🕷️",
	"🗡":          "🗡️",
	"🕶":          "🕶️",
	"☂":          "☂️",
	"⛸":          "⛸️",
	"🧜♀":         "🧜‍♀️",
	"🧜♀️":        "🧜‍♀️",
	"🧜‍♀":        "🧜‍♀️",
	"🐻‍❄️":       "🐻‍❄",
	"🧞‍♂️":       "🧞‍♂",
	"Jellyfish":  "🪼",
	"Jellyfish ": "🪼",
	"HailHelix ": "🐚", // Shinies need to have one space behind them >-< or idk
	"HailHelix":  "🐚",
	"SabaPing ":  "🐟",
}

// EquivalentFishType checks if the current fish type is in the list of equivalent fish types
// and returns the corresponding equivalent fish type if it exists.
func EquivalentFishType(fishType string) string {
	equivalent, ok := equivalentFishTypes[fishType]
	if ok {
		return equivalent
	}
	return fishType // Return the original fish type if no equivalent is found
}

func Ranks(rank int) string {
	var ranks string

	switch rank {
	case 1:
		ranks = fmt.Sprintf("%d 🥇", rank)
	case 2:
		ranks = fmt.Sprintf("%d 🥈", rank)
	case 3:
		ranks = fmt.Sprintf("%d 🥉", rank)
	default:
		ranks = fmt.Sprintf("%d", rank)
	}

	return ranks
}

func ChangeEmoji(rank int, oldRank int, found bool) string {
	var changeEmoji string

	if found {
		if rank < oldRank {
			changeEmoji = "⬆"
		} else if rank > oldRank {
			changeEmoji = "⬇"
		} else {
			changeEmoji = ""
		}
	} else {
		changeEmoji = "🆕"
	}

	return changeEmoji
}
