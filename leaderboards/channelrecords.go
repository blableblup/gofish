package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
)

func processChannelRecords(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	config := params.Config
	global := params.Global
	title := params.Title
	limit := params.Limit
	mode := params.Mode
	chat := params.Chat
	pool := params.Pool
	path := params.Path

	var filePath, titlerecords string
	var weightlimit float64

	if path == "" {
		filePath = filepath.Join("leaderboards", chatName, "records.md")
	} else {
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		filePath = filepath.Join("leaderboards", chatName, path)
	}

	oldChannelRecords, err := ReadChannelRecords(filePath, pool)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error reading old leaderboard")
		return
	}

	if limit == "" {
		weightlimit = chat.Weightlimit
		if weightlimit == 0 {
			weightlimit = config.Chat["default"].Weightlimit
		}
	} else {
		weightlimit, err = strconv.ParseFloat(limit, 64)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Limit", limit).
				Str("Board", board).
				Msg("Error converting custom weight limit to float64")
			return
		}
	}

	records, err := getRecords(params, weightlimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error getting weight records")
		return
	}

	AreMapsSame := didMapsChange(records, oldChannelRecords)

	if AreMapsSame && mode != "force" {
		logs.Logs().Warn().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Not updating board because there are no changes")
		return
	}

	if title == "" {
		if global {
			titlerecords = "### History of global weight records\n"
		} else {
			if strings.HasSuffix(chatName, "s") {
				titlerecords = fmt.Sprintf("### History of channel records in %s' chat\n", chatName)
			} else {
				titlerecords = fmt.Sprintf("### History of channel records in %s's chat\n", chatName)
			}
		}
	} else {
		titlerecords = fmt.Sprintf("%s\n", title)
	}

	err = writeRecords(records, oldChannelRecords, filePath, titlerecords, global, weightlimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Error writing leaderboard")
	} else {
		logs.Logs().Info().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Leaderboard updated successfully")
	}
}

func didMapsChange(newMap map[float64]data.FishInfo, oldMap map[float64]LeaderboardInfo) bool {

	// Dont update the board if there are no changes
	// If maps are same length, check if a player renamed
	if len(oldMap) == len(newMap) {
		for weight := range newMap {
			if oldMap[weight].Player != newMap[weight].Player {
				return false
			}
		}
	} else {
		return false
	}

	return true
}

// Select the first fish above the weightlimit -> then select the first fish above that fishes weight -> ...
// Do this until there are no bigger fish anymore (if there is the error pgx.ErrNoRows)
// This will only show the oldest fish if there are multiple channel records with the same weight
func getRecords(params LeaderboardParams, weightlimit float64) (map[float64]data.FishInfo, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	global := params.Global
	date2 := params.Date2
	pool := params.Pool
	date := params.Date

	recordFish := make(map[float64]data.FishInfo)

	for {
		var fishInfo data.FishInfo

		if global {
			err := pool.QueryRow(context.Background(), `
		SELECT f.playerid, f.weight, f.fishname, f.bot, f.chat AS chatname, f.date, f.catchtype, f.fishid, f.chatid
		FROM fish f
		JOIN (
			SELECT min(date) AS min_date
			FROM fish 
			WHERE weight >= $1
			AND date < $2
	  		AND date > $3
		) min_fish ON f.date = min_fish.min_date`, weightlimit, date, date2).Scan(&fishInfo.PlayerID, &fishInfo.Weight, &fishInfo.TypeName, &fishInfo.Bot,
				&fishInfo.Chat, &fishInfo.Date, &fishInfo.CatchType, &fishInfo.FishId, &fishInfo.ChatId)
			if err != nil && err != pgx.ErrNoRows {
				logs.Logs().Error().Err(err).
					Str("Chat", chatName).
					Str("Board", board).
					Msg("Error querying fish database for channel record")
				return nil, err
			} else if err == pgx.ErrNoRows {
				break
			}
		} else {
			err := pool.QueryRow(context.Background(), `
		SELECT f.playerid, f.weight, f.fishname, f.bot, f.chat AS chatname, f.date, f.catchtype, f.fishid, f.chatid
		FROM fish f
		JOIN (
			SELECT min(date) AS min_date
			FROM fish 
			WHERE weight >= $1
			AND date < $2
	  		AND date > $3
			AND chat = $4
		) min_fish ON f.date = min_fish.min_date`, weightlimit, date, date2, chatName).Scan(&fishInfo.PlayerID, &fishInfo.Weight, &fishInfo.TypeName, &fishInfo.Bot,
				&fishInfo.Chat, &fishInfo.Date, &fishInfo.CatchType, &fishInfo.FishId, &fishInfo.ChatId)
			if err != nil && err != pgx.ErrNoRows {
				logs.Logs().Error().Err(err).
					Str("Chat", chatName).
					Str("Board", board).
					Msg("Error querying fish database for channel record")
				return nil, err
			} else if err == pgx.ErrNoRows {
				break
			}
		}

		err := pool.QueryRow(context.Background(), "SELECT name FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Player)
		if err != nil {
			logs.Logs().Error().Err(err).
				Int("PlayerID", fishInfo.PlayerID).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error retrieving player name for id")
			return nil, err
		}

		if fishInfo.Bot == "supibot" {
			err := pool.QueryRow(context.Background(), "SELECT verified FROM playerdata WHERE playerid = $1", fishInfo.PlayerID).Scan(&fishInfo.Verified)
			if err != nil {
				logs.Logs().Error().Err(err).
					Int("PlayerID", fishInfo.PlayerID).
					Str("Board", board).
					Str("Chat", chatName).
					Msg("Error retrieving verified status for playerid")
				return nil, err
			}
		}

		err = pool.QueryRow(context.Background(), "SELECT fishtype FROM fishinfo WHERE fishname = $1", fishInfo.TypeName).Scan(&fishInfo.Type)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("FishName", fishInfo.TypeName).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error retrieving fish type for fish name")
			return nil, err
		}

		if global {
			fishInfo.ChatPfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", fishInfo.Chat, fishInfo.Chat)
		}

		// A new channel record can never be 0.001 lbs bigger than the last one so this should never skip any records
		weightlimit = fishInfo.Weight + 0.001
		recordFish[fishInfo.Weight] = fishInfo
	}

	return recordFish, nil
}

func writeRecords(records map[float64]data.FishInfo, oldChannelRecords map[float64]LeaderboardInfo, filePath string, title string, global bool, weightlimit float64) error {

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "%s", title)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(file, "| # | Player | Fish | Weight in lbs ⚖️ | Date |"+func() string {
		if global {
			return " Chat |"
		}
		return ""
	}())
	_, err = fmt.Fprintln(file, "|-----|------|--------|-----------|---------|"+func() string {
		if global {
			return "-------|"
		}
		return ""
	}())
	if err != nil {
		return err
	}

	weights := sortWeights(records)

	rank := len(records) + 1

	for _, weight := range weights {
		weight := records[weight].Weight
		fishType := records[weight].Type
		fishName := records[weight].TypeName
		player := records[weight].Player
		date := records[weight].Date

		botIndicator := ""
		if records[weight].Bot == "supibot" && !records[weight].Verified {
			botIndicator = "*"
		}

		rank--

		var found bool

		oldRank := -1

		if info, ok := oldChannelRecords[weight]; ok {
			found = true
			oldRank = info.Rank
		}

		changeEmoji := ChangeEmoji(rank, oldRank, found)

		_, _ = fmt.Fprintf(file, "| %d %s | %s%s | %s %s | %v | %s |",
			rank, changeEmoji, player, botIndicator, fishType, fishName, weight, date.Format("2006-01-02 15:04:05 UTC"))
		if global {
			_, _ = fmt.Fprintf(file, " %s |", records[weight].ChatPfp)
		}
		_, err = fmt.Fprintln(file)
		if err != nil {
			return err
		}

	}

	_, _ = fmt.Fprintf(file, "\n_Only showing fish weighing >= %v lbs_\n", weightlimit)
	_, _ = fmt.Fprintf(file, "\n_Last updated at %s_", time.Now().In(time.UTC).Format("2006-01-02 15:04:05 UTC"))

	return nil
}

func sortWeights(records map[float64]data.FishInfo) []float64 {
	weights := make([]float64, 0, len(records))
	for weight := range records {
		weights = append(weights, weight)
	}

	sort.SliceStable(weights, func(i, j int) bool { return records[weights[i]].Weight > records[weights[j]].Weight })

	return weights
}
