package leaderboards

import (
	"fmt"
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

	// biggest fish
	queryBiggestFishOverall := `
		SELECT bub.weight, bub.fishname, bub.chat, bub.date, bub.catchtype, bub.playerid 
		FROM (
        SELECT fish.*, 
        RANK() OVER (
            PARTITION BY playerid
            ORDER BY weight DESC
        )
        FROM fish
		WHERE playerid = any($1)
		AND date < $2
	  	AND date > $3
    	) bub WHERE RANK <= 5`

	fishes, err := ReturnFishSliceQueryValidPlayers(params, queryBiggestFishOverall, validPlayers)
	if err != nil {
		return Wrappeds, err
	}

	for _, fish := range fishes {

		fish.DateString = fish.Date.Format("2006-01-02 15:04:05 UTC")

		fish.CatchType = CatchtypeNames[fish.CatchType]

		fish.Fish = fmt.Sprintf("%s %s", EmojisForFish[fish.FishName], fish.FishName)

		Wrappeds[fish.PlayerID].BiggestFish = append(Wrappeds[fish.PlayerID].BiggestFish, fish)
	}

	// add shinies to rarest fish first
	Wrappeds, err = GetTheShiniesForWrappeds(params, Wrappeds, year)
	if err != nil {
		return Wrappeds, err
	}

	// all fish seen
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

		Wrappeds[fishu.PlayerID].FishSeen = fishu.Bag
	}

	// and then the other "normal" fish to rarest fish
	Wrappeds, err = GetRarestFishForWrappeds(params, Wrappeds, EmojisForFish)
	if err != nil {
		return Wrappeds, err
	}

	// fish caught
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
			Wrappeds[caughtFish.PlayerID].Count = &TotalChatStruct{
				Chat: make(map[string]int),
			}
		}

		Wrappeds[caughtFish.PlayerID].Count.Chat[caughtFish.Chat] = caughtFish.Count

		Wrappeds[caughtFish.PlayerID].Count.Total = Wrappeds[caughtFish.PlayerID].Count.Total + caughtFish.Count
	}

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

		thing := fmt.Sprintf("%s %s (caught %d times)", emoji, fishOfTypeCaught.FishName, fishOfTypeCaught.Count)

		Wrappeds[fishOfTypeCaught.PlayerID].MostCaughtFish = append(Wrappeds[fishOfTypeCaught.PlayerID].MostCaughtFish, thing)
	}

	return Wrappeds, nil
}
