package data

import (
	"database/sql"
	"fmt"
	"gofish/logs"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PlayerData struct {
	PlayerID int
	TwitchID int

	confirmedDates Dates
}

type Dates struct {
	HighestDate time.Time
	LowestDate  time.Time
}

// all the data about the player in my playerdata table
type PlayerDataInDB struct {
	PlayerID int
	TwitchID sql.NullInt64
	Name     string
	OldNames []string
}

// some twitchids
// 212b_ 212003898

func ConfirmWhoIsWho(fishes []FishInfo, pool *pgxpool.Pool) (map[string][]PlayerData, error) {

	confirmedPlayers := make(map[string][]PlayerData)

	playersFishingDate, playerURLs, firstFishChats, err := GetAllThePlayerNamesAndWhenTheyFished(fishes, pool)
	if err != nil {
		return confirmedPlayers, err
	}

	// to not get rate limited from the thingy
	var playerCount int
	pause := time.Second * 60

	logs.Logs().Info().
		Int("Players", len(playersFishingDate)).
		Msg("Going over all the players for playerids.....")

	for player := range playersFishingDate {

		playerCount++
		if playerCount == 100 {
			playerCount = 0
			logs.Logs().Info().
				Msg("Pausing going over the players ....")
			time.Sleep(pause)
		}

		// the player has more than 1 dates, if there is a 6 month break in the fish data for them
		// and now need to find out if the name was used by other players before
		if len(playersFishingDate[player]) > 1 {
			logs.Logs().Warn().
				Str("Player", player).
				Interface("Dates", playersFishingDate[player]).
				Msg("Player name has more than 1 dates!")
		}

		for _, dates := range playersFishingDate[player] {

			// check now if highest date in the checked data is more than 6 months away
			months, years, err := DiffBetweenTwoDates(time.Now(), dates.HighestDate, pool)
			if err != nil {
				return confirmedPlayers, err
			}

			// find twitchid for player name
			var twitchID int
			var playerInApi bool
			var userData []map[string]any

			if months >= 6 || years != 0 {

				// if it is more than 6 months away
				// check raw logs page
				var found bool

				twitchID, found, err = PlayerNotRecent(player, dates, playerURLs[player])
				if err != nil {
					return confirmedPlayers, err
				}

				if !found {
					// now if twitchid isnt found....
					// check again if there are players who used name as current name
					// or old name
					// because they opted out of justlog
					// but this could mean that if the name is now being used by someone else
					// this will be wrong
					twitchID, found, err = PlayerCheckNo6Months(player, dates, pool)
					if err != nil {
						return confirmedPlayers, err
					}

					if !found {
						// idk
						logs.Logs().Error().
							Str("Player", player).
							Interface("Dates", dates).
							Msg("Cant find anyone for player :(")
					}
				}

			} else {

				twitchID, userData, playerInApi, err = PlayerRecent(player, dates, playerURLs[player], pool)
				if err != nil {
					return confirmedPlayers, err
				}
			}

			// now check if there is already a player in the db with that twitchid

			DBData, TwitchIDExists, err := CheckForTwitchIDInDB(twitchID, pool)
			if err != nil {
				logs.Logs().Error().Err(err).
					Str("Player", player).
					Msg("Error checking twitchID in DB for player")
				return confirmedPlayers, err
			}

			var playerID int

			if TwitchIDExists {

				if playerInApi {
					// playerinapi means that there was a player in twitchapi for the player name
					// now only need to check if need to update player name in playerdata
					// because it cant be an old name since they are using it rn
					currentName := GetCurrentName(userData)

					if currentName != DBData.Name {

						err = RenamePlayer(currentName, DBData.Name, twitchID, DBData.PlayerID, pool)
						if err != nil {
							logs.Logs().Error().Err(err).
								Str("Player", player).
								Int("twitchID", twitchID).
								Msg("Error renaming player")
							return confirmedPlayers, err
						}
					}

				} else {

					// check current name for twitchid
					userData, err := MakeApiRequestForPlayerToApiIVR("", twitchID, "id")
					if err != nil && err != ErrNoPlayerFound {

						logs.Logs().Error().Err(err).
							Str("Player", player).
							Msg("Error doing api ivr thing")
						return confirmedPlayers, err

					}

					// there can be no player found for twitchid if they were nuked ? or deleted account i guess
					if err == nil {

						currentName := GetCurrentName(userData)

						var renamed bool

						if currentName != DBData.Name {

							renamed = true

							err = RenamePlayer(currentName, DBData.Name, twitchID, DBData.PlayerID, pool)
							if err != nil {
								logs.Logs().Error().Err(err).
									Str("Player", player).
									Int("twitchID", twitchID).
									Msg("Error renaming player")
								return confirmedPlayers, err
							}

						}

						if player != currentName {

							if renamed {
								// add this to old names if they were renamed
								DBData.OldNames = append(DBData.OldNames, DBData.Name)
							}

							// append the name to the players old names (if it doesnt already exist there)
							// this doesnt append the name again, if the player used their old name multiple times though
							var oldNameExists bool

							if slices.Contains(DBData.OldNames, player) {
								oldNameExists = true
							}

							if !oldNameExists {
								err = AppendOldName(DBData.Name, player, twitchID, DBData.PlayerID, pool)
								if err != nil {
									logs.Logs().Error().Err(err).
										Str("Player", player).
										Int("twitchID", twitchID).
										Msg("Error adding old name for player")
									return confirmedPlayers, err
								}
							}
						}
					}
				}

				playerID = DBData.PlayerID

			} else {

				// if twitchid doesnt exist in db; add the new player
				firstFishDate := dates.LowestDate

				// also: this firstfishdate wont be the exact date of their first fish
				// it will be the year month day witouth the hh:mm:ss
				// but idk im not using that for anything

				playerID, err = AddNewPlayer(twitchID, player, firstFishDate, firstFishChats[player], pool)
				if err != nil {
					logs.Logs().Error().Err(err).
						Str("Player", player).
						Msg("Error adding new player to DB")
					return confirmedPlayers, err
				}
			}

			// add/remove 6 months from the highest / lowest date
			// tis is 7 * 24 * 4 * 6 idk 4032 hours
			datesEdited := Dates{
				HighestDate: dates.HighestDate.Add(time.Hour * 4032),
				LowestDate:  dates.LowestDate.Add(time.Hour * -4032),
			}

			dataPlayer := PlayerData{
				TwitchID:       twitchID,
				PlayerID:       playerID,
				confirmedDates: datesEdited,
			}

			confirmedPlayers[player] = append(confirmedPlayers[player], dataPlayer)

		}
	}

	logs.Logs().Info().
		Msg("Finished..")

	return confirmedPlayers, nil
}

func PlayerRecent(player string, dates Dates, urls map[time.Time][]string, pool *pgxpool.Pool) (int, []map[string]any, bool, error) {

	var twitchID int
	var playerInApi bool

	userdata, err := MakeApiRequestForPlayerToApiIVR(player, 0, "name")
	if err != nil && err != ErrNoPlayerFound {

		logs.Logs().Error().Err(err).
			Str("Player", player).
			Msg("Error doing api ivr thing")
		return 0, userdata, false, err

	} else if err == ErrNoPlayerFound {

		// if there is no player in the api currently using that name
		// check if the last time someone used that name in the db was not more than 6 months ago

		// check for players who have that name as current name first
		playersNameDB, err := GetPlayersForPlayerName(player, pool)
		if err != nil {
			return 0, userdata, false, err
		}

		var foundAPlayer bool

		for _, playerName := range playersNameDB {

			lastSeen, _, err := PlayerDates(pool, playerName.PlayerID, player)
			if err != nil {
				return 0, userdata, false, err
			}

			months, years, err := DiffBetweenTwoDates(time.Now(), lastSeen, pool)
			if err != nil {
				return 0, userdata, false, err
			}

			if months < 6 && years == 0 {
				// if its not more than 6 months ago, it has to be this player
				twitchID = int(playerName.TwitchID.Int64)
				foundAPlayer = true

				logs.Logs().Info().
					Str("Player", player).
					Int("TwitchID", twitchID).
					Int("PlayerID", playerName.PlayerID).
					Msg("Found player as current name")

				break
			}
		}

		if !foundAPlayer {
			// if no player found, check for players who have that name as an oldname
			playersOldNameDB, err := GetPlayersForOldName(player, pool)
			if err != nil {
				return 0, userdata, false, err
			}

			for _, oldPlayer := range playersOldNameDB {

				lastSeen, _, err := PlayerDates(pool, oldPlayer.PlayerID, player)
				if err != nil {
					return 0, userdata, false, err
				}

				months, years, err := DiffBetweenTwoDates(time.Now(), lastSeen, pool)
				if err != nil {
					return 0, userdata, false, err
				}

				if months < 6 && years == 0 {

					twitchID = int(oldPlayer.TwitchID.Int64)

					logs.Logs().Info().
						Str("Player", player).
						Int("TwitchID", twitchID).
						Int("PlayerID", oldPlayer.PlayerID).
						Msg("Found player as old name")

					break
				}
			}
		}

	} else if err == nil {

		// use twitchid of the player from the api if there is one
		twitchID, err = GetTwitchID(userdata)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("Player", player).
				Msg("Error getting twitchID for player")
			return 0, userdata, false, err
		}
		playerInApi = true
	}

	if twitchID == 0 {
		// if there is still noone found for that name
		// check raw logs page
		logs.Logs().Error().
			Str("Player", player).
			Msg("No player found for recent player!")

		var found bool

		twitchID, found, err = PlayerNotRecent(player, dates, urls)
		if err != nil {
			return twitchID, userdata, playerInApi, err
		}

		if !found {
			logs.Logs().Error().
				Str("Player", player).
				Msg("Cant find recent player in raw logs aswell!!!!!")
			// idk how this can happen
		} else {
			logs.Logs().Info().
				Str("Player", player).
				Int("TwitchID", twitchID).
				Msg("Found recent player in raw logs")
		}

	}

	return twitchID, userdata, playerInApi, nil
}

func PlayerNotRecent(player string, dates Dates, playerURLs map[time.Time][]string) (int, bool, error) {

	if len(playerURLs[dates.HighestDate]) == 0 {
		logs.Logs().Error().
			Str("Player", player).
			Str("Date", dates.HighestDate.Format("2006-01-2 15:04:05")).
			Msg("No URL for date for player for raw logs page!!!!")
	}

	// twitchid should be the same between instances
	// else this can be weird
	// also only need to check either highest or lowest date
	// since both should be same player
	// because they cant be more than 6 months apart
	// because that is being checked in another function

	day := dates.HighestDate.Day()
	month := dates.HighestDate.Month()
	year := dates.HighestDate.Year()

	var twitchID int
	var thereIsTwitchID bool

	for _, instance := range playerURLs[dates.HighestDate] {

		// change the url to be of the channel instead of from the bot
		urlSplit := strings.Split(instance, "/")

		instanceURL := fmt.Sprintf("%s/%s/%s/%s/%s", urlSplit[0], urlSplit[1], urlSplit[2], urlSplit[3], urlSplit[4])

		url := fmt.Sprintf("%s/%d/%d/%d?raw=true", instanceURL, year, month, day)

		var foundTwitchID bool
		var err error

		twitchID, foundTwitchID, err = CheckRawLogsForPlayer(player, url)
		if err != nil {
			logs.Logs().Error().Err(err).
				Str("URL", url).
				Str("Player", player).
				Msg("Error checking raw logs twitch page for player!!!")
			return twitchID, thereIsTwitchID, err
		}

		if foundTwitchID {
			thereIsTwitchID = true
			break
		}

	}

	if !thereIsTwitchID {
		logs.Logs().Error().
			Str("Player", player).
			Msg("Couldnt find twitchID for player in all raw logs URLs!!!!!!")
	}

	return twitchID, thereIsTwitchID, nil
}

func PlayerCheckNo6Months(player string, dates Dates, pool *pgxpool.Pool) (int, bool, error) {

	playersNameDB, err := GetPlayersForPlayerName(player, pool)
	if err != nil {
		return 0, false, err
	}

	var foundAPlayer bool
	var twitchID int

	for _, playerName := range playersNameDB {

		lastSeen, firstSeen, err := PlayerDates(pool, playerName.PlayerID, player)
		if err != nil {
			return 0, false, err
		}

		// add / remove 6 months from their last and firstseen
		if dates.LowestDate.After(firstSeen.Add(time.Hour*-4032)) && dates.HighestDate.Before(lastSeen.Add(time.Hour*4032)) {
			twitchID = int(playerName.TwitchID.Int64)
			foundAPlayer = true

			logs.Logs().Info().
				Str("Player", player).
				Int("TwitchID", twitchID).
				Int("PlayerID", playerName.PlayerID).
				Msg("Found player as current name")

			break
		}
	}

	if !foundAPlayer {

		playersOldNameDB, err := GetPlayersForOldName(player, pool)
		if err != nil {
			return 0, false, err
		}

		for _, oldPlayer := range playersOldNameDB {

			lastSeen, firstSeen, err := PlayerDates(pool, oldPlayer.PlayerID, player)
			if err != nil {
				return 0, false, err
			}

			if dates.LowestDate.After(firstSeen.Add(time.Hour*-4032)) && dates.HighestDate.Before(lastSeen.Add(time.Hour*4032)) {
				twitchID = int(oldPlayer.TwitchID.Int64)
				foundAPlayer = true

				logs.Logs().Info().
					Str("Player", player).
					Int("TwitchID", twitchID).
					Int("PlayerID", oldPlayer.PlayerID).
					Msg("Found player as old name")

				break
			}
		}
	}

	return twitchID, foundAPlayer, nil
}

func GetAllThePlayerNamesAndWhenTheyFished(fishes []FishInfo, pool *pgxpool.Pool) (map[string][]Dates, map[string]map[time.Time][]string, map[string]string, error) {

	playersFishingDate := make(map[string][]Dates)
	playersFishingFish := make(map[string][]FishInfo)
	firstFishChats := make(map[string]string)
	playerURLs := make(map[string]map[time.Time][]string)
	var err error

	// first add the player to the map and get the fish that name caught
	for _, fish := range fishes {

		// the fish are sorted by date ascending, so this will be first fish chat
		// if the player name was used by different players in the checked data this can be wrong
		// if they are new
		if _, ok := firstFishChats[fish.Player]; !ok {
			firstFishChats[fish.Player] = fish.Chat
		}

		playersFishingFish[fish.Player] = append(playersFishingFish[fish.Player], fish)
	}

	for player := range playersFishingFish {

		playersFishingDate[player], playerURLs[player], err = AllTheDaysAPlayerFished(playersFishingFish[player], pool)
		if err != nil {
			return playersFishingDate, playerURLs, firstFishChats, err
		}
	}

	return playersFishingDate, playerURLs, firstFishChats, nil
}

func AllTheDaysAPlayerFished(fishes []FishInfo, pool *pgxpool.Pool) ([]Dates, map[time.Time][]string, error) {

	var days []Dates

	oneday := (time.Hour * 24)

	// to store from which instance the fish came from
	// it is []string since there are multiple instances for a chat
	// this can add multiple urls from same instance for same day
	playerURLs := make(map[time.Time][]string)

	var highestDay, lowestDay, lastDay time.Time
	var resetAtleastOnce bool

	for _, fish := range fishes {

		day := fish.Date.Truncate(oneday)
		lastDay = day

		if len(days) == 0 {
			playerURLs[day] = append(playerURLs[day], fish.Url)
			highestDay = day
			lowestDay = day
		} else {
			if day.Before(lowestDay) {
				lowestDay = day
				playerURLs[day] = append(playerURLs[day], fish.Url)
			}
			if day.After(highestDay) {
				highestDay = day
				playerURLs[day] = append(playerURLs[day], fish.Url)
			}
		}

		// if diff above 6 months; this could be a different player using that name
		months, years, err := DiffBetweenTwoDates(day, lastDay, pool)
		if err != nil {
			return days, playerURLs, err
		}

		if months > 6 || years > 0 {

			resetAtleastOnce = true

			newDays := Dates{
				HighestDate: highestDay,
				LowestDate:  lowestDay,
			}

			days = append(days, newDays)

			// reset the thing idk
			highestDay, lowestDay = day, day
		}

	}

	// if the player never stopped fishing for more than 6 months in the checked data
	// or if the thing was reset
	if len(days) == 0 || resetAtleastOnce {
		newDays := Dates{
			HighestDate: highestDay,
			LowestDate:  lowestDay,
		}

		days = append(days, newDays)
	}

	return days, playerURLs, nil
}
