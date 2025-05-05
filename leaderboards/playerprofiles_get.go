package leaderboards

import (
	"context"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"sort"

	"github.com/jackc/pgx/v5"
)

func GetThePlayerProfiles(params LeaderboardParams, validPlayers []int, allFish []string, allShinies []string, redAveryTreasures []string, originalMythicalFish []string) (map[int]*PlayerProfile, error) {
	pool := params.Pool

	// the * to update the maps inside the struct directly
	Profiles := make(map[int]*PlayerProfile, len(validPlayers))

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
				PlayerID:       playerID,
				CountYear:      make(map[string]int),
				ChatCounts:     make(map[string]int),
				ChatCountsYear: make(map[string]map[string]int),
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

		if Profiles[playerID].ChatCountsYear[year] == nil {
			Profiles[playerID].ChatCountsYear[year] = make(map[string]int)
		}
		Profiles[playerID].ChatCountsYear[year][chat] = count

		// Calculate the total count, the count per year and the count per chat
		Profiles[playerID].Count = Profiles[playerID].Count + count
		Profiles[playerID].CountYear[year] = Profiles[playerID].CountYear[year] + count
		Profiles[playerID].ChatCounts[chat] = Profiles[playerID].ChatCounts[chat] + count
	}

	// The last seen bag
	queryLastSeenBag := ` 
		select bag, bot, chat, date, b.playerid
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
		Profiles[lastBag.PlayerID].Bag = lastBag

		// Count the items in their bag
		Profiles[lastBag.PlayerID].BagCounts = make(map[string]int)
		for n, ItemInBag := range Profiles[lastBag.PlayerID].Bag.Bag {

			var isashiny bool
			for _, shiny := range allShinies {
				if ItemInBag == shiny {

					isashiny = true

					// Change the item to the shiny emoji
					shinyEmoji := fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/shiny/%s.png)", ItemInBag, ItemInBag)

					Profiles[lastBag.PlayerID].Bag.Bag[n] = shinyEmoji

					Profiles[lastBag.PlayerID].BagCounts[shinyEmoji]++
				}
			}

			// Dont count the item again if it is a shiny
			if !isashiny {
				Profiles[lastBag.PlayerID].BagCounts[ItemInBag]++
			}
		}
	}

	// The first seen bag which had the ✉️ letter in it
	queryFirstSeenLetterBag := `
		select bag, bot, chat, date, b.playerid
		from bag b
		join
		(
		select min(date) as min_date, playerid
		from bag
		where playerid = any($1)
		and date < $2
	  	and date > $3
		and '✉️' = any(bag)
		group by playerid
		) bag on b.playerid = bag.playerid and b.date = bag.min_date`

	letterbags, err := ReturnFishSliceQueryValidPlayers(params, queryFirstSeenLetterBag, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, bag := range letterbags {
		Profiles[bag.PlayerID].SonnyDay.HasLetter = true
		Profiles[bag.PlayerID].SonnyDay.LetterInBagReceived = bag.Date
		Profiles[bag.PlayerID].Stars++
	}

	// The fish type caught count per year per chat per catchtype
	queryFishTypesCaughtCountYearChat := `
		select count(*),
		fishname as typename,
		chat,
		to_char(date_trunc('year', date), 'YYYY') as url,
		catchtype,
		playerid
		from fish
		where playerid = any($1)
		and date < $2
	  	and date > $3
		group by typename, chat, date, catchtype, playerid`

	fishTypesCaughtCountYearChat, err := ReturnFishSliceQueryValidPlayers(params, queryFishTypesCaughtCountYearChat, validPlayers)
	if err != nil {
		return Profiles, err
	}

	// a
	for _, ble := range fishTypesCaughtCountYearChat {
		count := ble.Count
		fishname := ble.TypeName
		chat := ble.Chat
		year := ble.Url
		catchtype := ble.CatchType
		playerID := ble.PlayerID

		if Profiles[playerID].CountCatchtype == nil {
			Profiles[playerID].CountCatchtype = make(map[string]int)
			Profiles[playerID].CountCatchtypeChat = make(map[string]map[string]int)
			Profiles[playerID].FishTypesCaughtCountCatchtype = make(map[string]map[string]int)
			Profiles[playerID].FishTypesCaughtCountCatchtypeChat = make(map[string]map[string]map[string]int)

			Profiles[playerID].FishTypesCaughtCount = make(map[string]int)
			Profiles[playerID].FishTypesCaughtCountChat = make(map[string]map[string]int)
			Profiles[playerID].FishTypesCaughtCountYear = make(map[string]map[string]int)
			Profiles[playerID].FishTypesCaughtCountYearChat = make(map[string]map[string]map[string]int)
			Profiles[playerID].FishTypesCaughtCountYearChatCatchtype = make(map[string]map[string]map[string]map[string]int)
		}

		// // fish of that type caught per year per chat per catchtype; im not using this ?
		if Profiles[playerID].FishTypesCaughtCountYearChatCatchtype[fishname] == nil {
			Profiles[playerID].FishTypesCaughtCountYearChatCatchtype[fishname] = make(map[string]map[string]map[string]int)
		}

		if Profiles[playerID].FishTypesCaughtCountYearChatCatchtype[fishname][year] == nil {
			Profiles[playerID].FishTypesCaughtCountYearChatCatchtype[fishname][year] = make(map[string]map[string]int)
		}

		if Profiles[playerID].FishTypesCaughtCountYearChatCatchtype[fishname][year][chat] == nil {
			Profiles[playerID].FishTypesCaughtCountYearChatCatchtype[fishname][year][chat] = make(map[string]int)
		}

		Profiles[playerID].FishTypesCaughtCountYearChatCatchtype[fishname][year][chat][catchtype] = count

		// fish of that type caught per year per chat
		if Profiles[playerID].FishTypesCaughtCountYearChat[fishname] == nil {
			Profiles[playerID].FishTypesCaughtCountYearChat[fishname] = make(map[string]map[string]int)
		}
		if Profiles[playerID].FishTypesCaughtCountYearChat[fishname][year] == nil {
			Profiles[playerID].FishTypesCaughtCountYearChat[fishname][year] = make(map[string]int)
		}
		Profiles[playerID].FishTypesCaughtCountYearChat[fishname][year][chat] = Profiles[playerID].FishTypesCaughtCountYearChat[fishname][year][chat] + count

		// fish of that type caught overall
		Profiles[playerID].FishTypesCaughtCount[fishname] = Profiles[playerID].FishTypesCaughtCount[fishname] + count

		// increase the fish seen per chat if that fish wasnt already in this map for the chat
		if _, ok := Profiles[playerID].FishTypesCaughtCountChat[fishname][chat]; !ok {
			if Profiles[playerID].FishSeenChat == nil {
				Profiles[playerID].FishSeenChat = make(map[string]int)
			}

			Profiles[playerID].FishSeenChat[chat] = Profiles[playerID].FishSeenChat[chat] + 1
		}

		// fish of that type caught per chat
		if Profiles[playerID].FishTypesCaughtCountChat[fishname] == nil {
			Profiles[playerID].FishTypesCaughtCountChat[fishname] = make(map[string]int)
		}
		Profiles[playerID].FishTypesCaughtCountChat[fishname][chat] = Profiles[playerID].FishTypesCaughtCountChat[fishname][chat] + count

		// fish of that type caught per year
		if Profiles[playerID].FishTypesCaughtCountYear[fishname] == nil {
			Profiles[playerID].FishTypesCaughtCountYear[fishname] = make(map[string]int)
		}
		Profiles[playerID].FishTypesCaughtCountYear[fishname][year] = Profiles[playerID].FishTypesCaughtCountYear[fishname][year] + count

		// fish caught per catchtype
		Profiles[playerID].CountCatchtype[catchtype] = Profiles[playerID].CountCatchtype[catchtype] + count

		// fish caught per catchtype per chat
		if Profiles[playerID].CountCatchtypeChat[catchtype] == nil {
			Profiles[playerID].CountCatchtypeChat[catchtype] = make(map[string]int)
		}

		Profiles[playerID].CountCatchtypeChat[catchtype][chat] = Profiles[playerID].CountCatchtypeChat[catchtype][chat] + count

		// fish of that type caught per catchtype
		if Profiles[playerID].FishTypesCaughtCountCatchtype[fishname] == nil {
			Profiles[playerID].FishTypesCaughtCountCatchtype[fishname] = make(map[string]int)
		}

		Profiles[playerID].FishTypesCaughtCountCatchtype[fishname][catchtype] = Profiles[playerID].FishTypesCaughtCountCatchtype[fishname][catchtype] + count

		// fish of that type caught per catchtype per chat
		if Profiles[playerID].FishTypesCaughtCountCatchtypeChat[fishname] == nil {
			Profiles[playerID].FishTypesCaughtCountCatchtypeChat[fishname] = make(map[string]map[string]int)
		}

		if Profiles[playerID].FishTypesCaughtCountCatchtypeChat[fishname][catchtype] == nil {
			Profiles[playerID].FishTypesCaughtCountCatchtypeChat[fishname][catchtype] = make(map[string]int)
		}

		Profiles[playerID].FishTypesCaughtCountCatchtypeChat[fishname][catchtype][chat] = Profiles[playerID].FishTypesCaughtCountCatchtypeChat[fishname][catchtype][chat] + count
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

		// Also get the fish that player never caught
		for _, fishy := range allFish {
			fishneverseen := true

			for _, seenfish := range Profiles[fishu.PlayerID].FishSeen {
				if fishy == seenfish {
					fishneverseen = false
				}
			}

			if fishneverseen {
				Profiles[fishu.PlayerID].FishNotSeen = append(Profiles[fishu.PlayerID].FishNotSeen, fishy)
			}
		}

		// Sort the fish not seen by name
		sort.SliceStable(Profiles[fishu.PlayerID].FishNotSeen, func(i, j int) bool {
			return Profiles[fishu.PlayerID].FishNotSeen[i] < Profiles[fishu.PlayerID].FishNotSeen[j]
		})

	}

	// also check if any valid player caught a shiny
	Profiles, err = GetTheShiniesForPlayerProfiles(params, Profiles)
	if err != nil {
		return Profiles, err
	}

	// The 10 biggest fish per player
	queryBiggestFishOverall := `
		SELECT bub.weight, bub.fishname as typename, bub.bot, bub.chat, bub.date, bub.catchtype, bub.fishid, bub.chatid, bub.playerid
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

		Profiles[fish.PlayerID].BiggestFish = append(Profiles[fish.PlayerID].BiggestFish, fish)
	}

	// The 10 last fish per player
	queryLastFishOverall := `
		SELECT bub.weight, bub.fishname as typename, bub.bot, bub.chat, bub.date, bub.catchtype, bub.fishid, bub.chatid, bub.playerid 
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

		Profiles[fish.PlayerID].LastFish = append(Profiles[fish.PlayerID].LastFish, fish)
	}

	// For biggest and smallest ignore the fish which i dont see the weight of in the catch message (squirrels and release bonus fish)
	// Also for biggest and smallest im ordering by date desc, so that as the rows are being read, if someone caught that weight multiple times
	// it should always end up printing the oldest one with that weight on the profile
	queryBiggestFishChat := `
		SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
		SELECT playerid, MAX(weight) AS max_weight, chat
		FROM fish
		WHERE playerid = any($1)
		AND date < $2
		AND date > $3
		AND catchtype != 'release' AND catchtype != 'squirrel'
		GROUP BY playerid, chat
		) AS sub
		ON f.playerid = sub.playerid AND f.weight = sub.max_weight AND f.chat = sub.chat AND f.catchtype != 'release' AND f.catchtype != 'squirrel'
		WHERE f.playerid = any($1)
		ORDER BY date desc`

	fishes, err = ReturnFishSliceQueryValidPlayers(params, queryBiggestFishChat, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, fish := range fishes {
		if Profiles[fish.PlayerID].BiggestFishChat == nil {
			Profiles[fish.PlayerID].BiggestFishChat = make(map[string]data.FishInfo)
		}

		Profiles[fish.PlayerID].BiggestFishChat[fish.Chat] = fish
	}

	// If first / last catch was a mouth bonus catch dont select the mouth catch,
	// So that there arent two fish with max / min date
	queryFirstFishChat := ` 
		SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
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
			Profiles[fish.PlayerID].FirstFishChat = make(map[string]data.FishInfo)
		}

		Profiles[fish.PlayerID].FirstFishChat[fish.Chat] = fish
	}

	queryLastFishChat := `	
		SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
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
			Profiles[fish.PlayerID].LastFishChat = make(map[string]data.FishInfo)
		}

		Profiles[fish.PlayerID].LastFishChat[fish.Chat] = fish
	}

	// Get the biggest, smallest, last and first fish per fishtype
	queryBiggestFishPerType := ` 
		SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
		SELECT playerid, fishname, MAX(weight) AS max_weight
		FROM fish
		WHERE playerid = any($1)
		AND date < $2 
		AND date > $3
		AND catchtype != 'release' AND catchtype != 'squirrel'
		GROUP BY playerid, fishname
		) AS sub
		ON f.playerid = sub.playerid AND f.weight = sub.max_weight AND f.fishname = sub.fishname AND f.catchtype != 'release' AND f.catchtype != 'squirrel'
		WHERE f.playerid = any($1)
		ORDER BY date desc`

	fishes, err = ReturnFishSliceQueryValidPlayers(params, queryBiggestFishPerType, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, fish := range fishes {
		if Profiles[fish.PlayerID].BiggestFishPerType == nil {
			Profiles[fish.PlayerID].BiggestFishPerType = make(map[string]data.FishInfo)
		}

		Profiles[fish.PlayerID].BiggestFishPerType[fish.TypeName] = fish
	}

	querySmallestFishPerType := `
		SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
		FROM fish f
		JOIN (
		SELECT playerid, fishname, MIN(weight) AS min_weight
		FROM fish
		WHERE playerid = any($1)
		AND date < $2 
		AND date > $3
		AND catchtype != 'release' AND catchtype != 'squirrel'
		GROUP BY playerid, fishname
		) AS sub
		ON f.playerid = sub.playerid AND f.weight = sub.min_weight AND f.fishname = sub.fishname AND f.catchtype != 'release' AND f.catchtype != 'squirrel'
		WHERE f.playerid = any($1)
		ORDER BY date desc`

	fishes, err = ReturnFishSliceQueryValidPlayers(params, querySmallestFishPerType, validPlayers)
	if err != nil {
		return Profiles, err
	}

	for _, fish := range fishes {
		if Profiles[fish.PlayerID].SmallestFishPerType == nil {
			Profiles[fish.PlayerID].SmallestFishPerType = make(map[string]data.FishInfo)
		}

		Profiles[fish.PlayerID].SmallestFishPerType[fish.TypeName] = fish
	}

	// If someones last catch of a type was a mouth catch and the fish in the mouth and the other catch are of the same type
	// this will select two fishes, doesnt rly happen a lot anyways so idc (?)
	queryLastFishPerType := `
		SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
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
		if Profiles[fish.PlayerID].LastCaughtFishPerType == nil {
			Profiles[fish.PlayerID].LastCaughtFishPerType = make(map[string]data.FishInfo)
		}

		Profiles[fish.PlayerID].LastCaughtFishPerType[fish.TypeName] = fish
	}

	queryFirstFishPerType := `
		SELECT f.weight, f.fishname as typename, f.bot, f.chat, f.date, f.catchtype, f.fishid, f.chatid, f.playerid
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
		if Profiles[fish.PlayerID].FirstCaughtFishPerType == nil {
			Profiles[fish.PlayerID].FirstCaughtFishPerType = make(map[string]data.FishInfo)
		}

		Profiles[fish.PlayerID].FirstCaughtFishPerType[fish.TypeName] = fish

		// Update their progress for the Red Avery Treasures
		for _, redAveryTreasure := range redAveryTreasures {

			if fish.TypeName == redAveryTreasure {

				Profiles[fish.PlayerID].Treasures.RedAveryTreasureCount++

				if Profiles[fish.PlayerID].Treasures.RedAveryTreasureCount == len(redAveryTreasures) {
					Profiles[fish.PlayerID].Treasures.HasAllRedAveryTreasure = true
					Profiles[fish.PlayerID].Stars++
				}
			}
		}

		// Update their progress for the Mythical Fish
		for _, ogMythicalFish := range originalMythicalFish {

			if fish.TypeName == ogMythicalFish {

				Profiles[fish.PlayerID].MythicalFish.OriginalMythicalFishCount++

				if Profiles[fish.PlayerID].MythicalFish.OriginalMythicalFishCount == len(originalMythicalFish) {
					Profiles[fish.PlayerID].MythicalFish.HasAllOriginalMythicalFish = true
					Profiles[fish.PlayerID].Stars++
				}
			}
		}
	}

	return Profiles, nil
}

func ReturnFishSliceQueryValidPlayers(params LeaderboardParams, query string, validPlayers []int) ([]data.FishInfo, error) {
	date2 := params.Date2
	date := params.Date
	pool := params.Pool

	rows, err := pool.Query(context.Background(), query, validPlayers, date, date2)
	if err != nil {
		return []data.FishInfo{}, err
	}

	fishy, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[data.FishInfo])
	if err != nil && err != pgx.ErrNoRows {
		return []data.FishInfo{}, err
	}

	return fishy, nil
}

func GetTheShiniesForPlayerProfiles(params LeaderboardParams, Profiles map[int]*PlayerProfile) (map[int]*PlayerProfile, error) {

	// use the function from the shiny board for this
	Shinies, err := getShinies(params)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", params.ChatName).
			Str("Board", params.LeaderboardType).
			Msg("Error getting shinies for player profiles")
		return Profiles, err
	}

	for _, fish := range Shinies {

		// because the leaderboard function doesnt use the validplayers
		// only store the shiny if the player is already in the map
		if _, ok := Profiles[fish.PlayerID]; ok {
			Profiles[fish.PlayerID].Shiny.ShinyCatch = append(Profiles[fish.PlayerID].Shiny.ShinyCatch, fish)

			Profiles[fish.PlayerID].Shiny.HasShiny = true
		}
	}

	return Profiles, nil
}
