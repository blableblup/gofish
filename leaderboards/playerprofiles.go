package leaderboards

import (
	"context"
	"database/sql"
	"fmt"
	"gofish/data"
	"gofish/logs"
	"gofish/utils"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/jackc/pgx/v5"
)

type PlayerProfile struct {
	Name     string
	PlayerID int
	TwitchID int
	Verified sql.NullBool

	// To show the "progress" of a player in their fishing career
	Treasures TreasureProgress
	SonnyDay  SonnyDayProgress
	// and shinies
	Shiny Shinies

	Count              int
	CountYear          map[string]int
	ChatCounts         map[string]int
	ChatCountsYear     map[string]map[string]int
	CountCatchtype     map[string]int
	CountCatchtypeChat map[string]map[string]int

	BiggestFish     []data.FishInfo
	LastFish        []data.FishInfo
	FirstFish       data.FishInfo
	BiggestFishChat map[string]data.FishInfo
	LastFishChat    map[string]data.FishInfo
	FirstFishChat   map[string]data.FishInfo

	FishSeen     []string
	FishNotSeen  []string
	FishSeenChat map[string]int

	FishTypesCaughtCount                  map[string]int
	FishTypesCaughtCountChat              map[string]map[string]int
	FishTypesCaughtCountYear              map[string]map[string]int
	FishTypesCaughtCountYearChat          map[string]map[string]map[string]int
	FishTypesCaughtCountCatchtype         map[string]map[string]int
	FishTypesCaughtCountCatchtypeChat     map[string]map[string]map[string]int
	FishTypesCaughtCountYearChatCatchtype map[string]map[string]map[string]map[string]int

	BiggestFishPerType     map[string]data.FishInfo
	SmallestFishPerType    map[string]data.FishInfo
	FirstCaughtFishPerType map[string]data.FishInfo
	LastCaughtFishPerType  map[string]data.FishInfo

	Bag       data.FishInfo
	BagCounts map[string]int
}

type TreasureProgress struct {
	FirstTimeCaughtRedAveryTreasure map[string]data.FishInfo
	HasAllRedAveryTreasure          bool
	RedAveryTreasureCount           int
}

type SonnyDayProgress struct {
	LetterInBag data.FishInfo
	HasLetter   bool
}

type Shinies struct {
	ShinyCatch []data.FishInfo
	HasShiny   bool
}

func GetPlayerProfiles(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	config := params.Config
	limit := params.Limit
	pool := params.Pool

	var countlimit int
	var err error
	if limit == "" {
		// This can only be run as "global" so always go with default
		countlimit = config.Chat["default"].PlayerCountLimit
	} else {
		countlimit, err = strconv.Atoi(limit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Limit", limit).
				Str("Board", board).
				Msg("Error converting custom limit to int")
			return
		}
	}

	validPlayers, err := GetValidPlayers(params, countlimit)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error getting valid players")
		return
	}

	// I need to get the newest emoji per fishname
	// because there were fish which had different emojis in older logs (like ⛸ / ⛸️ for ice skate)
	// and i didnt change that
	// so get all the fish first and then use the utils function
	allFishes, err := GetAllFishNames(params)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error getting all fish names")
		return
	}

	FishWithEmoji := make(map[string]string)

	for _, fish := range allFishes {
		FishWithEmoji[fish], err = FishStuff(fish, params, pool)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Str("Fish", fish).
				Msg("Error getting fish emoji")
			return
		}
	}

	// Get the names of all the shinies in the db
	// this is only for if a player has a shiny in their bag so that it shows the emote
	// if a shiny is in any other table on the profile, it will not show the shiny but the emote of the fishname instead
	allShinies, err := GetAllShinies(params)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error getting all shinies")
		return
	}

	// Get the names for the different type of ways you can catch fish
	Catchtypenames := CatchtypeNames()

	// Get the player profiles and print them for each player

	logs.Logs().Info().
		Int("Amount of players", len(validPlayers)).
		Msg("Updating player profiles")

	playerProfiles, err := GetThePlayerProfiles(params, validPlayers, allFishes, allShinies)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error getting player profiles")
	}

	wg := new(sync.WaitGroup)

	for _, validPlayer := range validPlayers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			err = PrintPlayerProfile(playerProfiles[validPlayer], FishWithEmoji, Catchtypenames)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Chat", chatName).
					Str("Board", board).
					Int("PlayerID", validPlayer).
					Msg("Error printing player profile")
			}
		}()
	}

	wg.Wait()

	logs.Logs().Info().
		Str("Chat", chatName).
		Str("Board", board).
		Msg("Done updating player profiles")

}

func GetValidPlayers(params LeaderboardParams, limit int) ([]int, error) {
	board := params.LeaderboardType
	chatName := params.ChatName
	date2 := params.Date2
	date := params.Date
	mode := params.Mode
	pool := params.Pool

	var validPlayers []int
	var rows pgx.Rows
	var err error

	// If nothing else was specified for date 2 and its the default date
	// have date 2 be 7 days before date1, so the players above the limit who fished in the last 7 days are selected
	// because ill update this with the other leaderboards on sunday
	if date2 == "2022-12-03" {
		datetime, err := utils.ParseDate(date)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Date", date).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error parsing date into time.Time for active fishers")
			return validPlayers, err
		}
		date2 = datetime.AddDate(0, 0, -7).Format("2006-01-2")
	}

	// select all the players
	queryall := `
		select playerid from fish
		group by playerid
		having count(*) >= $1`

	// only select the players who fished in that time period and have their total count above the limit
	queryrecent := `
		select f.playerid from fish f
		join 
		(
		select playerid from fish
		where date < $2
		and date >= $3
		group by playerid
		) bla on bla.playerid = f.playerid
		group by f.playerid
		having count(*) >= $1`

	if mode == "force" {

		rows, err = pool.Query(context.Background(), queryall, limit)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error querying database")
			return validPlayers, err
		}
		defer rows.Close()

	} else {

		rows, err = pool.Query(context.Background(), queryrecent, limit, date, date2)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error querying database")
			return validPlayers, err
		}
		defer rows.Close()

	}

	for rows.Next() {

		var playerID int

		if err := rows.Scan(&playerID); err != nil {
			logs.Logs().Error().Err(err).
				Str("Chat", chatName).
				Str("Board", board).
				Msg("Error scanning row")
			return validPlayers, err
		}

		validPlayers = append(validPlayers, playerID)
	}

	if err = rows.Err(); err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error iterating over rows")
		return validPlayers, err
	}

	return validPlayers, nil
}

// can put code which is printing the same type of maps into their own function ? or use the already existing leaderboard functions ? ?
// https://github.com/nao1215/markdown ? ?
func PrintPlayerProfile(Profile *PlayerProfile, EmojisForFish map[string]string, CatchtypeNames map[string]string) error {

	filePath := filepath.Join("leaderboards", "global", "players", fmt.Sprintf("%d", Profile.TwitchID)+".md")

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, _ = fmt.Fprintf(file, "# %s", Profile.Name)

	// show off some stuff

	// this means that they caught them atleast once
	// doesnt mean that they still have them in their bag
	if Profile.Treasures.HasAllRedAveryTreasure {
		_, _ = fmt.Fprintln(file, "\n* 🏅 Has found all the treasures from legendary pirate Red Avery 🗡️ 👑 🧭 !")
	}

	// received means when it first appeared in their bag
	if Profile.SonnyDay.HasLetter {
		_, _ = fmt.Fprintf(file, "\n* 🏅 Has gotten a letter ✉️ ! (Received: %s)\n", Profile.SonnyDay.LetterInBag.Date.Format("2006-01-02 15:04:05"))
	}

	if Profile.Shiny.HasShiny {
		// no medal because its just extremely rare and doesnt count as progress in anything
		_, _ = fmt.Fprintln(file, "\n* Has caught a shiny !")

		_, _ = fmt.Fprintln(file, "\n| Fish | Weight in lbs | Catchtype | Date in UTC | Chat |")

		_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|-------|")
		// sort them ? extremely unlikely to catch multiple shinies
		for _, shiny := range Profile.Shiny.ShinyCatch {
			_, _ = fmt.Fprintf(file, "\n| %s %s | %.2f | %s | %s | %s |",
				shiny.Type,
				shiny.TypeName,
				shiny.Weight,
				CatchtypeNames[shiny.CatchType],
				shiny.Date.Format("2006-01-02 15:04:05"),
				fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", shiny.Chat, shiny.Chat))
		}
	}

	// show their fish count per chat, catchtype, year
	_, _ = fmt.Fprintln(file, "\n## Data for their fish caught")

	_, _ = fmt.Fprintf(file, "\n| Total fish caught | %d |", Profile.Count)

	_, _ = fmt.Fprintln(file, "\n|-------|-------|")

	_, _ = fmt.Fprintln(file, "\n\nFish caught per chat")

	_, _ = fmt.Fprintln(file, "\n| Rank | Chat | Count |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|")

	rank := 1 // make chats with same count have same rank

	sortedChatCounts := sortMapString(Profile.ChatCounts, "countdesc")

	for _, chat := range sortedChatCounts {
		_, _ = fmt.Fprintf(file, "\n| %s | %s %s | %d |",
			Ranks(rank),
			chat,
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat),
			Profile.ChatCounts[chat])
		rank++
	}

	_, _ = fmt.Fprintln(file, "\n\nFish caught per year")

	_, _ = fmt.Fprintln(file, "\n| --- | Year | Count | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|")

	rank = 1

	sortedYearCounts := sortMapString(Profile.CountYear, "nameasc")

	for _, year := range sortedYearCounts {
		_, _ = fmt.Fprintf(file, "\n| %d | %s | %d |",
			rank,
			year,
			Profile.CountYear[year])

		sortedChatCountsYear := sortMapString(Profile.ChatCountsYear[year], "countdesc")

		for _, chat := range sortedChatCountsYear {
			_, _ = fmt.Fprintf(file, " %s %d ",
				fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat),
				Profile.ChatCountsYear[year][chat])
		}
		_, _ = fmt.Fprint(file, "|")
		rank++
	}

	_, _ = fmt.Fprintln(file, "\n\nFish caught per catchtype")

	_, _ = fmt.Fprintln(file, "\n| --- | Catchtype | Count | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|")

	rank = 1

	sortedCatchtypes := sortMapString(Profile.CountCatchtype, "countdesc")

	for _, catch := range sortedCatchtypes {

		catchtype := CatchtypeNames[catch]

		_, _ = fmt.Fprintf(file, "\n| %d | %s | %d |",
			rank,
			catchtype,
			Profile.CountCatchtype[catch])

		sortedChatCounts := sortMapString(Profile.CountCatchtypeChat[catch], "countdesc")

		for _, chat := range sortedChatCounts {
			_, _ = fmt.Fprintf(file, " %s %d",
				fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat),
				Profile.CountCatchtypeChat[catch][chat])
		}

		_, _ = fmt.Fprint(file, "|")
		rank++
	}

	// first, biggest, last fish
	_, _ = fmt.Fprintln(file, "\n## First, biggest and last fish")

	_, _ = fmt.Fprintln(file, "\n\nFirst ever fish caught per chat")

	_, _ = fmt.Fprintln(file, "\n| --- | Fish | Weight in lbs | Catchtype | Date in UTC | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|-------|-------|")

	rank = 1

	sortedChatDates := sortMapStringFishInfo(Profile.FirstFishChat, "dateasc")

	for _, chat := range sortedChatDates {
		_, _ = fmt.Fprintf(file, "\n| %d | %s %s | %.2f | %s | %s | %s |",
			rank,
			EmojisForFish[Profile.FirstFishChat[chat].TypeName],
			Profile.FirstFishChat[chat].TypeName,
			Profile.FirstFishChat[chat].Weight,
			CatchtypeNames[Profile.FirstFishChat[chat].CatchType],
			Profile.FirstFishChat[chat].Date.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat))
		rank++
	}

	_, _ = fmt.Fprintln(file, "\n\nLast fish caught per chat")

	_, _ = fmt.Fprintln(file, "\n| --- | Fish | Weight in lbs | Catchtype | Date in UTC | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|-------|-------|")

	rank = 1

	sortedChatDates2 := sortMapStringFishInfo(Profile.LastFishChat, "dateasc")

	for _, chat := range sortedChatDates2 {
		_, _ = fmt.Fprintf(file, "\n| %d | %s %s | %.2f | %s | %s | %s |",
			rank,
			EmojisForFish[Profile.LastFishChat[chat].TypeName],
			Profile.LastFishChat[chat].TypeName,
			Profile.LastFishChat[chat].Weight,
			CatchtypeNames[Profile.LastFishChat[chat].CatchType],
			Profile.LastFishChat[chat].Date.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat))
		rank++
	}

	_, _ = fmt.Fprintln(file, "\n\nBiggest fish caught per chat")

	_, _ = fmt.Fprintln(file, "\n| --- | Fish | Weight in lbs | Catchtype | Date in UTC | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|-------|-------|")

	rank = 1

	sortedChatWeights := sortMapStringFishInfo(Profile.BiggestFishChat, "weightdesc")

	for _, chat := range sortedChatWeights {
		_, _ = fmt.Fprintf(file, "\n| %d | %s %s | %.2f | %s | %s | %s |",
			rank,
			EmojisForFish[Profile.BiggestFishChat[chat].TypeName],
			Profile.BiggestFishChat[chat].TypeName,
			Profile.BiggestFishChat[chat].Weight,
			CatchtypeNames[Profile.BiggestFishChat[chat].CatchType],
			Profile.BiggestFishChat[chat].Date.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat))
		rank++
	}

	_, _ = fmt.Fprintln(file, "\n\nTheir overall biggest fish caught")

	_, _ = fmt.Fprintln(file, "\n| --- | Fish | Weight in lbs | Catchtype | Date in UTC | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|-------|-------|")

	rank = 1

	for _, Fish := range Profile.BiggestFish {
		_, _ = fmt.Fprintf(file, "\n| %s | %s %s | %.2f | %s | %s | %s |",
			Ranks(rank),
			EmojisForFish[Fish.TypeName],
			Fish.TypeName,
			Fish.Weight,
			CatchtypeNames[Fish.CatchType],
			Fish.Date.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", Fish.Chat, Fish.Chat))
		rank++
	}

	_, _ = fmt.Fprintln(file, "\n\nTheir overall last fish caught")

	_, _ = fmt.Fprintln(file, "\n| --- | Fish | Weight in lbs | Catchtype | Date in UTC | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|-------|-------|")

	rank = 1

	for _, Fish := range Profile.LastFish {
		_, _ = fmt.Fprintf(file, "\n| %d | %s %s | %.2f | %s | %s | %s |",
			rank,
			EmojisForFish[Fish.TypeName],
			Fish.TypeName,
			Fish.Weight,
			CatchtypeNames[Fish.CatchType],
			Fish.Date.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", Fish.Chat, Fish.Chat))
		rank++
	}

	// how many different fish they have seen in total and per chat
	_, _ = fmt.Fprintln(file, "\n## Their fish seen")

	_, _ = fmt.Fprintf(file, "\n| Total fish seen | %d |", len(Profile.FishSeen))

	_, _ = fmt.Fprintln(file, "\n|-------|-------|")

	_, _ = fmt.Fprintln(file, "\nFish seen per chat")

	_, _ = fmt.Fprintln(file, "\n| Rank | Chat | Count |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|")

	rank = 1

	sortedChatCounts = sortMapString(Profile.FishSeenChat, "countdesc")

	for _, chat := range sortedChatCounts {
		_, _ = fmt.Fprintf(file, "\n| %s | %s %s | %d |",
			Ranks(rank),
			chat,
			fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat),
			Profile.FishSeenChat[chat])
		rank++
	}

	_, _ = fmt.Fprintln(file)

	_, _ = fmt.Fprintln(file, "\n## Data about each of their seen fish")

	// print one block for each seen fish type
	// show their total count caught, count per year per chat
	// and first, last, biggest and smallest per type
	for _, fish := range Profile.FishSeen {

		_, _ = fmt.Fprintf(file, "\n| %s %s | Total caught | %d |",
			EmojisForFish[fish],
			fish,
			Profile.FishTypesCaughtCount[fish])

		_, _ = fmt.Fprintln(file, "\n|-------|-------|-------|")

		_, _ = fmt.Fprintf(file, "\n| %s | Year | Count | Chat |\n", EmojisForFish[fish])

		_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|")

		rank = 1

		for _, year := range sortedYearCounts {

			// Skip the fish not caught in that year
			if Profile.FishTypesCaughtCountYear[fish][year] == 0 {
				continue
			}

			_, _ = fmt.Fprintf(file, "\n| %d | %s | %d |",
				rank,
				year,
				Profile.FishTypesCaughtCountYear[fish][year])

			sortedChatCountsType := sortMapString(Profile.FishTypesCaughtCountYearChat[fish][year], "countdesc")

			for _, chat := range sortedChatCountsType {

				_, _ = fmt.Fprintf(file, " %s %d",
					fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat),
					Profile.FishTypesCaughtCountYearChat[fish][year][chat])
			}

			_, _ = fmt.Fprint(file, " |")

			rank++

		}
		_, _ = fmt.Fprintln(file)

		_, _ = fmt.Fprintf(file, "\n| %s | Catchtype | Count | Chat |\n", EmojisForFish[fish])

		_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|")

		rank = 1

		sortedCatchtypes := sortMapString(Profile.FishTypesCaughtCountCatchtype[fish], "countdesc")

		for _, catch := range sortedCatchtypes {

			catchtype := CatchtypeNames[catch]

			_, _ = fmt.Fprintf(file, "\n| %d | %s | %d |",
				rank,
				catchtype,
				Profile.FishTypesCaughtCountCatchtype[fish][catch])

			sortedChatCountsType := sortMapString(Profile.FishTypesCaughtCountCatchtypeChat[fish][catch], "countdesc")

			for _, chat := range sortedChatCountsType {

				_, _ = fmt.Fprintf(file, " %s %d",
					fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", chat, chat),
					Profile.FishTypesCaughtCountCatchtypeChat[fish][catch][chat])
			}

			_, _ = fmt.Fprint(file, " |")

			rank++

		}
		_, _ = fmt.Fprintln(file)

		_, _ = fmt.Fprintf(file, "\n| %s | Weight in lbs | Catchtype | Date in UTC | Chat |\n", EmojisForFish[fish])

		_, _ = fmt.Fprint(file, "|-------|-------|-------|-------|-------|")

		MapsToUse := []map[string]data.FishInfo{Profile.FirstCaughtFishPerType, Profile.LastCaughtFishPerType, Profile.BiggestFishPerType, Profile.SmallestFishPerType}
		Stringy := []string{"First Caught", "Last caught", "Biggest", "Smallest"}

		for Inty, Mup := range MapsToUse {
			_, _ = fmt.Fprintf(file, "\n| %s | %.2f | %s | %s | %s |",
				Stringy[Inty],
				Mup[fish].Weight,
				CatchtypeNames[Mup[fish].CatchType],
				Mup[fish].Date.Format("2006-01-02 15:04:05"),
				fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)",
					Mup[fish].Chat,
					Mup[fish].Chat))
		}

		_, _ = fmt.Fprintln(file)
	}

	// show what fish they never caught
	_, _ = fmt.Fprintln(file, "\n## Fish they have never seen")

	for _, fish := range Profile.FishNotSeen {

		_, _ = fmt.Fprintf(file, "\n* %s %s", EmojisForFish[fish], fish)

	}

	_, _ = fmt.Fprintf(file, "\n\nIn total %d fish never seen", len(Profile.FishNotSeen))

	// show their last seen bag, and how many of each fish they had in their bag
	_, _ = fmt.Fprintln(file, "\n## Their last seen bag")

	_, _ = fmt.Fprintln(file, "\n| Bag | Date in UTC | Chat |")

	_, _ = fmt.Fprint(file, "|-------|-------|-------|")

	_, _ = fmt.Fprintf(file, "\n| %s | %s | %s |",
		Profile.Bag.Bag,
		Profile.Bag.Date.Format("2006-01-02 15:04:05"),
		fmt.Sprintf("![%s](https://raw.githubusercontent.com/blableblup/gofish/main/images/players/%s.png)", Profile.Bag.Chat, Profile.Bag.Chat))

	_, _ = fmt.Fprintln(file, "\n\nCount of each item in that bag:")

	sortedBagCounts := sortMapString(Profile.BagCounts, "countdesc")

	for _, bagItem := range sortedBagCounts {
		_, _ = fmt.Fprintf(file, " [%s %d]",
			bagItem,
			Profile.BagCounts[bagItem])
	}

	return nil
}
