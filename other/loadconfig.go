package other

import (
	"encoding/json"
	"fmt"
	"os"
)

type URLSet struct {
	Logs            string `json:"logs"`
	Trophies        string `json:"trophies"`
	Fishweek        string `json:"fishweek"`
	Fishweeklimit   string `json:"fishweek_limit"`
	Weight          string `json:"weight"`
	Weightlimit     string `json:"weight_limit"`
	Type            string `json:"type"`
	Totalcount      string `json:"totalcount"`
	Totalcountlimit string `json:"totalcount_limit"`
	TotalcountOld   string `json:"totalcountold"`
	LogsHost        string `json:"logs_host"`
	LogsHostOld     string `json:"logs_host_old"`
}

type Config struct {
	URLSets map[string]URLSet `json:"url_sets"`
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
