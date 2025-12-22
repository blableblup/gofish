package leaderboards

import (
	"fmt"
	"gofish/logs"
	"os"
	"path/filepath"
)

func PrintWrapped(Wrapped *Wrapped, fishWithEmoji map[string]string) error {

	filePath := filepath.Join("leaderboards", "global", "wrappeds", Wrapped.Year, fmt.Sprintf("%d", Wrapped.TwitchID)+".json")

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	err = writeRaw(filePath, Wrapped)
	if err != nil {
		logs.Logs().Info().Str("FilePath", filePath).Msg("Error writing wrapped json")
		return err
	}

	return nil
}
