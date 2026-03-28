package database

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

func resolveTables() error {
	if err := DB.AutoMigrate(&RegulatedEntities{}, &RegulatedEntitySite{}, &EnvironmentalOfficer{}, &OPS{}); err != nil {
		return err
	}

	if err := DB.AutoMigrate(&PermitRequest{}); err != nil {
		return err
	}

	if err := DB.AutoMigrate(&EnvironmentalPermits{}, &PermitRequestDecision{}, &Payment{}, &Permit{}); err != nil {
		return err
	}

	return nil
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
	if err := resolveTables(); err != nil {
		return err
	}
	return nil
}
