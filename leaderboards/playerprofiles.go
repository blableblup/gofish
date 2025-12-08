package leaderboards

import (
	"context"
	"database/sql"
	"gofish/logs"
	"gofish/utils"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

type PlayerProfile struct {
	Name     string
	PlayerID int          `json:"-"`
	TwitchID int          `json:"-"`
	Verified sql.NullBool `json:"-"`

	Progress []string
	Records  []string `json:"Noteable records"`

	Other OtherAchievements `json:"Other accomplishments"`

	Treasures    TreasureProgress     `json:"-"`
	SonnyDay     SonnyDayProgress     `json:"-"`
	MythicalFish MythicalFishProgress `json:"-"`
	Birds        BirdProgress         `json:"-"`
	Flowers      FlowerProgress       `json:"-"`
	Bugs         BugsProgress

	Count          *TotalChatStruct            `json:"Fish caught in total"`
	CountYear      map[string]*TotalChatStruct `json:"Fish caught per year"`
	CountCatchtype map[string]*TotalChatStruct `json:"Fish caught per catchtype"`

	FirstFish     ProfileFish            `json:"First ever fish caught"`
	FirstFishChat map[string]ProfileFish `json:"Their first fish per chat"`

	BiggestFish     []ProfileFish          `json:"Their overall biggest fish"`
	BiggestFishChat map[string]ProfileFish `json:"Their biggest fish per chat"`

	LastFish     []ProfileFish          `json:"Their overall last fish"`
	LastFishChat map[string]ProfileFish `json:"Their last fish per chat"`

	TotalWeight     *TotalChatStructFloat            `json:"Combined weight of all caught fish"`
	TotalWeightYear map[string]*TotalChatStructFloat `json:"Combined weight of all caught fish per year"`

	Bag             ProfileBag     `json:"Their last seen bag"`
	BagCounts       map[string]int `json:"Count of each item in that bag"`
	BagCountsSorted []string

	FishSeen      []string         `json:"-"`
	FishSeenTotal *TotalChatStruct `json:"Fish seen in total"`

	FishData map[string]*ProfileFishData `json:"Data about each of their seen fish"`

	FishNotSeen      []string `json:"Fish they never saw"`
	FishNotSeenTotal int      `json:"Total fish not seen"`

	InfoBottom  []string `json:"Some info"`
	LastUpdated string   `json:"Profile last updated at"`
}

// for the counts which have a total and chats
type TotalChatStruct struct {
	Total int
	Chat  map[string]int `json:"Per chat"`
}

// for total weight and per chat
type TotalChatStructFloat struct {
	Total float64
	Chat  map[string]float64 `json:"Per chat"`
}

// all the structs for the progress thingys
type TreasureProgress struct {
	HasAllRedAveryTreasure bool
	RedAveryTreasureCount  int
}

type SonnyDayProgress struct {
	HasLetter           bool
	LetterInBagReceived time.Time
}

type MythicalFishProgress struct {
	HasAllOriginalMythicalFish bool
	OriginalMythicalFishCount  int
}

type BirdProgress struct {
	HasAllBirds bool
	BirdCount   int
}

type FlowerProgress struct {
	HasAllFlowers bool
	FLowerCount   int
}

type BugsProgress struct {
	HasAllBugs bool
	BugCount   int
}

type OtherAchievements struct {
	HasOtherStuff bool
	HasShiny      bool
	ShinyMessage  []string
	ShinyCatch    []ProfileFish
	HasPresents   bool
	Gifts         map[string]WinterGifts
}

type WinterGifts struct {
	Presents []string
	Date     string
	Chat     string
}

// different struct for bag so that it doesnt show weight
type ProfileBag struct {
	Bag        []string `json:"Bag,omitempty"`
	DateString string   `json:"Date,omitempty"`
	Chat       string   `json:"Chat,omitempty"`

	Date time.Time `json:"-"`
}

type ProfileFish struct {
	Fish   string  `json:"Fish,omitempty"`
	Weight float64 `json:"Weight in lbs"`
	// cant put omitempty for weight, else it wont show a weight for 0 lbs catches
	// but now this will show 0 lbs weight for release + jumped bonus + squirrel,
	// even though they have a weight, but i cant see it in log message
	CatchType  string   `json:"Catchtype,omitempty"`
	DateString string   `json:"Date,omitempty"`
	Chat       string   `json:"Chat,omitempty"`
	Record     []string `json:"Record,omitempty"`

	// these are for wrapped
	Rank        int `json:"Rank,omitempty"`
	RankAllTime int `json:"RankAllTime,omitempty"`

	// these are to scan the data into the struct
	// but arent printed out in the end
	Bag      []string  `json:"-"`
	Count    int       `json:"-"`
	PlayerID int       `json:"-"`
	FishID   int       `json:"-"`
	FishName string    `json:"-"`
	ChatPfp  string    `json:"-"`
	Url      string    `json:"-"`
	Date     time.Time `json:"-"`
}

// struct for each fish types data
type ProfileFishData struct {
	TotalCount     *TotalChatStruct            `json:"Caught in total"`
	CountYear      map[string]*TotalChatStruct `json:"Caught per year"`
	CountCatchtype map[string]*TotalChatStruct `json:"Caught per catchtype"`

	First    ProfileFish `json:"First catch"`
	Last     ProfileFish `json:"Last catch"`
	Biggest  ProfileFish `json:"Biggest catch"`
	Smallest ProfileFish `json:"Smallest catch"`
}

func GetPlayerProfiles(params LeaderboardParams) {
	board := params.LeaderboardType
	chatName := params.ChatName
	config := params.Config
	limit := params.Limit

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

	fishLists := make(map[string][]string)

	allFishes, err := GetAllFishNames(params)
	if err != nil {
		return
	}

	fishLists["all"] = allFishes

	FishWithEmoji := make(map[string]string)

	for _, fish := range allFishes {
		FishWithEmoji[fish], err = FishStuff(fish, params)
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
	// this is only to check if a player has a shiny in their bag so that it shows the emote
	// if a shiny is in any other table on the profile, it will not show the shiny but the emote of the fishname instead
	allShinies, err := GetAllShinies(params)
	if err != nil {
		return
	}

	fishLists["shiny"] = allShinies

	// Get the treasures, the mythical fish, the birds, the flowers, the buggs

	tags := []string{"r.a.treasure", "mythic", "bird", "flower", "bug"}

	for _, tag := range tags {

		fish, err := ReturnFishTags(params, tag)
		if err != nil {
			return
		}

		fishLists[tag] = fish
	}

	logs.Logs().Info().
		Int("Amount of players", len(validPlayers)).
		Msg("Updating player profiles")

	// Get the player profiles and print them for each player
	playerProfiles, err := GetThePlayerProfiles(params, FishWithEmoji, validPlayers, fishLists)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error getting player profiles")
		return
	}

	// add records from other leaderboards to the profiles
	playerProfiles, err = UpdatePlayerProfilesRecords(params, playerProfiles)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error updating player profiles records")
		return
	}

	type nameID struct {
		Name string
		ID   int
	}

	var nameIDs []nameID

	wg := new(sync.WaitGroup)
	mu := new(sync.Mutex)

	for _, validPlayer := range validPlayers {
		wg.Add(1)
		go func() {

			if playerProfiles[validPlayer].TwitchID != 0 {
				err = PrintPlayerProfile(playerProfiles[validPlayer], FishWithEmoji, fishLists)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Chat", chatName).
						Str("Board", board).
						Int("PlayerID", validPlayer).
						Msg("Error printing player profile")
				} else {
					var nameAndId nameID
					nameAndId.Name = playerProfiles[validPlayer].Name
					nameAndId.ID = playerProfiles[validPlayer].TwitchID
					mu.Lock()
					nameIDs = append(nameIDs, nameAndId)
					mu.Unlock()
				}
			}

			wg.Done()
		}()
	}

	wg.Wait()

	// Sort nameids by name
	sort.SliceStable(nameIDs, func(i, j int) bool {
		return nameIDs[i].Name < nameIDs[j].Name
	})

	// print out the json file with the names and their ids
	err = writeRaw(filepath.Join("leaderboards", "global", "profiles", "nameID"), nameIDs)
	if err != nil {
		logs.Logs().Error().Err(err).
			Str("Chat", chatName).
			Str("Board", board).
			Msg("Error writing nameID file")
		return
	}

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
