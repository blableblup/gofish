package leaderboards

import (
	"fmt"
	"gofish/logs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

func PrintPlayerProfile(Profile *PlayerProfile, EmojisForFish map[string]string, fishLists map[string][]string) error {

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
		"If the player has multiple catches as fish type records in different channels they wont show. It will only show if one of their current catches is a record.")

	Profile.InfoBottom = append(Profile.InfoBottom,
		"The records at the top and the records per fish type will only show records from channels which have their own leaderboards.")

	Profile.InfoBottom = append(Profile.InfoBottom,
		"The players biggest or smallest catch of a fish type can be nothing, if the player only caught the fish through catches which do not show the weight in the catch message.")

	Profile.InfoBottom = append(Profile.InfoBottom,
		"Catches which do not show their weight in the catch message will show a weight of 0, even though they have a weight.")

	// update the last updated
	Profile.LastUpdated = time.Now().In(time.UTC).Format("2006-01-02 15:04:05 UTC")

	// print it
	err := PrintPlayerProfileMD(Profile, EmojisForFish)
	if err != nil {
		return err
	}

	return nil
}

func PrintPlayerProfileMD(Profile *PlayerProfile, EmojisForFish map[string]string) error {

	filePath := filepath.Join("leaderboards", "global", "profiles", fmt.Sprintf("%d", Profile.TwitchID)+".md")

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, _ = fmt.Fprintf(file, "# 🎣 %s\n", Profile.Name)

	PrintSliceMD(Profile.Progress, "## Accomplishments", file)

	_, _ = fmt.Fprintln(file)

	PrintSliceMD(Profile.Records, "## Noteable records", file)

	_, _ = fmt.Fprintln(file)

	PrintSliceMD(Profile.Other.Other, "## Other stuff", file)

	_, _ = fmt.Fprintln(file)

	if Profile.Other.HasShiny {
		err = PrintTableMD(Profile.Other.ShinyCatch, []string{"Fish", "Weight in lbs", "Catchtype", "Date", "Chat"}, "", "profilefishslice", false, file)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(file)
	}

	_, _ = fmt.Fprintln(file, "---------------")

	_, _ = fmt.Fprintln(file, "\n## Data for their fish caught 🪣")

	err = PrintTableMD(Profile.Count.Total, []string{"Total fish caught"}, "", "notslice", false, file)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(file)

	err = PrintTableMD(Profile.Count, []string{"-", "Chat", "Fish caught"}, "Fish caught per chat", "totalchat", true, file)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(file)

	err = PrintTableMD(Profile.CountYear, []string{"-", "Year", "Count", "Chat"}, "Fish caught per year", "totalchatmap", true, file)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(file)

	err = PrintTableMD(Profile.CountCatchtype, []string{"-", "Catchtype", "Count", "Chat"}, "Fish caught per catchtype", "totalchatmap", true, file)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(file)

	err = PrintTableMD(Profile.TotalWeight.Total, []string{"Total weight of all caught fish in lbs"}, "", "notslice", false, file)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(file)

	err = PrintTableMD(Profile.TotalWeight, []string{"-", "Chat", "Total weight in lbs"}, "Total weight per chat", "totalchatfloat", true, file)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(file)

	err = PrintTableMD(Profile.TotalWeightYear, []string{"-", "Year", "Total weight in lbs", "Chat"}, "Total weight per year", "totalchatmapfloat", true, file)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(file, "\n---------------")

	_, _ = fmt.Fprintln(file, "\n## First, biggest and last fish ⚖️")

	err = PrintTableMD(Profile.FirstFishChat, []string{"Chat", "Fish", "Weight in lbs", "Catchtype", "Date"}, "### First ever fish caught per chat", "mapstringprofilefish", false, file)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(file)

	err = PrintTableMD(Profile.LastFishChat, []string{"Chat", "Fish", "Weight in lbs", "Catchtype", "Date"}, "### Last fish caught per chat", "mapstringprofilefish", false, file)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(file)

	err = PrintTableMD(Profile.BiggestFishChat, []string{"Chat", "Fish", "Weight in lbs", "Catchtype", "Date"}, "### Biggest fish caught per chat", "mapstringprofilefish", false, file)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(file)

	err = PrintTableMD(Profile.BiggestFish, []string{"Fish", "Weight in lbs", "Catchtype", "Date", "Chat"}, "### Overall biggest fish", "profilefishslice", false, file)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(file)

	err = PrintTableMD(Profile.LastFish, []string{"Fish", "Weight in lbs", "Catchtype", "Date", "Chat"}, "### Overall last fish", "profilefishslice", false, file)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(file, "\n---------------")

	_, _ = fmt.Fprintln(file, "\n## Their last seen bag 🎒")

	err = PrintTableMD(Profile.Bag, []string{"Bag", "Date", "Chat"}, "", "notslice", false, file)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(file, "\n---------------")

	_, _ = fmt.Fprintln(file, "\n## Their fish seen")

	err = PrintTableMD(Profile.FishSeenTotal.Total, []string{"Total fish seen"}, "", "notslice", false, file)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(file)

	err = PrintTableMD(Profile.FishSeenTotal, []string{"-", "Chat", "Fish seen"}, "Fish seen per chat", "totalchat", true, file)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprint(file, "\n<details>")

	_, _ = fmt.Fprint(file, "\n<summary>Fish never seen</summary>")

	PrintSliceMD(Profile.FishNotSeen, "", file)

	_, _ = fmt.Fprintf(file, "\nIn total %d fish never seen", len(Profile.FishNotSeen))

	_, _ = fmt.Fprint(file, "\n</details>\n")

	_, _ = fmt.Fprintln(file, "\n---------------")

	_, _ = fmt.Fprintln(file, "\n## Data about each of their seen fish")

	// sort fish seen by name and print all their data

	sort.SliceStable(Profile.FishSeen, func(i, j int) bool { return Profile.FishSeen[i] < Profile.FishSeen[j] })

	for _, fish := range Profile.FishSeen {

		fishType := fmt.Sprintf("%s %s", fish, EmojisForFish[fish])

		_, _ = fmt.Fprintf(file, "\n## %s %s", EmojisForFish[fish], fish)

		_, _ = fmt.Fprintln(file)

		_, _ = fmt.Fprint(file, "\n<details>")

		_, _ = fmt.Fprint(file, "\n<summary>Fish data</summary>\n\n")

		err = PrintTableMD(Profile.FishData[fishType].TotalCount.Total, []string{"Caught in total"}, "", "notslice", false, file)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintln(file)

		err = PrintTableMD(Profile.FishData[fishType].TotalCount, []string{EmojisForFish[fish], "Chat", "Fish caught"}, "### Fish caught per chat", "totalchat", false, file)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintln(file)

		err = PrintTableMD(Profile.FishData[fishType].CountYear, []string{EmojisForFish[fish], "Year", "Count", "Chat"}, "### Fish caught per year", "totalchatmap", false, file)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintln(file)

		err = PrintTableMD(Profile.FishData[fishType].CountCatchtype, []string{EmojisForFish[fish], "Catchtype", "Count", "Chat"}, "### Fish caught per catchtype", "totalchatmap", false, file)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintln(file)

		err = PrintTableMD(Profile.FishData[fishType], []string{EmojisForFish[fish], "Weight in lbs", "Catchtype", "Date", "Chat"}, "", "profilefishdata", false, file)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprint(file, "\n</details>\n")

		fishBlock := []ProfileFish{
			Profile.FishData[fishType].First,
			Profile.FishData[fishType].Last,
			Profile.FishData[fishType].Biggest,
			Profile.FishData[fishType].Smallest,
		}

		var fishRecords []string

		for _, fish := range fishBlock {
			if len(fish.Record) != 0 {
				fishRecords = append(fishRecords, fish.Record...)
			}
		}

		if len(fishRecords) != 0 {
			PrintSliceMD(fishRecords, "", file)
		}

		_, _ = fmt.Fprintln(file, "\n---------------")

	}

	PrintSliceMD(Profile.InfoBottom, "## Some info", file)

	_, _ = fmt.Fprintln(file)

	_, _ = fmt.Fprintf(file, "_Profile last updated at %s_\n", Profile.LastUpdated)

	return nil
}

func PrintTableMD(data any, header []string, title string, what string, hide bool, file *os.File) error {

	if hide {
		_, _ = fmt.Fprint(file, "<details>")

		if title != "" {
			_, _ = fmt.Fprintf(file, "\n<summary>%s</summary>\n\n", title)
		} else {
			_, _ = fmt.Fprint(file, "\n<summary>Summary</summary>\n\n")
		}

	} else {
		if title != "" {
			_, _ = fmt.Fprintln(file, title)
		}
	}

	table := tablewriter.NewTable(file,
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Alignment: tw.CellAlignment{
					Global: tw.AlignLeft,
				},
				Formatting: tw.CellFormatting{
					AutoFormat: tw.Off,
				},
			},
			Row: tw.CellConfig{
				Alignment: tw.CellAlignment{
					Global: tw.AlignLeft,
				},
			},
		}),
		tablewriter.WithRenderer(renderer.NewMarkdown()),
	)

	table.Header(header)

	var err error

	switch what {
	default:
		logs.Logs().Error().Msg("IDK WHAT TO DO PrintTableMD")

	case "profilefishslice":
		for _, fish := range data.([]ProfileFish) {
			err = table.Append(
				fish.Fish,
				fish.Weight,
				fish.CatchType,
				fish.DateString,
				fish.Chat,
			)
			if err != nil {
				return err
			}
		}

	case "notslice":

		err = table.Append(data)
		if err != nil {
			return err
		}

	case "mapstringprofilefish":

		// sort them by name
		slice := make([]string, 0, len(data.(map[string]ProfileFish)))
		for whatever := range data.(map[string]ProfileFish) {
			slice = append(slice, whatever)
		}

		sort.SliceStable(slice, func(i, j int) bool { return slice[i] < slice[j] })

		for _, stuff := range slice {
			err = table.Append(
				data.(map[string]ProfileFish)[stuff].Chat,
				data.(map[string]ProfileFish)[stuff].Fish,
				data.(map[string]ProfileFish)[stuff].Weight,
				data.(map[string]ProfileFish)[stuff].CatchType,
				data.(map[string]ProfileFish)[stuff].DateString,
			)
			if err != nil {
				return err
			}
		}

	case "totalchat":

		rank := 1
		prevRank := 1
		prevCount := -1
		occupiedRanks := make(map[int]int)

		// sort them by count and rank them
		sortedCounts := sortMapStringInt(data.(*TotalChatStruct).Chat, "countdesc")

		for _, chat := range sortedCounts {

			if data.(*TotalChatStruct).Chat[chat] != prevCount {
				rank += occupiedRanks[rank]
				occupiedRanks[rank] = 1
			} else {
				rank = prevRank
				occupiedRanks[rank]++
			}

			err = table.Append(rank, chat, data.(*TotalChatStruct).Chat[chat])
			if err != nil {
				return err
			}

			prevCount = data.(*TotalChatStruct).Chat[chat]
			prevRank = rank
		}

	case "totalchatfloat":

		rank := 1
		prevRank := 1
		prevWeight := 0.0
		occupiedRanks := make(map[int]int)

		// sort them by weight and rank them
		slice := make([]string, 0, len(data.(*TotalChatStructFloat).Chat))
		for chat := range data.(*TotalChatStructFloat).Chat {
			slice = append(slice, chat)
		}

		sort.SliceStable(slice, func(i, j int) bool {
			return data.(*TotalChatStructFloat).Chat[slice[i]] > data.(*TotalChatStructFloat).Chat[slice[j]]
		})

		for _, chat := range slice {

			if data.(*TotalChatStructFloat).Chat[chat] != prevWeight {
				rank += occupiedRanks[rank]
				occupiedRanks[rank] = 1
			} else {
				rank = prevRank
				occupiedRanks[rank]++
			}

			err = table.Append(rank, chat, data.(*TotalChatStructFloat).Chat[chat])
			if err != nil {
				return err
			}

			prevWeight = data.(*TotalChatStructFloat).Chat[chat]
			prevRank = rank
		}

	case "totalchatmap":

		// sort the year first
		sortedCounts := make([]string, 0, len(data.(map[string]*TotalChatStruct)))
		for chat := range data.(map[string]*TotalChatStruct) {
			sortedCounts = append(sortedCounts, chat)
		}

		// by name if its year
		// by count if its catchtype
		if strings.Contains(title, "year") {
			sort.SliceStable(sortedCounts, func(i, j int) bool {
				return sortedCounts[i] < sortedCounts[j]
			})
		} else {
			sort.SliceStable(sortedCounts, func(i, j int) bool {
				return data.(map[string]*TotalChatStruct)[sortedCounts[i]].Total > data.(map[string]*TotalChatStruct)[sortedCounts[j]].Total
			})
		}

		rank := 1
		prevRank := 1
		prevCount := -1
		occupiedRanks := make(map[int]int)

		for _, year := range sortedCounts {

			if data.(map[string]*TotalChatStruct)[year].Total != prevCount {
				rank += occupiedRanks[rank]
				occupiedRanks[rank] = 1
			} else {
				rank = prevRank
				occupiedRanks[rank]++
			}

			// need to sort and append the chats and their counts
			sortedYearCounts := sortMapStringInt(data.(map[string]*TotalChatStruct)[year].Chat, "countdesc")

			var chats string

			for _, chat := range sortedYearCounts {
				chats = chats + fmt.Sprintf("%s: %d ", chat, data.(map[string]*TotalChatStruct)[year].Chat[chat])
			}

			err = table.Append(
				rank,
				year,
				data.(map[string]*TotalChatStruct)[year].Total,
				chats)
			if err != nil {
				return err
			}

			prevCount = data.(map[string]*TotalChatStruct)[year].Total
			prevRank = rank
		}

	case "totalchatmapfloat":

		// sort the year first
		sortedCounts := make([]string, 0, len(data.(map[string]*TotalChatStructFloat)))
		for chat := range data.(map[string]*TotalChatStructFloat) {
			sortedCounts = append(sortedCounts, chat)
		}

		sort.SliceStable(sortedCounts, func(i, j int) bool {
			return sortedCounts[i] < sortedCounts[j]
		})

		rank := 1
		prevRank := 1
		prevWeight := 0.0
		occupiedRanks := make(map[int]int)

		for _, year := range sortedCounts {

			if data.(map[string]*TotalChatStructFloat)[year].Total != prevWeight {
				rank += occupiedRanks[rank]
				occupiedRanks[rank] = 1
			} else {
				rank = prevRank
				occupiedRanks[rank]++
			}

			sortedYearCounts := make([]string, 0, len(data.(map[string]*TotalChatStructFloat)[year].Chat))
			for chat := range data.(map[string]*TotalChatStructFloat)[year].Chat {
				sortedYearCounts = append(sortedYearCounts, chat)
			}

			sort.SliceStable(sortedYearCounts, func(i, j int) bool {
				return data.(map[string]*TotalChatStructFloat)[year].Chat[sortedYearCounts[i]] > data.(map[string]*TotalChatStructFloat)[year].Chat[sortedYearCounts[j]]
			})

			var chats string

			for _, chat := range sortedYearCounts {
				chats = chats + fmt.Sprintf("%s: %.2f ", chat, data.(map[string]*TotalChatStructFloat)[year].Chat[chat])
			}

			err = table.Append(
				rank,
				year,
				data.(map[string]*TotalChatStructFloat)[year].Total,
				chats)
			if err != nil {
				return err
			}

			prevWeight = data.(map[string]*TotalChatStructFloat)[year].Total
			prevRank = rank
		}

	case "profilefishdata":

		fishBlock := []ProfileFish{
			data.(*ProfileFishData).First,
			data.(*ProfileFishData).Last,
			data.(*ProfileFishData).Biggest,
			data.(*ProfileFishData).Smallest,
		}

		things := []string{
			"First catch",
			"Last catch",
			"Biggest catch",
			"Smallest catch"}

		for i, fish := range fishBlock {

			err = table.Append(things[i],
				fish.Weight,
				fish.CatchType,
				fish.DateString,
				fish.Chat)
			if err != nil {
				return err
			}
		}

	}

	err = table.Render()
	if err != nil {
		return err
	}

	if hide {
		_, _ = fmt.Fprint(file, "\n</details>\n")
	}

	return nil
}

func PrintSliceMD(data any, header string, file *os.File) {

	if len(data.([]string)) != 0 {
		_, _ = fmt.Fprint(file, header)

		_, _ = fmt.Fprintln(file)

		for _, row := range data.([]string) {
			_, _ = fmt.Fprintln(file, "\n* ", row)
		}

	}

}
