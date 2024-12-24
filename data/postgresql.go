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

type DBInfo struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	DBName   string `json:"dbname"`
	SSLMode  string `json:"sslmode"`
}

type DBConfig struct {
	DB map[string]DBInfo `json:"database"`
}

func LoadDBConfig() DBConfig {

	wd, err := os.Getwd()
	if err != nil {
		logs.Logs().Fatal().Err(err).
			Msg("Error getting current working directory")
	}

	configFilePath := filepath.Join(wd, "sqlconfig.json")

	file, err := os.Open(configFilePath)
	if err != nil {
		logs.Logs().Fatal().Err(err).
			Msg("Error opening DB config file")
	}
	defer file.Close()

	var config DBConfig
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		logs.Logs().Fatal().Err(err).
			Msg("Error parsing DB config file")
	}

	return config
}

func connectToDatabase(config *DBConfig, database string) (*pgxpool.Pool, error) {
	_, ok := config.DB[database]
	if !ok {
		logs.Logs().Fatal().
			Str("Database", database).
			Msg("Database not found in sqlconfig")
	}

	connConfig := fmt.Sprintf("user=%s password=%s host=%s port=%s dbname=%s sslmode=%s",
		config.DB[database].User, config.DB[database].Password, config.DB[database].Host, config.DB[database].Port, config.DB[database].DBName, config.DB[database].SSLMode)

	pool, err := pgxpool.New(context.Background(), connConfig)
	if err != nil {
		return nil, err
	}

	// Return the connection pool
	return pool, nil
}

func Connect(database string) (*pgxpool.Pool, error) {
	config := LoadDBConfig()

	return connectToDatabase(&config, database)
}
