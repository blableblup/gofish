package scripts

import (
	"fmt"
	"gofish/data"
	"regexp"
)

func RunPattern() {
	textContent := `
	[2023-09-01 01:36:20] #psp1g supibot: 👥 davidprospero_, You caught a 🫧 🐟 🫧! It weighs 1.41 lbs. A new record! 🎉 (30m cooldown after a catch)
	[2023-02-25 03:21:11] #breadworms supibot: 👥 islcfc, Bye bye 🪸! 🫳🌊 ...Huh? ✨ Something is sparkling in the ocean... 🥍 🍬 Got it!
	[2024-03-28 01:33:53] #breadworms gofishgame: @xxx_r0ze_xxx, Huh?! ✨ Something jumped out of the water to snatch your rare candy! ...Got it! 🥍 🐳 298.1 lbs!
	[2024-03-27 20:25:00] #breadworms gofishgame: @sicklymaidrobot, You caught a ✨ 🦈 ✨! It weighs 162.95 lbs. And!... 🐍 (10.1 lbs) was in its mouth! 🎏 broke!💢 🪝 broke!💢 (30m cooldown after a catch)
	[2024-04-14 07:39:20] #breadworms gofishgame: @fishingalt, Huh?! 🪺 is hatching!... It's a 🪽 🐦‍⬛ 🪽! It weighs 2.96 lbs.
	[2024-04-14 11:18:24] #breadworms gofishgame: @dayzedinndaydreams, 📣 The results are in! You caught 🪣 27 fish: 10th place. Together they weighed 📟 693.4 lbs: 5th place. Your biggest catch weighed 🎣 113.75 lbs: 9th place.
	[2024-04-15 03:11:35] #breadworms gofishgame: @julialuxel, Last week... You caught 🪣 2 fish: 17th place. Together they weighed 📟 69.13 lbs: 17th place. Your biggest catch weighed 🎣 67.59 lbs: 13th place.
	[2023-07-15 22:51:54] #breadworms supibot: 👥 puzzlow, 📣 The results are in! You caught 🪣 29 fish: 7th place. Together they weighed ⚖ 627.56 lbs: 6th place. Your biggest catch weighed 🎣 215.5 lbs: 5th place.
	[2023-10-8 11:20:26] #breadworms gofishgame: @qu4ttromila, 📣 The results are in! You caught 🪣 24 fish: That's third 🥉! Together they weighed ⚖️ 436.47 lbs: 5th place. Your biggest catch weighed 🎣 99.72 lbs: 5th place.
	[2023-05-14 13:53:47] #breadworms supibot: 👥 kishma9, The results for last week are in 🎣! You caught 90 fish making you the runner-up 🥈!. Together they weighed 1791.61 lbs, making you the runner-up 🥈!. Your biggest catch weighed 247.9 lbs, making it the runner-up 🥈!.

	`

	testAllPatterns(textContent)
}

func testAllPatterns(textContent string) {
	fmt.Println("Testing all patterns against the text content:")
	fmt.Println()

	testPattern(textContent, "MouthPattern", data.MouthPattern)
	testPattern(textContent, "ReleasePattern", data.ReleasePattern)
	testPattern(textContent, "JumpedPattern", data.JumpedPattern)
	testPattern(textContent, "NormalPattern", data.NormalPattern)
	testPattern(textContent, "BirdPattern", data.BirdPattern)
	testPattern(textContent, "TrnmPattern", data.TrnmPattern)
	testPattern(textContent, "Trnm2Pattern", data.Trnm2Pattern)

}

func testPattern(textContent string, patternName string, pattern *regexp.Regexp) {
	fmt.Println("Testing pattern:", patternName)

	// Find all matches in the text content
	matches := pattern.FindAllStringSubmatch(textContent, -1)

	// Print matches
	for _, match := range matches {
		fmt.Println("Match:", match)
	}
	fmt.Println()
}
