package leaderboards

import (
	"fmt"
	"gofish/data"
	"gofish/logs"
	"sort"
	"time"
)

func logRecord(newRecords map[string]data.FishInfo, oldRecords map[string]LeaderboardInfo, board string) {

	// Log new or updated records for the (global) type and weight leaderboards
	for playerORfish, newRecord := range newRecords {
		oldRecord, exists := oldRecords[playerORfish]
		if !exists {
			logs.Logs().Info().
				Str("Date", newRecord.Date.Format(time.RFC3339)).
				Str("Chat", newRecord.Chat).
				Float64("Weight", newRecord.Weight).
				Str("TypeName", newRecord.TypeName).
				Str("CatchType", newRecord.CatchType).
				Str("FishType", newRecord.Type).
				Str("Player", newRecord.Player).
				Str("Board", board).
				Int("ChatID", newRecord.ChatId).
				Int("FishID", newRecord.FishId).
				Msg("New Record")
		} else {
			if board != "typesmall" && board != "typesmallglobal" {
				if newRecord.Weight > oldRecord.Weight {
					logs.Logs().Info().
						Str("Date", newRecord.Date.Format(time.RFC3339)).
						Str("Chat", newRecord.Chat).
						Float64("New_Weight", newRecord.Weight).
						Float64("Old_Weight", oldRecord.Weight).
						Str("TypeName", newRecord.TypeName).
						Str("CatchType", newRecord.CatchType).
						Str("FishType", newRecord.Type).
						Str("Player", newRecord.Player).
						Str("Board", board).
						Int("ChatID", newRecord.ChatId).
						Int("FishID", newRecord.FishId).
						Msg("Updated Record")
				}
			} else {
				if newRecord.Weight < oldRecord.Weight {
					logs.Logs().Info().
						Str("Date", newRecord.Date.Format(time.RFC3339)).
						Str("Chat", newRecord.Chat).
						Float64("New_Weight", newRecord.Weight).
						Float64("Old_Weight", oldRecord.Weight).
						Str("TypeName", newRecord.TypeName).
						Str("CatchType", newRecord.CatchType).
						Str("FishType", newRecord.Type).
						Str("Player", newRecord.Player).
						Str("Board", board).
						Int("ChatID", newRecord.ChatId).
						Int("FishID", newRecord.FishId).
						Msg("Updated Record")
				}
			}
		}
	}
}

func SortMapByCountDesc(fishCaught map[string]data.FishInfo) []string {
	// Create a slice of player names
	players := make([]string, 0, len(fishCaught))
	for player := range fishCaught {
		players = append(players, player)
	}

	sort.SliceStable(players, func(i, j int) bool { return fishCaught[players[i]].Player < fishCaught[players[j]].Player })
	sort.SliceStable(players, func(i, j int) bool { return fishCaught[players[i]].Count > fishCaught[players[j]].Count })

	return players
}

func SortMapByWeightDesc(fishCaught map[string]data.FishInfo) []string {
	// Create a slice of player names
	players := make([]string, 0, len(fishCaught))
	for player := range fishCaught {
		players = append(players, player)
	}

	sort.SliceStable(players, func(i, j int) bool { return fishCaught[players[i]].Player < fishCaught[players[j]].Player })
	sort.SliceStable(players, func(i, j int) bool { return fishCaught[players[i]].Weight > fishCaught[players[j]].Weight })

	return players
}

func SortMapByValueDesc(totalPoints map[string]float64) []string {
	// Create a slice of player names
	players := make([]string, 0, len(totalPoints))
	for player := range totalPoints {
		players = append(players, player)
	}

	sort.SliceStable(players, func(i, j int) bool { return players[i] < players[j] })
	sort.SliceStable(players, func(i, j int) bool { return totalPoints[players[i]] > totalPoints[players[j]] })

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

// This isnt needed anymore ?
func ConvertToFishInfo(info LeaderboardInfo) data.FishInfo {
	return data.FishInfo{
		Weight: info.Weight,
		Type:   info.Type,
		Bot:    info.Bot,
		Player: info.Player,
	}
}
