package data

import (
	"gofish/playerdata"
	"gofish/utils"
	"regexp"

	"github.com/jackc/pgx/v4/pgxpool"
)

func TData(chatName string, newResults []string, pool *pgxpool.Pool) ([]TrnmInfo, error) {

	cheaters := playerdata.ReadCheaters()

	patterns := []*regexp.Regexp{
		TrnmPattern,
		Trnm2Pattern,
	}

	var tdata []TrnmInfo

	for _, line := range newResults {
		Results := extractInfoFromTPatterns(line, patterns)

		for _, result := range Results {
			player := result.Player
			date := result.Date
			bot := result.Bot
			fishcaught := result.FishCaught
			fishplacement := result.FishPlacement
			totalweight := result.TotalWeight
			weightplacement := result.WeightPlacement
			biggestfish := result.BiggestFish
			biggestfishplacement := result.BiggestFishPlacement

			if utils.Contains(cheaters, player) {
				continue // Skip processing for ignored players
			}

			chat := chatName

			Tdata := TrnmInfo{
				Player:               player,
				Bot:                  bot,
				Date:                 date,
				Chat:                 chat,
				FishCaught:           fishcaught,
				FishPlacement:        fishplacement,
				TotalWeight:          totalweight,
				WeightPlacement:      weightplacement,
				BiggestFish:          biggestfish,
				BiggestFishPlacement: biggestfishplacement,
			}

			tdata = append(tdata, Tdata)
		}
	}

	return tdata, nil
}
