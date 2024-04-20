package leaderboards

import (
	"fmt"
	"gofish/data"
	"sort"
)

func SortMapByCountDesc(fishCaught map[string]data.FishInfo) []string {
	// Create a slice of player names
	players := make([]string, 0, len(fishCaught))
	for player := range fishCaught {
		players = append(players, player)
	}

	// Sort the slice based on the count of fish caught
	sort.Slice(players, func(i, j int) bool {
		return fishCaught[players[i]].Count > fishCaught[players[j]].Count
	})

	return players
}

func SortMapByWeightDesc(fishCaught map[string]data.FishInfo) []string {
	// Create a slice of player names
	players := make([]string, 0, len(fishCaught))
	for player := range fishCaught {
		players = append(players, player)
	}

	// Sort the slice based on the weight of fish caught
	sort.Slice(players, func(i, j int) bool {
		return fishCaught[players[i]].Weight > fishCaught[players[j]].Weight
	})

	return players
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

func ConvertToFishInfo(info LeaderboardInfo) data.FishInfo {
	return data.FishInfo{
		Weight: info.Weight,
		Type:   info.Type,
		Bot:    info.Bot,
		Player: info.Player,
	}
}
