package scripts

import (
	"fmt"
	"gofish/data"
	"regexp"
)

func RunPattern() {
	// Sample text content to test the patterns
	textContent := `
	[2023-09-01 01:36:20] #psp1g supibot: 👥 davidprospero_, You caught a 🫧 🐟 🫧! It weighs 1.41 lbs. A new record! 🎉 (30m cooldown after a catch)
	[2023-02-25 03:21:11] #breadworms supibot: 👥 islcfc, Bye bye 🪸! 🫳🌊 ...Huh? ✨ Something is sparkling in the ocean... 🥍 🍬 Got it!
	[2024-03-28 01:33:53] #breadworms gofishgame: @xxx_r0ze_xxx, Huh?! ✨ Something jumped out of the water to snatch your rare candy! ...Got it! 🥍 🐳 298.1 lbs!
	[2024-03-27 20:25:00] #breadworms gofishgame: @sicklymaidrobot, You caught a ✨ 🦈 ✨! It weighs 162.95 lbs. And!... 🐍 (10.1 lbs) was in its mouth! 🎏 broke!💢 🪝 broke!💢 (30m cooldown after a catch)
	[2024-04-14 07:39:20] #breadworms gofishgame: @fishingalt, Huh?! 🪺 is hatching!... It's a 🪽 🐦‍⬛ 🪽! It weighs 2.96 lbs.
	`

	testAllPatterns(textContent)
}

// Function to test all patterns against the given text content
func testAllPatterns(textContent string) {
	fmt.Println("Testing all patterns against the text content:")
	fmt.Println()

	// Test MouthPattern
	testPattern(textContent, "MouthPattern", data.MouthPattern)

	// Test ReleasePattern
	testPattern(textContent, "ReleasePattern", data.ReleasePattern)

	// Test JumpedPattern
	testPattern(textContent, "JumpedPattern", data.JumpedPattern)

	// Test NormalPattern
	testPattern(textContent, "NormalPattern", data.NormalPattern)

	// Test BirdPattern
	testPattern(textContent, "BirdPattern", data.BirdPattern)
}

// Function to test a pattern against the given text content
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
