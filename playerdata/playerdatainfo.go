package playerdata

import (
	"bufio"
	"gofish/logs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// for verified script
func ReadVerifiedPlayers() []string {
	verifiedPlayers := make([]string, 0)

	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)

	verifiedPath := filepath.Join(dir, "verified.txt")

	file, err := os.Open(verifiedPath)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error opening file")
		return verifiedPlayers
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		verifiedPlayers = append(verifiedPlayers, strings.TrimSpace(scanner.Text()))
	}

	if err := scanner.Err(); err != nil {
		logs.Logs().Error().Err(err).Msg("Error reading file")
		return verifiedPlayers
	}

	return verifiedPlayers
}
