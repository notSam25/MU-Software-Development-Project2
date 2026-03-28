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

	if err := DB.AutoMigrate(&EnvironmentalPermits{}); err != nil {
		return err
	}

	if err := DB.AutoMigrate(&PermitRequest{}, &PermitRequestDecision{}, &Payment{}, &Permit{}); err != nil {
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

func SeedDefaultEntries() error {
	// Create default EnvironmentalOfficer if it doesn't exist
	defaultEO := &EnvironmentalOfficer{
		Name:     "Default Officer",
		Email:    "officer@example.com",
		Password: "default-password-123",
	}

	if err := DB.FirstOrCreate(
		defaultEO,
		EnvironmentalOfficer{Email: defaultEO.Email},
	).Error; err != nil {
		return fmt.Errorf("failed to create default environmental officer: %w", err)
	}

	// Create default EnvironmentalPermits if they don't exist
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
