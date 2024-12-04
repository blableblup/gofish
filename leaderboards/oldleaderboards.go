package leaderboards

import (
	"bufio"
	"gofish/logs"
	"os"
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
