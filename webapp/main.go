package main

import (
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	DB *gorm.DB
)

func GetEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func ConnectDatabase() error {

	// Retrieve environment variables with defaults
	user := GetEnv("POSTGRES_USER", "user")
	password := GetEnv("POSTGRES_PASSWORD", "password")
	dbName := GetEnv("POSTGRES_DB", "db")
	dbHost := GetEnv("POSTGRES_HOST", "postgres")
	dbPort := GetEnv("POSTGRES_PORT", "5432")

	// Build the connection string
	connectionString := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, user, password, dbName,
	)

	db, err := gorm.Open(postgres.Open(connectionString), &gorm.Config{})
	if err != nil {
		return err
	}

	DB = db
	return nil
}

func main() {
	fmt.Println("Hello, World!")

	if err := ConnectDatabase(); err != nil {
		fmt.Println("Failed to connect to database:", err)
		return
	}

	fmt.Println("Connected to database successfully!")
}
