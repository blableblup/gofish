package utils

import (
	"encoding/json"
	"gofish/logs"
	"os"
	"path/filepath"
)

type ChatInfo struct {
	TwitchID           string     `json:"twitchid,omitempty"`
	LogsInstances      []Instance `json:"logs_instances,omitempty"`
	PlayerCountLimit   int        `json:"playercount_limit,omitempty"`
	Fishweeklimit      int        `json:"fishweek_limit,omitempty"`
	Weightlimitmouth   float64    `json:"weightmouth_limit,omitempty"`
	Weightlimittotal   float64    `json:"weighttotal_limit,omitempty"`
	Weightlimit        float64    `json:"weight_limit,omitempty"`
	WeightlimitRecords float64    `json:"weight_limit_records,omitempty"`
	Totalcountlimit    int        `json:"totalcount_limit,omitempty"`
	Uniquelimit        int        `json:"unique_limit,omitempty"`
	Rowlimit           int        `json:"row_limit,omitempty"`
	CheckFData         bool       `json:"checkfdata"`
	BoardsEnabled      bool       `json:"board_enabled"`
}

type Config struct {
	Chat map[string]ChatInfo `json:"twitch_chat"`
}

type Instance struct {
	URL       string `json:"url"`
	LogsAdded string `json:"logs_added"`
}

func LoadConfig() Config {

	wd, err := os.Getwd()
	if err != nil {
		logs.Logs().Fatal().Err(err).
			Msg("Error getting current working directory")
	}

	configFilePath := filepath.Join(wd, "config.json")

	file, err := os.Open(configFilePath)
	if err != nil {
		logs.Logs().Fatal().Err(err).
			Msg("Error opening config file")
	}
	defer file.Close()

	var config Config
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		logs.Logs().Fatal().Err(err).
			Msg("Error parsing config file")
	}

	return config
}
