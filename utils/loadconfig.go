package utils

import (
	"encoding/json"
	"fmt"
	"os"
)

type ChatInfo struct {
	Logs            string  `json:"logs"`
	Trophies        string  `json:"trophies"`
	Fishweek        string  `json:"fishweek"`
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
		fmt.Println("Error opening config file:", err)
		os.Exit(1)
	}
	defer file.Close()

	var config Config
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		fmt.Println("Error parsing config file:", err)
		os.Exit(1)
	}

	return config
}
