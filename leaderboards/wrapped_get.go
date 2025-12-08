package leaderboards

import (
	"fmt"
	"sort"
)

func GetTheWrappeds(params LeaderboardParams, EmojisForFish map[string]string, validPlayers []int, year string) (map[int]*Wrapped, error) {
	CatchtypeNames := params.Catchtypenames

	Wrappeds := make(map[int]*Wrapped, len(validPlayers))

	// name + twitchid here first
	for _, validPlayer := range validPlayers {

		var twitchID int
		var name string

		name, _, _, twitchID, err := PlayerStuff(validPlayer, params, params.Pool)
		if err != nil {
			return Wrappeds, err
		}

		Wrappeds[validPlayer] = &Wrapped{
			Name:     name,
			TwitchID: twitchID,
			PlayerID: validPlayer,
			Year:     year,
		}
	}

	// biggest fish and their rank overall for the year and all time
	queryBiggestFishOverall := `
		SELECT f.playerid, f.weight, f.fishname, f.chat, f.date, f.catchtype, f.fishid,
		RANK() OVER (ORDER BY f.weight DESC), bubi.rank as rankalltime
		FROM fish f
		JOIN (
			SELECT fishid, 
			RANK() OVER (ORDER BY weight DESC)
			FROM fish 
			GROUP BY fishid
		) bubi ON f.fishid = bubi.fishid
		where playerid = any($1)
		and date < $2
	  	and date > $3`

	fishes, err := ReturnFishSliceQueryValidPlayers(params, queryBiggestFishOverall, validPlayers)
	if err != nil {
		return Wrappeds, err
	}

	for _, fish := range fishes {

		if len(Wrappeds[fish.PlayerID].BiggestFish) < 5 {
			fish.DateString = fish.Date.Format("2006-01-02 15:04:05 UTC")

			fish.CatchType = CatchtypeNames[fish.CatchType]

			fish.Fish = fmt.Sprintf("%s %s", EmojisForFish[fish.FishName], fish.FishName)

			Wrappeds[fish.PlayerID].BiggestFish = append(Wrappeds[fish.PlayerID].BiggestFish, fish)
		}
	}

	// add shinies to rarest fish first
	// this could mean, that someone caught a rarer fish than a shiny, but shiny will always show as rarest ? idk
	Wrappeds, err = GetTheShiniesForWrappeds(params, Wrappeds, year)
	if err != nil {
		return Wrappeds, err
	}

	// all fish seen
	var fishSeenData []int

	queryFishSeen := `
		select array_agg(distinct(fishname)) as bag, playerid
		from fish 
		where playerid = any($1)
		and date < $2
	  	and date > $3
		group by playerid`

	fishseen, err := ReturnFishSliceQueryValidPlayers(params, queryFishSeen, validPlayers)
	if err != nil {
		return Wrappeds, err
	}

	for _, fishu := range fishseen {
		fishSeenData = append(fishSeenData, len(fishu.Bag))
		Wrappeds[fishu.PlayerID].FishSeen = fishu.Bag
		Wrappeds[fishu.PlayerID].FishSeenCount = len(fishu.Bag)
	}

	// and then calculate their percentile
	Wrappeds = CalculatePercentileWrappedFishSeen(Wrappeds, fishSeenData)

	// and then the other "normal" fish to rarest fish
	Wrappeds, err = GetRarestFishForWrappeds(params, Wrappeds, EmojisForFish)
	if err != nil {
		return Wrappeds, err
	}

	// fish caught
	fishCaughtDataChat := make(map[string][]int)

	queryFishCaught := `
	select count(*), playerid, chat
	from fish 
	where playerid = any($1)
	and date < $2
	and date > $3
	group by playerid, chat
	`

	fishCaught, err := ReturnFishSliceQueryValidPlayers(params, queryFishCaught, validPlayers)
	if err != nil {
		return Wrappeds, err
	}

	for _, caughtFish := range fishCaught {

		if Wrappeds[caughtFish.PlayerID].Count == nil {
			Wrappeds[caughtFish.PlayerID].Count = &TotalChatStructPercentile{
				ChatCounts: make(map[string]*TotalChatStructPercentile),
			}
		}

		if Wrappeds[caughtFish.PlayerID].Count.ChatCounts[caughtFish.Chat] == nil {
			Wrappeds[caughtFish.PlayerID].Count.ChatCounts[caughtFish.Chat] = &TotalChatStructPercentile{}
		}

		Wrappeds[caughtFish.PlayerID].Count.ChatCounts[caughtFish.Chat].Total = caughtFish.Count
		fishCaughtDataChat[caughtFish.Chat] = append(fishCaughtDataChat[caughtFish.Chat], caughtFish.Count)

		Wrappeds[caughtFish.PlayerID].Count.Total = Wrappeds[caughtFish.PlayerID].Count.Total + caughtFish.Count
	}

	Wrappeds = CalculatePercentileWrappedFishCaught(Wrappeds, fishCaughtDataChat)

	// most fish of a type caught
	queryMostFishOfTypeCaught := `
		SELECT bub.fishname, bub.playerid, bub.count
		FROM (
        SELECT fish.fishname, count(*), fish.playerid,
        RANK() OVER (
            PARTITION BY playerid
            ORDER BY count(*) DESC
        )
        FROM fish
		WHERE playerid = any($1)
		AND date < $2
	  	AND date > $3
		group by fishname, playerid
    	) bub WHERE RANK <= 5`

	mostFishOfTypeCaught, err := ReturnFishSliceQueryValidPlayers(params, queryMostFishOfTypeCaught, validPlayers)
	if err != nil {
		return Wrappeds, err
	}

	for _, fishOfTypeCaught := range mostFishOfTypeCaught {

		emoji := EmojisForFish[fishOfTypeCaught.FishName]

		caughtFish := CaughtFish{
			Count: fishOfTypeCaught.Count,
			Fish:  fmt.Sprintf("%s %s", emoji, fishOfTypeCaught.FishName),
		}

		Wrappeds[fishOfTypeCaught.PlayerID].MostCaughtFish = append(Wrappeds[fishOfTypeCaught.PlayerID].MostCaughtFish, caughtFish)
	}

	return Wrappeds, nil
}

func CalculatePercentileWrappedFishSeen(Wrappeds map[int]*Wrapped, fishSeenData []int) map[int]*Wrapped {

	sort.SliceStable(fishSeenData, func(i, j int) bool { return fishSeenData[i] > fishSeenData[j] })

	totalLength := len(fishSeenData)

	for _, wrapped := range Wrappeds {

		for d, fishSeenNumber := range fishSeenData {
			if fishSeenNumber == wrapped.FishSeenCount {
				wrapped.FishSeenPercentile = (float64(d+1) / float64(totalLength)) * 100.00
			}
		}
	}

	return Wrappeds
}

func CalculatePercentileWrappedFishCaught(Wrappeds map[int]*Wrapped, fishCaughtDataChat map[string][]int) map[int]*Wrapped {

	for chat, data := range fishCaughtDataChat {

		sort.SliceStable(data, func(i, j int) bool { return data[i] > data[j] })

		totalLength := len(data)

		for _, wrapped := range Wrappeds {

			if _, ok := wrapped.Count.ChatCounts[chat]; ok {
				for d, fishCaughtNumber := range data {
					if fishCaughtNumber == wrapped.Count.ChatCounts[chat].Total {
						wrapped.Count.ChatCounts[chat].Percentile = (float64(d+1) / float64(totalLength)) * 100.00
					}
				}
			}
		}
	}

	var fishCaughtDataTotal []int

	for _, wrapped := range Wrappeds {
		fishCaughtDataTotal = append(fishCaughtDataTotal, wrapped.Count.Total)
	}

	sort.SliceStable(fishCaughtDataTotal, func(i, j int) bool { return fishCaughtDataTotal[i] > fishCaughtDataTotal[j] })

	totalLength := len(fishCaughtDataTotal)

	for _, wrapped := range Wrappeds {

		for d, fishCaughtNumber := range fishCaughtDataTotal {
			if fishCaughtNumber == wrapped.Count.Total {
				wrapped.Count.Percentile = (float64(d+1) / float64(totalLength)) * 100.00
			}
		}
	}

	return Wrappeds
}
