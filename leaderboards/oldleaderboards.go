package leaderboards

import (
	"bufio"
	"fmt"
	"gofish/logs"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
)

// Storing data from the old leaderboards
type LeaderboardInfo struct {
	Trophy int
	Silver int
	Bronze int
	Points float64
	Rank   int
	Count  int
	Weight float64
	Type   string
	Bot    string
	Player string
}

var oldweight float64
var fishType, player, chat string

func ReadOldChatStats(filePath string) (map[string]LeaderboardInfo, error) {
	oldLeaderboardStats := make(map[string]LeaderboardInfo)

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		// If the file doesn't exist, return empty rankings and counts
		return oldLeaderboardStats, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	skipHeader := 0
	for scanner.Scan() {
		line := scanner.Text()
		if skipHeader < 3 {
			skipHeader++
			continue
		}
		if strings.HasPrefix(line, "|") {
			parts := strings.Split(line, "|")
			rankStr := strings.TrimSpace(parts[1])
			rank, err := strconv.Atoi(strings.Split(rankStr, " ")[0])
			if err != nil {
				return nil, err
			}

			chatstr := strings.TrimSpace(parts[2])
			chatParts := strings.Split(chatstr, " ")
			if len(chatParts) > 0 {
				chat = chatParts[0]
			}

			countStr := strings.TrimSpace(parts[3])
			count, _ := strconv.Atoi(strings.Split(countStr, " ")[0])

			activeStr := strings.TrimSpace(parts[4])
			active, _ := strconv.Atoi(strings.Split(activeStr, " ")[0])

			uniqueStr := strings.TrimSpace(parts[5])
			unique, _ := strconv.Atoi(strings.Split(uniqueStr, " ")[0])

			uniqueFStr := strings.TrimSpace(parts[6])
			uniquef, _ := strconv.Atoi(strings.Split(uniqueFStr, " ")[0])

			oldWeightStr := strings.TrimSpace(parts[7])
			re := regexp.MustCompile(`([0-9.]+)`)
			matches := re.FindStringSubmatch(oldWeightStr)
			if len(matches) >= 2 {
				oldweight, err = strconv.ParseFloat(matches[1], 64)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Old weight string", oldWeightStr).
						Str("FishType", fishType).
						Str("Player", player).
						Str("Path", filePath).
						Str("Path", filePath).
						Msg("Could not convert old weight to float64")
					return nil, err
				}
			} else {
				err = fmt.Errorf("weight invalid")
				logs.Logs().Error().Err(err).
					Str("Old weight string", oldWeightStr).
					Str("FishType", fishType).
					Str("Player", player).
					Str("Path", filePath).
					Msg("No valid weight found") // idk
				return nil, err
			}

			oldLeaderboardStats[chat] = LeaderboardInfo{
				Rank:   rank,
				Count:  count,
				Silver: active,
				Bronze: unique,
				Trophy: uniquef,
				Weight: oldweight,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return oldLeaderboardStats, nil
}

// Not using ReadWeightRankings here because thats a map[string] and this is a map[float64]
func ReadChannelRecords(filepath string, pool *pgxpool.Pool) (map[float64]LeaderboardInfo, error) {
	oldLeaderboardRecords := make(map[float64]LeaderboardInfo)

	file, err := os.Open(filepath)
	if err != nil {
		return oldLeaderboardRecords, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	skipHeader := 0
	for scanner.Scan() {
		line := scanner.Text()
		if skipHeader < 3 {
			skipHeader++
			continue
		}
		if strings.HasPrefix(line, "|") {
			parts := strings.Split(line, "|")

			rankStr := strings.TrimSpace(parts[1])
			rank, err := strconv.Atoi(strings.Split(rankStr, " ")[0])
			if err != nil {
				return nil, err
			}

			oldPlayerStr := strings.TrimSpace(parts[2])
			oldplayer := strings.Split(oldPlayerStr, " ")[0]
			if strings.Contains(oldplayer, "*") {
				oldplayer = strings.TrimRight(oldplayer, "*")
			}

			oldWeightStr := strings.TrimSpace(parts[4])
			oldweight, err = strconv.ParseFloat(oldWeightStr, 64)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Old weight string", oldWeightStr).
					Str("Path", filepath).
					Msg("Could not convert old weight to float64")
				return nil, err
			}

			oldLeaderboardRecords[oldweight] = LeaderboardInfo{
				Weight: oldweight,
				Rank:   rank,
				Player: oldplayer,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return oldLeaderboardRecords, nil
}
