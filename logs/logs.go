package logs

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var logger *zerolog.Logger

func InitializeLogger() {

	l := log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006/01/02 15:04:05"})
	logger = &l

}

func Logs() *zerolog.Logger {

	if logger == nil {
		InitializeLogger()
	}
	return logger
}
