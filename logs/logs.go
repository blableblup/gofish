package logs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

var logger *zerolog.Logger

func InitializeLogger(debug bool, multi bool, pathlog string) {

	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Only log to console by default
	consoleLogger := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006/01/02 15:04:05"}
	var output io.Writer = consoleLogger

	if multi {
		// Can also log to a json file to save the logs
		// File will be in logs/logs_files, logs_files is being gitignored
		// By default the file will have the current time as name, doesnt need to be in utc i think
		var filePath string
		if pathlog == "" {
			filePath = filepath.Join("logs", "logs_files", fmt.Sprintf("log_%s.json", time.Now().Format("2006_01_02_15_04_05")))
		} else {
			if !strings.HasSuffix(pathlog, ".json") {
				pathlog += ".json"
			}
			filePath = filepath.Join("logs", "logs_files", pathlog)
		}

		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			panic(err)
		}

		// putting defer file.Close() here makes it so cant log to file
		// idk if this is a problem ?
		file, err := os.Create(filePath)
		if err != nil {
			panic(err)
		}

		// Log to the file and to the console
		output = zerolog.MultiLevelWriter(consoleLogger, file)
	}

	// Set the logger
	log := zerolog.New(output).With().Timestamp().Logger()

	logger = &log
}

func Logs() *zerolog.Logger {

	return logger
}
