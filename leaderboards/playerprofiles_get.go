package leaderboards

import (
	"fmt"
	"gofish/logs"
	"gofish/utils"
	"sort"
)

func GetThePlayerProfiles(params LeaderboardParams, EmojisForFish map[string]string, validPlayers []int, fishLists map[string][]string) (map[int]*PlayerProfile, error) {
	CatchtypeNames := params.Catchtypenames
	pool := params.Pool

	// the * to update the maps inside the struct directly
	Profiles := make(map[int]*PlayerProfile, len(validPlayers))

	// to see how many players have each record and their ids
	HowManyPlayersHaveRecords := make(map[string]int)
	PlayersWithRecordsPlayerIDs := make(map[string][]int)

	ignoredCatchtypes := ConstructIgnoredCatchtypeSQL()

	// The count per year per chat
	queryCountYearChat := `
		select count(*), 
		to_char(date_trunc('year', date), 'YYYY') as chatpfp,
		chat,
		playerid
		from fish 
		where playerid = any($1)
		and date < $2
	  	and date > $3
		group by chatpfp, chat, playerid`

	countyearchat, err := ReturnFishSliceQueryValidPlayers(params, queryCountYearChat, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, ble := range countyearchat {
		count := ble.Count
		year := ble.ChatPfp
		chat := ble.Chat
		playerID := ble.PlayerID

		// Add the playerid to the map first and get their name, their verified status and their twitchid
		if _, ok := Profiles[playerID]; !ok {

			Profiles[playerID] = &PlayerProfile{
				PlayerID:  playerID,
				CountYear: make(map[string]*TotalChatStruct),
			}

			Profiles[playerID].Count = &TotalChatStruct{
				Chat: make(map[string]int),
			}

			Profiles[playerID].Name, _, Profiles[playerID].Verified.Bool, Profiles[playerID].TwitchID, err = PlayerStuff(playerID, params, pool)
			if err != nil {
				return Profiles, err
			}

			if Profiles[playerID].TwitchID == 0 {
				logs.Logs().Error().
					Str("Player", Profiles[playerID].Name).
					Int("PlayerID", playerID).
					Msg("Player does not have a twitchID in the DB!")
			}

			Profiles[playerID].PlayerID = playerID
		}

		// initialize the year
		if _, ok := Profiles[playerID].CountYear[year]; !ok {

			Profiles[playerID].CountYear[year] = &TotalChatStruct{
				Chat: make(map[string]int),
			}
		}

		// Calculate the total count, the count per year and the count per chat
		Profiles[playerID].Count.Total = Profiles[playerID].Count.Total + count
		Profiles[playerID].Count.Chat[chat] = Profiles[playerID].Count.Chat[chat] + count

		Profiles[playerID].CountYear[year].Chat[chat] = count
		Profiles[playerID].CountYear[year].Total = Profiles[playerID].CountYear[year].Total + count
	}

	// sum of weight per year per chat and total
	queryWeights := `
	select round(sum(weight::numeric), 2) as weight,
	playerid,
	chat,
	to_char(date_trunc('year', date), 'YYYY') as chatpfp
	from fish
	where playerid = any($1)
	and date < $2
	and date > $3
	group by playerid, chat, chatpfp`

	weightresults, err := ReturnFishSliceQueryValidPlayers(params, queryWeights, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, result := range weightresults {
		playerID := result.PlayerID
		weight := result.Weight
		year := result.ChatPfp
		chat := result.Chat

		if Profiles[playerID].TotalWeight == nil {
			Profiles[playerID].TotalWeight = &TotalChatStructFloat{
				Chat: make(map[string]float64)}

			Profiles[playerID].TotalWeightYear = make(map[string]*TotalChatStructFloat)
		}

		Profiles[playerID].TotalWeight.Total = utils.RoundFloat(Profiles[playerID].TotalWeight.Total+weight, 2)
		Profiles[playerID].TotalWeight.Chat[chat] = utils.RoundFloat(Profiles[playerID].TotalWeight.Chat[chat]+weight, 2)

		if _, ok := Profiles[playerID].TotalWeightYear[year]; !ok {
			Profiles[playerID].TotalWeightYear[year] = &TotalChatStructFloat{
				Chat: make(map[string]float64)}
		}

		Profiles[playerID].TotalWeightYear[year].Total = utils.RoundFloat(Profiles[playerID].TotalWeightYear[year].Total+weight, 2)
		Profiles[playerID].TotalWeightYear[year].Chat[chat] = utils.RoundFloat(Profiles[playerID].TotalWeightYear[year].Chat[chat]+weight, 2)
	}

	// The last seen bag
	queryLastSeenBag := ` 
		select bag, chat, date, b.playerid
		from bag b
		join
		(
		select max(date) as max_date, playerid
		from bag
		where playerid = any($1)
		and date < $2
	  	and date > $3
		group by playerid
		) bag on b.playerid = bag.playerid and b.date = bag.max_date`

	bags, err := ReturnFishSliceQueryValidPlayers(params, queryLastSeenBag, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, lastBag := range bags {

		// If a player never did +bag this will just be nothing
		lastBag.DateString = lastBag.Date.Format("2006-01-02 15:04:05 UTC")
		Profiles[lastBag.PlayerID].Bag = ProfileBag{
			Bag:        lastBag.Bag,
			DateString: lastBag.DateString,
			Chat:       lastBag.Chat,
		}

		// Count the items in their bag
		Profiles[lastBag.PlayerID].BagCounts = make(map[string]int)
		for n, ItemInBag := range Profiles[lastBag.PlayerID].Bag.Bag {

			// change jellyfish for older fishers who never did +bag after it was changed
			// but could also just update this in the db idk
			var isjellyfish bool
			if ItemInBag == "Jellyfish" {
				isjellyfish = true

				jellyfishEmoji := EmojisForFish["jellyfish"]
				Profiles[lastBag.PlayerID].Bag.Bag[n] = jellyfishEmoji
				Profiles[lastBag.PlayerID].BagCounts[jellyfishEmoji]++
			}

			var isashiny bool
			for _, shiny := range fishLists["shiny"] {
				if ItemInBag == shiny {

					isashiny = true

					// Change the item to the shiny emoji
					shinyEmoji := fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/shiny/%s.png)", ItemInBag, ItemInBag)
					Profiles[lastBag.PlayerID].Bag.Bag[n] = shinyEmoji
					Profiles[lastBag.PlayerID].BagCounts[shinyEmoji]++
				}
			}

			if !isashiny && !isjellyfish {
				Profiles[lastBag.PlayerID].BagCounts[ItemInBag]++
			}
		}

		Profiles[lastBag.PlayerID].BagCountsSorted = sortMapStringInt(Profiles[lastBag.PlayerID].BagCounts, "countdesc")
	}

	// The fish type caught count per year per chat per catchtype
	queryFishTypesCaughtCountYearChat := `
		select count(*),
		fishname,
		chat,
		to_char(date_trunc('year', date), 'YYYY') as url,
		catchtype,
		playerid
		from fish
		where playerid = any($1)
		and date < $2
	  	and date > $3
		group by fishname, chat, date, catchtype, playerid`

	fishTypesCaughtCountYearChat, err := ReturnFishSliceQueryValidPlayers(params, queryFishTypesCaughtCountYearChat, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, ble := range fishTypesCaughtCountYearChat {
		count := ble.Count
		fishname := fmt.Sprintf("%s %s", ble.FishName, EmojisForFish[ble.FishName])
		chat := ble.Chat
		year := ble.Url
		catchtype := CatchtypeNames[ble.CatchType]
		playerID := ble.PlayerID

		// initialize the catchtype map and the fishdata map first
		if Profiles[playerID].CountCatchtype == nil {

			Profiles[playerID].CountCatchtype = make(map[string]*TotalChatStruct)

			Profiles[playerID].FishData = make(map[string]*ProfileFishData)
		}

		if _, ok := Profiles[playerID].CountCatchtype[catchtype]; !ok {

			Profiles[playerID].CountCatchtype[catchtype] = &TotalChatStruct{
				Chat: make(map[string]int),
			}
		}

		// fish caught per catchtype
		Profiles[playerID].CountCatchtype[catchtype].Total = Profiles[playerID].CountCatchtype[catchtype].Total + count

		// fish caught per catchtype per chat
		Profiles[playerID].CountCatchtype[catchtype].Chat[chat] = Profiles[playerID].CountCatchtype[catchtype].Chat[chat] + count

		// initialize the maps for the fish type
		if _, ok := Profiles[playerID].FishData[fishname]; !ok {

			Profiles[playerID].FishData[fishname] = &ProfileFishData{
				TotalCount: &TotalChatStruct{
					Chat: make(map[string]int),
				},
				CountYear:      make(map[string]*TotalChatStruct),
				CountCatchtype: make(map[string]*TotalChatStruct),
			}
		}

		// initialize the year
		if _, ok := Profiles[playerID].FishData[fishname].CountYear[year]; !ok {

			Profiles[playerID].FishData[fishname].CountYear[year] = &TotalChatStruct{
				Chat: make(map[string]int),
			}
		}

		// fish of that type caught per year per chat
		Profiles[playerID].FishData[fishname].CountYear[year].Chat[chat] = Profiles[playerID].FishData[fishname].CountYear[year].Chat[chat] + count

		// fish of that type caught overall
		Profiles[playerID].FishData[fishname].TotalCount.Total = Profiles[playerID].FishData[fishname].TotalCount.Total + count

		// increase the fish seen per chat if that fish wasnt already in this map for the chat
		if _, ok := Profiles[playerID].FishData[fishname].TotalCount.Chat[chat]; !ok {

			if Profiles[playerID].FishSeenTotal == nil {
				Profiles[playerID].FishSeenTotal = &TotalChatStruct{
					Chat: make(map[string]int),
				}
			}

			Profiles[playerID].FishSeenTotal.Chat[chat] = Profiles[playerID].FishSeenTotal.Chat[chat] + 1
		}

		// fish of that type caught per chat
		Profiles[playerID].FishData[fishname].TotalCount.Chat[chat] = Profiles[playerID].FishData[fishname].TotalCount.Chat[chat] + count

		// fish of that type caught per year
		Profiles[playerID].FishData[fishname].CountYear[year].Total = Profiles[playerID].FishData[fishname].CountYear[year].Total + count

		// initialize the catchtype count per fish type
		if _, ok := Profiles[playerID].FishData[fishname].CountCatchtype[catchtype]; !ok {

			Profiles[playerID].FishData[fishname].CountCatchtype[catchtype] = &TotalChatStruct{
				Chat: make(map[string]int),
			}
		}

		// fish of that type caught per catchtype
		Profiles[playerID].FishData[fishname].CountCatchtype[catchtype].Total = Profiles[playerID].FishData[fishname].CountCatchtype[catchtype].Total + count

		Profiles[playerID].FishData[fishname].CountCatchtype[catchtype].Chat[chat] = Profiles[playerID].FishData[fishname].CountCatchtype[catchtype].Chat[chat] + count
	}

	// all their fish seen; could get this from the fishtypescaughtcount maps
	// but this is also sorting them by name, so i dont need to sort them later
	queryFishSeen := `
		select array_agg(distinct(fishname)) as bag, playerid
		from fish 
		where playerid = any($1)
		and date < $2
	  	and date > $3
		group by playerid`

	fishseen, err := ReturnFishSliceQueryValidPlayers(params, queryFishSeen, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, fishu := range fishseen {

		Profiles[fishu.PlayerID].FishSeen = fishu.Bag
		Profiles[fishu.PlayerID].FishSeenTotal.Total = len(fishu.Bag)

		// Also get the fish that player never caught
		for _, fishy := range fishLists["all"] {
			fishneverseen := true

			for _, seenfish := range Profiles[fishu.PlayerID].FishSeen {
				if fishy == seenfish {
					fishneverseen = false
				}
			}

			if fishneverseen {
				Profiles[fishu.PlayerID].FishNotSeen = append(Profiles[fishu.PlayerID].FishNotSeen, fmt.Sprintf("%s %s", fishy, EmojisForFish[fishy]))
				Profiles[fishu.PlayerID].FishNotSeenTotal++
			}
		}

		// Sort the fish not seen by name
		sort.SliceStable(Profiles[fishu.PlayerID].FishNotSeen, func(i, j int) bool {
			return Profiles[fishu.PlayerID].FishNotSeen[i] < Profiles[fishu.PlayerID].FishNotSeen[j]
		})

	}

	// with this the order of the gifts in the present dont match the log message
	queryWinterGifts := `
		select array_agg(f.fishname) as bag, f.playerid, dates.date, f.chat
		from fish f
		join (
		select distinct(date)
		from fish
		where catchtype = 'giftwinter'
		and playerid = any($1)
		and date < $2
	  	and date > $3
		) as dates
		on f.date = dates.date
		group by playerid, dates.date, f.chat
	`

	wintergifts, err := ReturnFishSliceQueryValidPlayers(params, queryWinterGifts, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, fishu := range wintergifts {

		if Profiles[fishu.PlayerID].Other.Gifts == nil {
			Profiles[fishu.PlayerID].Other.HasOtherStuff = true
			Profiles[fishu.PlayerID].Other.HasPresents = true
			Profiles[fishu.PlayerID].Other.Gifts = make(map[string][]WinterGifts)
		}

		fishu.DateString = fishu.Date.Format("2006-01-02 15:04:05 UTC")

		var Presents WinterGifts

		for _, fish := range fishu.Bag {

			if fish == "letter" {
				Profiles[fishu.PlayerID].SonnyDay.HasLetter = true
				Profiles[fishu.PlayerID].SonnyDay.LetterInBagReceived = fishu.Date

				HowManyPlayersHaveRecords["letter"]++
				PlayersWithRecordsPlayerIDs["letter"] = append(PlayersWithRecordsPlayerIDs["letter"], fishu.PlayerID)
			}

			fish = fmt.Sprintf("%s %s", EmojisForFish[fish], fish)

			Presents.Presents = append(Presents.Presents, fish)
		}

		year := fmt.Sprintf("%d", fishu.Date.Year())

		Presents.Chat = fishu.Chat
		Presents.DateOpened = fishu.DateString

		// players can have multiple presents per year if they just dont open theirs or get it later
		Profiles[fishu.PlayerID].Other.Gifts[year] = append(Profiles[fishu.PlayerID].Other.Gifts[year], Presents)
	}

	// The 10 biggest fish per player
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
    	) bub WHERE RANK <= 10`

	fishes, err := ReturnFishSliceQueryValidPlayers(params, queryBiggestFishOverall, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, fish := range fishes {

		fish.DateString = fish.Date.Format("2006-01-02 15:04:05 UTC")

		fish.CatchType = CatchtypeNames[fish.CatchType]

		fish.Fish = fmt.Sprintf("%s %s", EmojisForFish[fish.FishName], fish.FishName)

		Profiles[fish.PlayerID].BiggestFish = append(Profiles[fish.PlayerID].BiggestFish, fish)
	}

	// The 10 last fish per player
	queryLastFishOverall := `
		SELECT bub.weight, bub.fishname, bub.chat, bub.date, bub.catchtype, bub.playerid 
		FROM (
        SELECT fish.*, 
        RANK() OVER (
            PARTITION BY playerid
            ORDER BY date DESC
        )
        FROM fish
		WHERE playerid = any($1)
		AND date < $2
	  	AND date > $3
    	) bub WHERE RANK <= 10`

	fishes, err = ReturnFishSliceQueryValidPlayers(params, queryLastFishOverall, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, fish := range fishes {

		fish.DateString = fish.Date.Format("2006-01-02 15:04:05 UTC")

		fish.CatchType = CatchtypeNames[fish.CatchType]

		fish.Fish = fmt.Sprintf("%s %s", EmojisForFish[fish.FishName], fish.FishName)

		Profiles[fish.PlayerID].LastFish = append(Profiles[fish.PlayerID].LastFish, fish)
	}

	// For biggest and smallest ignore the fish which i dont see the weight of in the catch message (squirrels and release bonus fish)
	// Also for biggest and smallest im ordering by date desc, so that as the rows are being read, if someone caught that weight multiple times
	// it should always end up printing the oldest one with that weight on the profile
	queryBiggestFishChat := fmt.Sprintf(`
		SELECT f.weight, f.fishname, f.chat, f.date, f.catchtype, f.playerid
		FROM fish f
		JOIN (
		SELECT playerid, MAX(weight) AS max_weight, chat
		FROM fish
		WHERE playerid = any($1)
		AND date < $2
		AND date > $3
		%s
		GROUP BY playerid, chat
		) AS sub
		ON f.playerid = sub.playerid AND f.weight = sub.max_weight AND f.chat = sub.chat %s
		WHERE f.playerid = any($1)
		ORDER BY date desc`, ignoredCatchtypes, ignoredCatchtypes)

	fishes, err = ReturnFishSliceQueryValidPlayers(params, queryBiggestFishChat, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, fish := range fishes {
		if Profiles[fish.PlayerID].BiggestFishChat == nil {
			Profiles[fish.PlayerID].BiggestFishChat = make(map[string]ProfileFish)
		}

		fish.DateString = fish.Date.Format("2006-01-02 15:04:05 UTC")

		fish.CatchType = CatchtypeNames[fish.CatchType]

		fish.Fish = fmt.Sprintf("%s %s", EmojisForFish[fish.FishName], fish.FishName)

		Profiles[fish.PlayerID].BiggestFishChat[fish.Chat] = fish
	}

	// If first / last catch was a mouth bonus catch dont select the mouth catch,
	// So that there arent two fish with max / min date
	queryFirstFishChat := ` 
		SELECT f.weight, f.fishname, f.chat, f.date, f.catchtype, f.playerid
		FROM fish f
		JOIN (
		SELECT playerid, MIN(date) AS min_date, chat
		FROM fish
		WHERE playerid = any($1)
		AND date < $2
		AND date > $3
		GROUP BY playerid, chat
		) AS sub
		ON f.playerid = sub.playerid AND f.date = sub.min_date AND f.chat = sub.chat AND f.catchtype != 'mouth'
		WHERE f.playerid = any($1)`

	fishes, err = ReturnFishSliceQueryValidPlayers(params, queryFirstFishChat, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, fish := range fishes {
		if Profiles[fish.PlayerID].FirstFishChat == nil {
			Profiles[fish.PlayerID].FirstFishChat = make(map[string]ProfileFish)
		}

		fish.DateString = fish.Date.Format("2006-01-02 15:04:05 UTC")

		fish.CatchType = CatchtypeNames[fish.CatchType]

		fish.Fish = fmt.Sprintf("%s %s", EmojisForFish[fish.FishName], fish.FishName)

		Profiles[fish.PlayerID].FirstFishChat[fish.Chat] = fish

		if Profiles[fish.PlayerID].FirstFish.Fish == "" {
			Profiles[fish.PlayerID].FirstFish = fish
		}

		if fish.Date.Before(Profiles[fish.PlayerID].FirstFish.Date) {
			Profiles[fish.PlayerID].FirstFish = fish
		}
	}

	queryLastFishChat := `	
		SELECT f.weight, f.fishname, f.chat, f.date, f.catchtype, f.playerid
		FROM fish f
		JOIN (
		SELECT playerid, MAX(date) AS max_date, chat
		FROM fish
		WHERE playerid = any($1)
		AND date < $2
		AND date > $3
		GROUP BY playerid, chat
		) AS sub
		ON f.playerid = sub.playerid AND f.date = sub.max_date AND f.chat = sub.chat AND f.catchtype != 'mouth'
		WHERE f.playerid = any($1)`

	fishes, err = ReturnFishSliceQueryValidPlayers(params, queryLastFishChat, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, fish := range fishes {
		if Profiles[fish.PlayerID].LastFishChat == nil {
			Profiles[fish.PlayerID].LastFishChat = make(map[string]ProfileFish)
		}

		fish.DateString = fish.Date.Format("2006-01-02 15:04:05 UTC")

		fish.CatchType = CatchtypeNames[fish.CatchType]
		fish.Fish = fmt.Sprintf("%s %s", EmojisForFish[fish.FishName], fish.FishName)

		Profiles[fish.PlayerID].LastFishChat[fish.Chat] = fish
	}

	// Get the biggest, smallest, last and first fish per fishtype
	queryBiggestFishPerType := fmt.Sprintf(` 
		SELECT f.weight, f.fishname, f.chat, f.date, f.catchtype, f.playerid
		FROM fish f
		JOIN (
		SELECT playerid, fishname, MAX(weight) AS max_weight
		FROM fish
		WHERE playerid = any($1)
		AND date < $2 
		AND date > $3
		%s
		GROUP BY playerid, fishname
		) AS sub
		ON f.playerid = sub.playerid AND f.weight = sub.max_weight AND f.fishname = sub.fishname %s
		WHERE f.playerid = any($1)
		ORDER BY date desc`, ignoredCatchtypes, ignoredCatchtypes)

	fishes, err = ReturnFishSliceQueryValidPlayers(params, queryBiggestFishPerType, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, fish := range fishes {

		fish.DateString = fish.Date.Format("2006-01-02 15:04:05 UTC")

		fish.CatchType = CatchtypeNames[fish.CatchType]

		fish.Fish = fmt.Sprintf("%s %s", EmojisForFish[fish.FishName], fish.FishName)

		Profiles[fish.PlayerID].FishData[fmt.Sprintf("%s %s", fish.FishName, EmojisForFish[fish.FishName])].Biggest = fish
	}

	querySmallestFishPerType := fmt.Sprintf(`
		SELECT f.weight, f.fishname, f.chat, f.date, f.catchtype, f.playerid
		FROM fish f
		JOIN (
		SELECT playerid, fishname, MIN(weight) AS min_weight
		FROM fish
		WHERE playerid = any($1)
		AND date < $2 
		AND date > $3
		%s
		GROUP BY playerid, fishname
		) AS sub
		ON f.playerid = sub.playerid AND f.weight = sub.min_weight AND f.fishname = sub.fishname %s
		WHERE f.playerid = any($1)
		ORDER BY date desc`, ignoredCatchtypes, ignoredCatchtypes)

	fishes, err = ReturnFishSliceQueryValidPlayers(params, querySmallestFishPerType, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, fish := range fishes {

		fish.DateString = fish.Date.Format("2006-01-02 15:04:05 UTC")

		fish.CatchType = CatchtypeNames[fish.CatchType]

		fish.Fish = fmt.Sprintf("%s %s", EmojisForFish[fish.FishName], fish.FishName)

		Profiles[fish.PlayerID].FishData[fmt.Sprintf("%s %s", fish.FishName, EmojisForFish[fish.FishName])].Smallest = fish
	}

	// If someones last catch of a type was a mouth catch and the fish in the mouth and the other catch are of the same type
	// this will select two fishes, doesnt rly happen a lot anyways so idc (?)
	queryLastFishPerType := `
		SELECT f.weight, f.fishname, f.chat, f.date, f.catchtype, f.playerid
		FROM fish f
		JOIN (
		SELECT playerid, MAX(date) AS max_date, fishname
		FROM fish
		WHERE playerid = any($1)
		AND date < $2 
		AND date > $3
		GROUP BY playerid, fishname
		) AS sub
		ON f.playerid = sub.playerid AND f.date = sub.max_date AND f.fishname = sub.fishname 
		WHERE f.playerid = any($1)`

	fishes, err = ReturnFishSliceQueryValidPlayers(params, queryLastFishPerType, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, fish := range fishes {

		fish.DateString = fish.Date.Format("2006-01-02 15:04:05 UTC")

		fish.CatchType = CatchtypeNames[fish.CatchType]

		fish.Fish = fmt.Sprintf("%s %s", EmojisForFish[fish.FishName], fish.FishName)

		Profiles[fish.PlayerID].FishData[fmt.Sprintf("%s %s", fish.FishName, EmojisForFish[fish.FishName])].Last = fish
	}

	queryFirstFishPerType := `
		SELECT f.weight, f.fishname, f.chat, f.date, f.catchtype, f.playerid
		FROM fish f
		JOIN (
		SELECT playerid, MIN(date) AS min_date, fishname
		FROM fish
		WHERE playerid = any($1)
		AND date < $2 
		AND date > $3
		GROUP BY playerid, fishname
		) AS sub
		ON f.playerid = sub.playerid AND f.date = sub.min_date AND f.fishname = sub.fishname 
		WHERE f.playerid = any($1)`

	fishes, err = ReturnFishSliceQueryValidPlayers(params, queryFirstFishPerType, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, fish := range fishes {

		fish.DateString = fish.Date.Format("2006-01-02 15:04:05 UTC")

		fish.CatchType = CatchtypeNames[fish.CatchType]

		fish.Fish = fmt.Sprintf("%s %s", EmojisForFish[fish.FishName], fish.FishName)

		Profiles[fish.PlayerID].FishData[fmt.Sprintf("%s %s", fish.FishName, EmojisForFish[fish.FishName])].First = fish

		// Update their progress for the Red Avery Treasures
		for _, redAveryTreasure := range fishLists["r.a.treasure"] {

			if fish.FishName == redAveryTreasure {

				Profiles[fish.PlayerID].Treasures.RedAveryTreasureCount++

				if Profiles[fish.PlayerID].Treasures.RedAveryTreasureCount == len(fishLists["r.a.treasure"]) {
					Profiles[fish.PlayerID].Treasures.HasAllRedAveryTreasure = true

					HowManyPlayersHaveRecords["r.a.treasure"]++
					PlayersWithRecordsPlayerIDs["r.a.treasure"] = append(PlayersWithRecordsPlayerIDs["r.a.treasure"], fish.PlayerID)
				}
			}
		}

		// Update their progress for the Mythical Fish
		for _, ogMythicalFish := range fishLists["mythic"] {

			if fish.FishName == ogMythicalFish {

				Profiles[fish.PlayerID].MythicalFish.OriginalMythicalFishCount++

				if Profiles[fish.PlayerID].MythicalFish.OriginalMythicalFishCount == len(fishLists["mythic"]) {
					Profiles[fish.PlayerID].MythicalFish.HasAllOriginalMythicalFish = true

					HowManyPlayersHaveRecords["mythic"]++
					PlayersWithRecordsPlayerIDs["mythic"] = append(PlayersWithRecordsPlayerIDs["mythic"], fish.PlayerID)
				}
			}
		}

		// update progress for birds
		for _, bird := range fishLists["bird"] {

			if fish.FishName == bird {

				Profiles[fish.PlayerID].Birds.BirdCount++

				if Profiles[fish.PlayerID].Birds.BirdCount == len(fishLists["bird"]) {
					Profiles[fish.PlayerID].Birds.HasAllBirds = true

					HowManyPlayersHaveRecords["bird"]++
					PlayersWithRecordsPlayerIDs["bird"] = append(PlayersWithRecordsPlayerIDs["bird"], fish.PlayerID)
				}
			}
		}

		// also for flowers ?
		for _, flower := range fishLists["flower"] {

			if fish.FishName == flower {

				Profiles[fish.PlayerID].Flowers.FLowerCount++

				if Profiles[fish.PlayerID].Flowers.FLowerCount == len(fishLists["flower"]) {
					Profiles[fish.PlayerID].Flowers.HasAllFlowers = true

					HowManyPlayersHaveRecords["flower"]++
					PlayersWithRecordsPlayerIDs["flower"] = append(PlayersWithRecordsPlayerIDs["flower"], fish.PlayerID)
				}
			}
		}

		// for bugs
		for _, bug := range fishLists["bug"] {
			if fish.FishName == bug {
				Profiles[fish.PlayerID].Bugs.BugCount++

				if Profiles[fish.PlayerID].Bugs.BugCount == len(fishLists["bug"]) {
					Profiles[fish.PlayerID].Bugs.HasAllBugs = true

					HowManyPlayersHaveRecords["bug"]++
					PlayersWithRecordsPlayerIDs["bug"] = append(PlayersWithRecordsPlayerIDs["bug"], fish.PlayerID)
				}
			}
		}
	}

	// debug log, need to do -mode force for this to show all of the records,
	// will only show for players which are in the validPlayers
	for record, count := range HowManyPlayersHaveRecords {
		logs.Logs().Debug().
			Str("Record", record).
			Int("Players", count).
			Interface("PlayerIDs", PlayersWithRecordsPlayerIDs[record]).
			Msg("Amount of players with record")
	}

	return Profiles, nil
}
