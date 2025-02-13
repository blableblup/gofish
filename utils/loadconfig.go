package utils

import (
	"encoding/json"
	"gofish/logs"
	"os"
	"path/filepath"
)

type ChatInfo struct {
	Fishweeklimit      int        `json:"fishweek_limit"`
	Weightlimit        float64    `json:"weight_limit"`
	WeightlimitRecords float64    `json:"weight_limit_records"`
	Totalcountlimit    int        `json:"totalcount_limit"`
	Uniquelimit        int        `json:"unique_limit"`
	Rowlimit           int        `json:"row_limit"`
	LogsInstances      []Instance `json:"logs_instances"`
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
