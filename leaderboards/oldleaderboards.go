package leaderboards

import (
	"bufio"
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"gofish/playerdata"
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
var fishinfotable = "fishinfo"
var fishType, bot, player, chat string

func ReadOldTrophyRankings(filePath string, pool *pgxpool.Pool) (map[string]LeaderboardInfo, error) {
	oldLeaderboardTrophy := make(map[string]LeaderboardInfo)

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		// If the file doesn't exist, return empty rankings and counts
		return oldLeaderboardTrophy, nil
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
			oldplayer := strings.TrimSpace(parts[2])
			if strings.Contains(oldplayer, "*") {
				oldplayer = strings.TrimRight(oldplayer, "*")
			}

			// Check if the player renamed
			player, err := playerdata.PlayerRenamed(oldplayer, pool)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Path", filePath).
					Str("OldPlayer", oldplayer).
					Msg("Error checking if player renamed")
				return nil, err
			}

			trohpyStr := strings.TrimSpace(parts[3])
			trophies, _ := strconv.Atoi(strings.Split(trohpyStr, " ")[0])
			silverMedalsStr := strings.TrimSpace(parts[4])
			silverMedals, _ := strconv.Atoi(strings.Split(silverMedalsStr, " ")[0])
			bronzeMedalsStr := strings.TrimSpace(parts[5])
			bronzeMedals, _ := strconv.Atoi(strings.Split(bronzeMedalsStr, " ")[0])
			pointsStr := strings.TrimSpace(parts[6])
			points, _ := strconv.ParseFloat(strings.Split(pointsStr, " ")[0], 64)

			oldLeaderboardTrophy[player] = LeaderboardInfo{
				Rank:   rank,
				Trophy: trophies,
				Silver: silverMedals,
				Bronze: bronzeMedals,
				Points: points,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return oldLeaderboardTrophy, nil
}

func ReadWeightRankings(filePath string, pool *pgxpool.Pool) (map[string]LeaderboardInfo, error) {
	oldLeaderboardWeight := make(map[string]LeaderboardInfo)

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		// If the file doesn't exist, return empty rankings and counts
		return oldLeaderboardWeight, nil
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
			oldplayer := strings.TrimSpace(parts[2])
			if strings.Contains(oldplayer, "*") {
				oldplayer = strings.TrimRight(oldplayer, "*")
				bot = "supibot"
			}

			// Check if the player renamed
			player, err := playerdata.PlayerRenamed(oldplayer, pool)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Path", filePath).
					Str("OldPlayer", oldplayer).
					Msg("Error checking if player renamed")
				return nil, err
			}

			// Get the fish name through get fish name for the fish on the leaderboard
			// And then get the fish type again
			// This way, the fish type on the leaderboard will always be the newest emote for it
			// And also, any fish types which shouldnt be on the leaderboard will get caught
			board := true
			oldfishTypeStr := strings.TrimSpace(parts[3])
			oldfishType := strings.Split(oldfishTypeStr, " ")[0]
			fishName, err := data.GetFishName(pool, fishinfotable, oldfishType, board)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("FishName", fishName).
					Str("Player", player).
					Str("Path", filePath).
					Msg("Error retrieving fish name for old fish type")
				return nil, err
			}
			err = pool.QueryRow(context.Background(), "SELECT fishtype FROM fishinfo WHERE fishname = $1", fishName).Scan(&fishType)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("FishName", fishName).
					Str("Player", player).
					Str("Path", filePath).
					Msg("Error retrieving fish type for fish name")
				return nil, err
			}

			oldWeightStr := strings.TrimSpace(parts[4])
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

			oldLeaderboardWeight[player] = LeaderboardInfo{
				Rank:   rank,
				Weight: oldweight,
				Type:   fishType,
				Bot:    bot,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return oldLeaderboardWeight, nil
}

func ReadTypeRankings(filePath string, pool *pgxpool.Pool) (map[string]LeaderboardInfo, error) {
	oldLeaderboardType := make(map[string]LeaderboardInfo)

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		// If the file doesn't exist, return empty rankings and counts
		return oldLeaderboardType, nil
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

			board := true
			oldfishTypeStr := strings.TrimSpace(parts[2])
			oldfishType := strings.Split(oldfishTypeStr, " ")[0]
			fishName, err := data.GetFishName(pool, fishinfotable, oldfishType, board)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("FishName", fishName).
					Str("Player", player).
					Str("Path", filePath).
					Msg("Error retrieving fish name for old fish type")
				return nil, err
			}
			err = pool.QueryRow(context.Background(), "SELECT fishtype FROM fishinfo WHERE fishname = $1", fishName).Scan(&fishType)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("FishName", fishName).
					Str("Player", player).
					Str("Path", filePath).
					Msg("Error retrieving fish type for fish name")
				return nil, err
			}

			oldplayer := strings.TrimSpace(parts[4])
			if strings.Contains(oldplayer, "*") {
				oldplayer = strings.TrimRight(oldplayer, "*")
				bot = "supibot"
			}

			// Check if the player renamed
			player, err := playerdata.PlayerRenamed(oldplayer, pool)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Path", filePath).
					Str("OldPlayer", oldplayer).
					Msg("Error checking if player renamed")
				return nil, err
			}

			oldWeightStr := strings.TrimSpace(parts[3])
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

			oldLeaderboardType[fishType] = LeaderboardInfo{
				Rank:   rank,
				Weight: oldweight,
				Player: player,
				Bot:    bot,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return oldLeaderboardType, nil
}

func ReadTotalcountRankings(filePath string, pool *pgxpool.Pool, isFish bool) (map[string]LeaderboardInfo, error) {
	oldLeaderboardCount := make(map[string]LeaderboardInfo)

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		// If the file doesn't exist, return empty rankings and counts
		return oldLeaderboardCount, nil
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
				bot = "supibot"
			}

			// Check if the player renamed or is a fish (for global rarest fish leaderboard)
			if isFish {
				board := true
				oldfishType := oldplayer
				fishName, err := data.GetFishName(pool, fishinfotable, oldfishType, board)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("FishName", fishName).
						Str("Player", player).
						Str("Path", filePath).
						Msg("Error retrieving fish name for old fish type")
					return nil, err
				}
				err = pool.QueryRow(context.Background(), "SELECT fishtype FROM fishinfo WHERE fishname = $1", fishName).Scan(&fishType)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("FishName", fishName).
						Str("Player", player).
						Str("Path", filePath).
						Msg("Error retrieving fish type for fish name")
					return nil, err
				}
				player = fishType
			} else {
				player, err = playerdata.PlayerRenamed(oldplayer, pool)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Path", filePath).
						Str("OldPlayer", oldplayer).
						Msg("Error checking if player renamed")
					return nil, err
				}
			}

			countStr := strings.TrimSpace(parts[3])
			count, _ := strconv.Atoi(strings.Split(countStr, " ")[0])

			oldLeaderboardCount[player] = LeaderboardInfo{
				Rank:  rank,
				Count: count,
				Bot:   bot,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return oldLeaderboardCount, nil
}

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
