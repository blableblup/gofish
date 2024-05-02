package utils

import (
	"encoding/json"
	"gofish/logs"
	"os"
)

type ChatInfo struct {
	Fishweeklimit   int     `json:"fishweek_limit"`
	Weightlimit     float64 `json:"weight_limit"`
	Totalcountlimit int     `json:"totalcount_limit"`
	LogsHost        string  `json:"logs_host"`
	LogsHostOld     string  `json:"logs_host_old"`
	LogsAdded       string  `json:"logs_added"`
	Emoji           string  `json:"emoji"`
	CheckEnabled    bool    `json:"check_enabled"`
}

type Config struct {
	Chat map[string]ChatInfo `json:"twitch_chat"`
}

func LoadConfig(filename string) Config {
	file, err := os.Open(filename)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error opening config file")
		os.Exit(1)
	}
	defer file.Close()

	var config Config
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error parsing config file")
		os.Exit(1)
	}

	return config
}
