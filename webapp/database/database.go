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

package database

import (
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	// DB is the global database connection instance used throughout the application
	DB *gorm.DB
)

// GetEnv retrieves an environment variable value or returns a default if not set
// This is a utility function for configuration management
func GetEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// resolveTables performs auto-migration for all database models
// This creates or updates database tables to match the Go struct definitions
func resolveTables() error {
	// Migrate user account tables
	if err := DB.AutoMigrate(&RegulatedEntities{}, &RegulatedEntitySite{}, &EnvironmentalOfficer{}, &OPS{}); err != nil {
		return err
	}

	// Migrate permit-related tables
	if err := DB.AutoMigrate(&EnvironmentalPermits{}); err != nil {
		return err
	}

	// Migrate permit request workflow tables
	if err := DB.AutoMigrate(&PermitRequest{}, &PermitRequestStatus{}, &PermitRequestDecision{}, &Payment{}, &Permit{}); err != nil {
		return err
	}

	return nil
}

// ConnectDatabase establishes a connection to the PostgreSQL database
// It reads connection parameters from environment variables and performs auto-migration
func ConnectDatabase() error {
	// Retrieve database connection parameters from environment variables with defaults
	user := GetEnv("POSTGRES_USER", "user")
	password := GetEnv("POSTGRES_PASSWORD", "password")
	dbName := GetEnv("POSTGRES_DB", "db")
	dbHost := GetEnv("POSTGRES_HOST", "postgres")
	dbPort := GetEnv("POSTGRES_PORT", "5432")

	// Build the PostgreSQL connection string
	connectionString := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, user, password, dbName,
	)

	// Open database connection using GORM
	db, err := gorm.Open(postgres.Open(connectionString), &gorm.Config{})
	if err != nil {
		return err
	}

	// Set the global database instance
	DB = db

	// Perform auto-migration to ensure tables exist
	if err := resolveTables(); err != nil {
		return err
	}
	return nil
}

// SeedDefaultEntries populates the database with default data required for the application to function
// This includes default user accounts and environmental permit templates
func SeedDefaultEntries() error {
	// Create default Environmental Officer account if it doesn't exist
	defaultEO := &EnvironmentalOfficer{
		Name:     "Default Officer",
		Email:    "officer@example.com",
		Password: "default-password-123",
	}

	// Use FirstOrCreate to avoid duplicates
	if err := DB.FirstOrCreate(
		defaultEO,
		EnvironmentalOfficer{Email: defaultEO.Email},
	).Error; err != nil {
		return fmt.Errorf("failed to create default environmental officer: %w", err)
	}

	// Create default OPS account if it doesn't exist
	defaultOPS := &OPS{
		Name:     "Default OPS",
		Email:    "ops@example.com",
		Password: "default-password-123",
	}

	if err := DB.FirstOrCreate(
		defaultOPS,
		OPS{Email: defaultOPS.Email},
	).Error; err != nil {
		return fmt.Errorf("failed to create default OPS account: %w", err)
	}

	// Define default environmental permit templates
	permits := []EnvironmentalPermits{
		{
			PermitName:  "Land Development Permit",
			PermitFee:   500.00,
			Description: "Permit for land development and construction activities",
		},
		{
			PermitName:  "Air Quality Permit",
			PermitFee:   300.00,
			Description: "Permit for air emissions and air quality compliance",
		},
		{
			PermitName:  "Water Discharge Permit",
			PermitFee:   400.00,
			Description: "Permit for water discharge and wastewater management",
		},
		{
			PermitName:  "Hazardous Waste Permit",
			PermitFee:   750.00,
			Description: "Permit for handling and disposal of hazardous waste",
		},
		{
			PermitName:  "Mining Operations Permit",
			PermitFee:   1000.00,
			Description: "Permit for mining and extraction operations",
		},
	}

	// Create each permit template if it doesn't already exist
	for _, permit := range permits {
		if err := DB.FirstOrCreate(
			&permit,
			EnvironmentalPermits{PermitName: permit.PermitName},
		).Error; err != nil {
			return fmt.Errorf("failed to create default permit %s: %w", permit.PermitName, err)
		}
	}

	return nil
}
