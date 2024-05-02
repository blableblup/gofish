package data

import (
	"context"
	"encoding/json"
	"fmt"
	"gofish/logs"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v4/pgxpool"
)

// Configuration struct to hold database connection parameters
type Config struct {
	DB struct {
		User     string `json:"user"`
		Password string `json:"password"`
		Host     string `json:"host"`
		Port     string `json:"port"`
		DBName   string `json:"dbname"`
		SSLMode  string `json:"sslmode"`
	} `json:"db"`
}

// Function to read configuration from JSON file
func readConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// Function to establish a connection to the PostgreSQL database
func connectToDatabase(config *Config) (*pgxpool.Pool, error) {
	// Database connection configuration
	connConfig := fmt.Sprintf("user=%s password=%s host=%s port=%s dbname=%s sslmode=%s",
		config.DB.User, config.DB.Password, config.DB.Host, config.DB.Port, config.DB.DBName, config.DB.SSLMode)

	// Create a connection pool
	pool, err := pgxpool.Connect(context.Background(), connConfig)
	if err != nil {
		return nil, err
	}

	// Return the connection pool
	return pool, nil
}

func Connect() (*pgxpool.Pool, error) {
	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error getting current working directory")
		os.Exit(1)
	}

	// Construct the absolute path to the config file
	configFilePath := filepath.Join(wd, "sqlconfig.json")

	// Load the config from the constructed file path
	config, err := readConfig(configFilePath)
	if err != nil {
		logs.Logs().Error().Err(err).Msg("Error loading configuration")
	}

	// Establish connection to the PostgreSQL database
	return connectToDatabase(config)
}
