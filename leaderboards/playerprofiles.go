package leaderboards

import (
	"context"
	"database/sql"
	"fmt"
	"gofish/logs"
	"gofish/utils"
	"path/filepath"
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
	Records  []string
	// right now shiny is the only other achievment
	Other OtherAchievements `json:"Other accomplishments"`

	Stars        int                  `json:"-"`
	StarsGlow    int                  `json:"-"`
	Treasures    TreasureProgress     `json:"-"`
	SonnyDay     SonnyDayProgress     `json:"-"`
	MythicalFish MythicalFishProgress `json:"-"`
	Birds        BirdProgress         `json:"-"`
	Flowers      FlowerProgress       `json:"-"`

	Count          *TotalChatStruct            `json:"Fish caught in total"`
	CountYear      map[string]*TotalChatStruct `json:"Fish caught per year"`
	CountCatchtype map[string]*TotalChatStruct `json:"Fish caught per catchtype"`

	FirstFishChat   map[string]ProfileFish `json:"Their first fish per chat"`
	LastFishChat    map[string]ProfileFish `json:"Their last fish per chat"`
	BiggestFishChat map[string]ProfileFish `json:"Their biggest fish per chat"`

	BiggestFish []ProfileFish `json:"Their overall biggest fish"`
	LastFish    []ProfileFish `json:"Their overall last fish"`

	Bag       ProfileBag     `json:"Their last seen bag"`
	BagCounts map[string]int `json:"Count of each item in that bag"`

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

// can maybe eventually add the big mammals from big as progress (?)

type OtherAchievements struct {
	Other      []string      `json:"Accomplishments"`
	HasShiny   bool          `json:"-"`
	ShinyCatch []ProfileFish `json:"Shinies"`
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
	CatchType  string `json:"Catchtype,omitempty"`
	DateString string `json:"Date,omitempty"`
	Chat       string `json:"Chat,omitempty"`

	// these are to scan the data into the struct
	// but arent printed out in the end
	Bag      []string  `json:"-"`
	Count    int       `json:"-"`
	PlayerID int       `json:"-"`
	TypeName string    `json:"-"`
	ChatPfp  string    `json:"-"`
	Url      string    `json:"-"`
	Date     time.Time `json:"-"`
}

// struct for each fish types data
type ProfileFishData struct {
	TotalCount     *TotalChatStruct            `json:"Caught in total"`
	CountYear      map[string]*TotalChatStruct `json:"Caught per year"`
	CountCatchtype map[string]*TotalChatStruct `json:"Caught per catchtype"`

	First    ProfileFish       `json:"First catch"`
	Last     ProfileFish       `json:"Last catch"`
	Biggest  TypeRecordProfile `json:"Biggest catch"`
	Smallest TypeRecordProfile `json:"Smallest catch"`
}

// to make it show if that fish is a record somewhere
type TypeRecordProfile struct {
	Fish     ProfileFish
	IsRecord []string `json:"Record,omitempty"`
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
	// this is only for if a player has a shiny in their bag so that it shows the emote
	// if a shiny is in any other table on the profile, it will not show the shiny but the emote of the fishname instead
	allShinies, err := GetAllShinies(params)
	if err != nil {
		return
	}

	fishLists["shiny"] = allShinies

	// Get the treasures, the mythical fish, the birds, the flowers

	tags := []string{"r.a.treasure", "mythic", "bird", "flower"}

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

	wg := new(sync.WaitGroup)

	for _, validPlayer := range validPlayers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			err = PrintPlayerProfile(playerProfiles[validPlayer], FishWithEmoji, fishLists)
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

func PrintPlayerProfile(Profile *PlayerProfile, EmojisForFish map[string]string, fishLists map[string][]string) error {

	if Profile.TwitchID == 0 {
		return nil
	}

	filePath := filepath.Join("leaderboards", "global", "players", fmt.Sprintf("%d", Profile.TwitchID)+".json")

	// update the progress

	// stars glow is for the rarer stuff
	// and the normal star for less rare things or unfinished stuff

	// this means that they caught them atleast once
	// doesnt mean that they still have them in their bag
	if Profile.MythicalFish.HasAllOriginalMythicalFish {
		baseText := "🌟 Has encountered all the mythical fish:"
		for _, fish := range fishLists["mythic"] {
			baseText = baseText + " " + EmojisForFish[fish] + " " + fish
		}
		baseText = baseText + " !"
		Profile.Progress = append(Profile.Progress, baseText)
	}

	if Profile.Treasures.HasAllRedAveryTreasure {
		baseText := "🌟 Has found all the treasures from legendary pirate Red Avery:"
		for _, fish := range fishLists["r.a.treasure"] {
			baseText = baseText + " " + EmojisForFish[fish] + " " + fish
		}
		baseText = baseText + " !"
		Profile.Progress = append(Profile.Progress, baseText)
	}

	if Profile.Birds.HasAllBirds {
		baseText := "🌟 Has seen all the birds:"
		for _, fish := range fishLists["bird"] {
			baseText = baseText + " " + EmojisForFish[fish] + " " + fish
		}
		baseText = baseText + " !"
		Profile.Progress = append(Profile.Progress, baseText)
	}

	// received means when it first appeared in their bag
	if Profile.SonnyDay.HasLetter {
		Profile.Progress = append(Profile.Progress,
			fmt.Sprintf("⭐ Has gotten a letter ✉️ ! (Received: %s UTC)", Profile.SonnyDay.LetterInBagReceived.Format("2006-01-02 15:04:05")))
	}

	// no star, since it isnt really rare
	// just means you've been at acorn pond all four seasons
	if Profile.Flowers.HasAllFlowers {
		baseText := "💐 Has seen all the flowers:"
		for _, fish := range fishLists["flower"] {
			baseText = baseText + " " + EmojisForFish[fish] + " " + fish
		}
		baseText = baseText + " !"
		Profile.Progress = append(Profile.Progress, baseText)
	}

	// add some notes to the bottom
	Profile.InfoBottom = append(Profile.InfoBottom,
		"If there are multiple catches with the same weight for biggest and smallest fish per type, it will only show the first catch with that weight.")

	Profile.InfoBottom = append(Profile.InfoBottom,
		"If the player has multiple catches as biggest / smallest fish per type records in different channels they wont show. It will only show if their current biggest or smallest fish per type is a record.")

	Profile.InfoBottom = append(Profile.InfoBottom,
		"The records at the top and the records per fish type will only show records from channels which have their own leaderboards.")

	Profile.InfoBottom = append(Profile.InfoBottom,
		"The players biggest or smallest catch of a fish type can be nothing, if the player only caught the fish through catches which do not show the weight in the catch message.")

	Profile.InfoBottom = append(Profile.InfoBottom,
		"Release bonus and jumped bonus catches and normal squirrels will show a weight of 0, even though they have a weight, but it is not shown in the catch message.")

	Profile.InfoBottom = append(Profile.InfoBottom,
		"For the progress, the profile does not check if the player still has the fish in their bag. The player needs to have caught them atleast once.")

	// update the last updated
	Profile.LastUpdated = time.Now().In(time.UTC).Format("2006-01-02 15:04:05 UTC")

	// print it
	err := writeRaw(filePath, Profile)
	if err != nil {
		return err
	}

	return nil
}
