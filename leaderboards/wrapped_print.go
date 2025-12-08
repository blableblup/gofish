package leaderboards

import (
	"fmt"
	"os"
	"path/filepath"
)

func PrintWrapped(Wrapped *Wrapped, fishWithEmoji map[string]string) error {

	filePath := filepath.Join("leaderboards", "global", "wrappeds", Wrapped.Year, fmt.Sprintf("%d", Wrapped.TwitchID)+".md")

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, _ = fmt.Fprintf(file, "# 🎇 %s Gofish Wrapped for %s 🎇\n", Wrapped.Year, Wrapped.Name)

	_, _ = fmt.Fprintln(file, "---------------")

	emojiForCount := ReturnEmojiFishCaught(Wrapped.Count.Total)

	_, _ = fmt.Fprintf(file, "* You caught %d fish in this year %s\n", Wrapped.Count.Total, emojiForCount)

	sortedChats := sortMapStringInt(Wrapped.Count.Chat, "countdesc")

	_, _ = fmt.Fprintf(file, "* You fished the most in chat %s (%d fish caught) \n", sortedChats[0], Wrapped.Count.Chat[sortedChats[0]])

	_, _ = fmt.Fprintln(file)

	err = PrintTableMD(Wrapped.Count, []string{"-", "Chat", "Fish caught"}, "All your other chats", "totalchat", true, file)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(file)

	_, _ = fmt.Fprintf(file, "* The fish you caught the most was a %s\n", Wrapped.MostCaughtFish[0])

	_, _ = fmt.Fprint(file, "\n<details>")

	_, _ = fmt.Fprint(file, "\n<summary>Your other most caught fish</summary>\n\n")

	for _, mostCaughtFish := range Wrapped.MostCaughtFish {

		_, _ = fmt.Fprintf(file, "* %s\n", mostCaughtFish)
	}

	_, _ = fmt.Fprint(file, "\n</details>\n")

	_, _ = fmt.Fprintln(file)

	_, _ = fmt.Fprintf(file, "* You have caught %d different fish this year\n", len(Wrapped.FishSeen))

	_, _ = fmt.Fprintf(file, "* The rarest fish you caught was a %s\n", Wrapped.RarestFish[0])

	_, _ = fmt.Fprint(file, "\n<details>")

	_, _ = fmt.Fprint(file, "\n<summary>Your other rarest fish</summary>\n\n")

	for _, rareFish := range Wrapped.RarestFish {

		_, _ = fmt.Fprintf(file, "* %s\n", rareFish)
	}

	_, _ = fmt.Fprint(file, "\n</details>\n")

	_, _ = fmt.Fprintln(file)

	emojiForWeight := ReturnEmojiBiggestFish(Wrapped.BiggestFish[0].Weight)

	_, _ = fmt.Fprintf(file, "* Your biggest fish was a %.2f lbs %s %s\n",
		Wrapped.BiggestFish[0].Weight,
		Wrapped.BiggestFish[0].Fish,
		emojiForWeight)

	_, _ = fmt.Fprintln(file)

	err = PrintTableMD(Wrapped.BiggestFish, []string{"Fish", "Weight in lbs", "Catchtype", "Date", "Chat"}, "Your other biggest fish", "profilefishslice", true, file)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(file)

	return nil
}

func ReturnEmojiFishCaught(count int) string {

	var emoji string

	switch {
	case count >= 2000:
		emoji = "🤯"

	case count >= 1000:
		emoji = "😵‍💫"

	case count >= 100:
		emoji = "😲"

	case count <= 10:
		emoji = "😴"
	}

	return emoji
}

func ReturnEmojiBiggestFish(weight float64) string {

	var emoji string

	switch {
	case weight >= 300:
		emoji = "🎉"
	}

	return emoji
}
