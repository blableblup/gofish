package data

import (
	"github.com/jackc/pgx/v4/pgxpool"
)

func TData(chatName string, newResults []string, pool *pgxpool.Pool) ([]TrnmInfo, error) {

	var tdata []TrnmInfo

	for _, line := range newResults {
		Results := extractInfoFromTData(line)

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
			line := result.Line

			// Skip the two players who cheated here
			if player == "cyancaesar" || player == "hansworthelias" {
				if bot == "supibot" {
					continue
				}
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
				Line:                 line,
			}

			tdata = append(tdata, Tdata)
		}
	}

	return tdata, nil
}
