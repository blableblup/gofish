package data

import (
	"context"
	"encoding/json"
	"fmt"
	"gofish/logs"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DBConfig struct {
	DB struct {
		User     string `json:"user"`
		Password string `json:"password"`
		Host     string `json:"host"`
		Port     string `json:"port"`
		DBName   string `json:"dbname"`
		SSLMode  string `json:"sslmode"`
	} `json:"db"`
}

func LoadDBConfig() DBConfig {

	wd, err := os.Getwd()
	if err != nil {
		logs.Logs().Fatal().Err(err).Msg("Error getting current working directory")
	}

	configFilePath := filepath.Join(wd, "sqlconfig.json")
	logs.Logs().Debug().Str("configFilePath", configFilePath).Msg("Loading DB config file")

	file, err := os.Open(configFilePath)
	if err != nil {
		logs.Logs().Fatal().Err(err).Msg("Error opening DB config file")
	}
	defer file.Close()

	var config DBConfig
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		logs.Logs().Fatal().Err(err).Msg("Error parsing DB config file")
	}

	logs.Logs().Debug().Interface("DB config", config).Msg("Loaded DB connection config")

	return config
}

func connectToDatabase(config *DBConfig) (*pgxpool.Pool, error) {
	connConfig := fmt.Sprintf("user=%s password=%s host=%s port=%s dbname=%s sslmode=%s",
		config.DB.User, config.DB.Password, config.DB.Host, config.DB.Port, config.DB.DBName, config.DB.SSLMode)

	pool, err := pgxpool.New(context.Background(), connConfig)
	if err != nil {
		return nil, err
	}

	// Return the connection pool
	return pool, nil
}

func Connect() (*pgxpool.Pool, error) {
	config := LoadDBConfig()

	// Establish connection to the PostgreSQL database
	return connectToDatabase(&config)
}
