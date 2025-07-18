package leaderboards

import (
	"context"
	"fmt"
	"gofish/logs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

func processAverageWeight(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	title := params.Title
	mode := params.Mode

	filePath := returnPath(params)

	oldWeights, err := getJsonBoardString(filePath)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error reading old leaderboard")
		return
	}

	Weights, err := getAverageWeights(params)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Path", filePath).
			Str("Board", board).
			Msg("Error getting leaderboard")
		return
	}

	AreMapsSame := didFishMapChange(params, oldWeights, Weights)

	if AreMapsSame && mode != "force" {
		logs.Logs().Warn().
			Str("Board", board).
			Str("Chat", chatName).
			Msg("Not updating board because there are no changes")
		return
	}

	if title == "" {
		title = "### Average weight per fish type\n"
	} else {
		title = fmt.Sprintf("%s\n", title)
	}

	err = writeAverageWeight(filePath, Weights, oldWeights, title)
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

func getAverageWeights(params LeaderboardParams) (map[string]BoardData, error) {
	board := params.LeaderboardType
	config := params.Config
	date2 := params.Date2
	date := params.Date
	pool := params.Pool

	Weights := make(map[string]BoardData)
	var rows pgx.Rows
	var err error

	for chatName, chat := range config.Chat {
		if !chat.CheckFData && chatName != "global" {
			if chatName != "default" {
				logs.Logs().Warn().
					Str("Board", board).
					Str("Chat", chatName).
					Msg("Skipping chat because checkfdata is false")
			}
			continue
		}

		// Query the database to get the average weight of each fish type per chat and globally
		if chatName != "global" {
			rows, err = pool.Query(context.Background(), `
				SELECT fishname, ROUND(AVG(weight::numeric), 2)
				FROM fish
				WHERE chat = $1
				AND date < $2
				AND date > $3
				AND catchtype != 'release'
				AND catchtype != 'squirrel'
				AND catchtype != 'sonnythrow'
				GROUP BY fishname
				`, chatName, date, date2)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Chat", chatName).
					Str("Board", board).
					Msg("Error querying database")
				return Weights, err
			}
			defer rows.Close()
		} else {
			rows, err = pool.Query(context.Background(), `
			SELECT fishname, ROUND(AVG(weight::numeric), 2)
			FROM fish
			WHERE date < $1
			AND date > $2
			AND catchtype != 'release'
			AND catchtype != 'squirrel'
			AND catchtype != 'sonnythrow'
			GROUP BY fishname
			`, date, date2)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Chat", chatName).
					Str("Board", board).
					Msg("Error querying database")
				return Weights, err
			}
			defer rows.Close()
		}

		for rows.Next() {
			var fishInfo BoardData

			if err := rows.Scan(&fishInfo.FishName, &fishInfo.Weight); err != nil {
				logs.Logs().Error().Err(err).
					Str("Chat", chatName).
					Str("Board", board).
					Msg("Error scanning row")
				return Weights, err
			}

			fishInfo.FishType, err = FishStuff(fishInfo.FishName, params)
			if err != nil {
				return Weights, err
			}

			var pfp string
			if chatName != "global" {
				pfp = fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chatName, chatName)
			}

			// If chat is global, store as weight. If not, store as chatweights
			existingFishInfo, exists := Weights[fishInfo.FishName]
			if exists {

				if chatName != "global" {
					if existingFishInfo.ChatWeights == nil {
						existingFishInfo.ChatWeights = make(map[string]float64)
					}
					existingFishInfo.ChatWeights[pfp] += fishInfo.Weight
				} else {
					existingFishInfo.Weight = fishInfo.Weight
				}

				Weights[fishInfo.FishName] = existingFishInfo
			} else {

				if chatName != "global" {
					Weights[fishInfo.FishName] = BoardData{
						FishName:    fishInfo.FishName,
						FishType:    fishInfo.FishType,
						ChatWeights: map[string]float64{pfp: fishInfo.Weight},
					}
				} else {
					Weights[fishInfo.FishName] = BoardData{
						Weight:   fishInfo.Weight,
						FishName: fishInfo.FishName,
						FishType: fishInfo.FishType,
					}
				}

			}
		}

		if err = rows.Err(); err != nil {
			logs.Logs().Error().Err(err).
				Str("Board", board).
				Str("Chat", chatName).
				Msg("Error iterating over rows")
			return Weights, err
		}
	}

	return Weights, nil
}

func writeAverageWeight(filePath string, Weights map[string]BoardData, oldWeights map[string]BoardData, title string) error {

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

	prefix := "| Rank | Fish | Weight in lbs | Chat |"

	_, _ = fmt.Fprintln(file, prefix)

	_, err = fmt.Fprintln(file, "|------|--------|-----------|-------|")
	if err != nil {
		return err
	}

	sortedFish := sortMapStringFishInfo(Weights, "weightdesc")

	rank := 1
	prevRank := 1
	prevWeight := 0.0
	occupiedRanks := make(map[int]int)

	for _, FishName := range sortedFish {
		Weight := Weights[FishName].Weight
		ChatWeights := Weights[FishName].ChatWeights
		Emoji := Weights[FishName].FishType

		if Weight != prevWeight {
			rank += occupiedRanks[rank]
			occupiedRanks[rank] = 1
		} else {
			rank = prevRank
			occupiedRanks[rank]++
		}

		// Store the rank
		if ranksksk, ok := Weights[FishName]; ok {

			ranksksk.Rank = rank

			Weights[FishName] = ranksksk
		}

		var found bool
		oldRank := -1
		oldWeight := Weight
		oldFishInfo, ok := oldWeights[FishName]
		if ok {
			found = true
			oldRank = oldFishInfo.Rank
			oldWeight = oldFishInfo.Weight
		}

		changeEmoji := ChangeEmoji(rank, oldRank, found)

		ranks := Ranks(rank)

		var weights string

		weightDifference := Weight - oldWeight
		if weightDifference != 0 {
			if weightDifference > 0 {
				weights = fmt.Sprintf("%.2f (+%.2f)", Weight, weightDifference)
			} else {
				weights = fmt.Sprintf("%.2f (%.2f)", Weight, weightDifference)
			}
		} else {
			weights = fmt.Sprintf("%.2f", Weight)
		}

		_, _ = fmt.Fprintf(file, "| %s %s | %s %s | %s |", ranks, changeEmoji, Emoji, FishName, weights)

		_, _ = fmt.Fprint(file, " <details>")

		_, _ = fmt.Fprint(file, " <summary>Chat data</summary>")

		ChatWeightsSlice := make([]struct {
			chat   string
			weight float64
		}, 0, 2)

		for k, v := range ChatWeights {
			ChatWeightsSlice = append(ChatWeightsSlice, struct {
				chat   string
				weight float64
			}{k, v})
		}

		sort.Slice(ChatWeightsSlice, func(i, j int) bool {
			return ChatWeightsSlice[i].chat < ChatWeightsSlice[j].chat
		})

		for _, weight := range ChatWeightsSlice {
			_, _ = fmt.Fprintf(file, " %s %v ", weight.chat, weight.weight)
		}

		_, _ = fmt.Fprint(file, " </details>")

		_, _ = fmt.Fprint(file, "|")

		_, err = fmt.Fprintln(file)
		if err != nil {
			return err
		}

		prevWeight = Weight
		prevRank = rank
	}

	_, _ = fmt.Fprintf(file, "\n_Last updated at %s_", time.Now().In(time.UTC).Format("2006-01-02 15:04:05 UTC"))

	// This has to be here, because im not getting the rank directly from the query
	err = writeRaw(filePath, Weights)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Path", filePath).
			Msg("Error writing raw leaderboard")
		return err
	} else {
		logs.Logs().Info().
			Str("Path", filePath).
			Msg("Raw leaderboard updated successfully")
	}

	return nil
}
