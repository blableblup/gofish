package data

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
)

// Function to check if a record already exists in the file
func recordExists(file *os.File, recordStr string) bool {
	if _, err := file.Seek(0, 0); err != nil {
		fmt.Println("Error seeking file:", err)
		return false
	}
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable number of fields
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}
		// Skip header line
		if record[0] == "player" || record[0] == "type" {
			continue
		}
		// Check if record string matches existing record
		if recordStr == concatRecord(record) {
			return true
		}
	}
	return false
}

// Function to concatenate record fields into a single string
func concatRecord(record []string) string {
	recordStr := ""
	for _, field := range record {
		recordStr += field
	}
	return recordStr
}

// Function to write weight log
func WriteWeightLog(chatName string, record string, newWeightRecord map[string]Record) {
	// Construct the absolute path to the weight log file
	logFilePath := filepath.Join("data", chatName, "weightlogs.csv")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
		fmt.Println("Error creating directory:", err)
		return
	}

	// Create or open the CSV file
	file, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer file.Close()

	// Create a CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// If the file is empty, write the header
	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Println("Error getting file info:", err)
		return
	}
	if fileInfo.Size() == 0 {
		header := []string{"player", "weight", "type", "catchtype", "bot", "date", "record"}
		if err := writer.Write(header); err != nil {
			fmt.Println("Error writing header to CSV:", err)
			return
		}
	}

	// Update the record and write to log if it doesn't exist
	for player, newWeightRecord := range newWeightRecord {
		// Create record string to check for duplicates
		recordStr := player + fmt.Sprintf("%.2f", newWeightRecord.Weight) + newWeightRecord.Type + newWeightRecord.CatchType + newWeightRecord.Bot + newWeightRecord.Date + record
		// Check if record already exists
		if recordExists(file, recordStr) {
			continue // Skip writing if record already exists
		}
		// Write the new record to log
		if err := writer.Write([]string{player, fmt.Sprintf("%.2f", newWeightRecord.Weight), newWeightRecord.Type, newWeightRecord.CatchType, newWeightRecord.Bot, newWeightRecord.Date, record}); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to weightlog of %s:\n%s\n", chatName, err)
		}
	}
}

// Function to write type log
func WriteTypeLog(chatName string, record string, newTypeRecord map[string]Record) {
	// Construct the absolute path to the type log file
	logFilePath := filepath.Join("data", chatName, "typelogs.csv")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
		fmt.Println("Error creating directory:", err)
		return
	}

	// Create or open the CSV file
	file, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer file.Close()

	// Create a CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// If the file is empty, write the header
	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Println("Error getting file info:", err)
		return
	}
	if fileInfo.Size() == 0 {
		header := []string{"type", "weight", "player", "catchtype", "bot", "date", "record"}
		if err := writer.Write(header); err != nil {
			fmt.Println("Error writing header to CSV:", err)
			return
		}
	}

	// Update the record and write to log if it doesn't exist
	for fishType, newTypeRecord := range newTypeRecord {
		// Create record string to check for duplicates
		recordStr := fishType + fmt.Sprintf("%.2f", newTypeRecord.Weight) + newTypeRecord.Player + newTypeRecord.CatchType + newTypeRecord.Bot + newTypeRecord.Date + record
		// Check if record already exists
		if recordExists(file, recordStr) {
			continue // Skip writing if record already exists
		}
		// Write the new record to log
		if err := writer.Write([]string{fishType, fmt.Sprintf("%.2f", newTypeRecord.Weight), newTypeRecord.Player, newTypeRecord.CatchType, newTypeRecord.Bot, newTypeRecord.Date, record}); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to typelog of %s:\n%s\n", chatName, err)
		}
	}
}
