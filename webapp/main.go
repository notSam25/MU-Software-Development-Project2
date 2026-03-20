package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
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

	// Note that during the 'compilation' of these structures into GORM, the struct name get's turned into `snake_case` and plural
	type Users struct {
		gorm.Model
		Username string `json:"username" gorm:"unique;not null"`
	}

	// Any structs added in here(comma separated) will be created/updated(if possible) in the database
	DB.AutoMigrate(&Users{})

	// Delete the sam user
	if err := DB.Where("username = sam").Delete(&Users{}).Error; err != nil {
		log.Printf("delete all users failed: %v", err)
	} else {
		log.Printf("deleted all users")
	}

	// Create a new user
	newUser := Users{Username: "sam"}
	if err := DB.Create(&newUser).Error; err != nil {
		log.Printf("create user failed: %v", err)
	} else {
		log.Printf("created user: id=%d username=%s", newUser.ID, newUser.Username)
	}

	// Try to create another user with the same name (should fail due to unique constraint)
	duplicateUser := Users{Username: "sam"}
	if err := DB.Create(&duplicateUser).Error; err != nil {
		log.Printf("expected duplicate-name error: %v", err)
	} else {
		log.Printf("unexpectedly created duplicate user: id=%d username=%s", duplicateUser.ID, duplicateUser.Username)
	}

	router := gin.Default()
	api_group := router.Group("/api")
	{
		// The HTTP router equivalent of Hello, World
		api_group.GET("/ping", func(ctx *gin.Context) {
			ctx.JSON(http.StatusOK, gin.H{"message_text": "pong!"})
		})
	}

	// Serve our endpoints on 0.0.0.0:8080. Note that these routes are under the same network as Docker.
	if err := router.Run(fmt.Sprintf("0.0.0.0:%s", GetEnv("HTTP_SERVER_PORT", "8080"))); err != nil {
		log.Fatalf("Failed to create HTTP server: %v", err)
	}
}
