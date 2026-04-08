package main

import (
	"fmt"
	"log"
	"net/http"
	"project/api"
	"project/database"
	"project/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	// Print a startup message to indicate the application is starting
	fmt.Println("Hello, World!")

	// Attempt to connect to the database using the configured connection parameters
	// If connection fails, print error and exit
	if err := database.ConnectDatabase(); err != nil {
		fmt.Println("Failed to connect to database:", err)
		return
	}

	// Seed the database with default entries such as default users and environmental permits
	// If seeding fails, print error and exit
	if err := database.SeedDefaultEntries(); err != nil {
		fmt.Println("Failed to seed database:", err)
		return
	}

	// Print success message after database connection and seeding
	fmt.Println("Connected to database successfully!")

	// Initialize a new Gin router instance for handling HTTP requests
	router := gin.Default()

	// Create a route group for API endpoints under "/api"
	api_group := router.Group("/api")
	{
		// Define a simple ping endpoint to test if the server is running
		// Returns a JSON response with "pong!" message
		api_group.GET("/ping", func(ctx *gin.Context) {
			ctx.JSON(http.StatusOK, gin.H{"message_text": "pong!"})
		})

		// Endpoint for registering a new regulated entity account
		api_group.POST("/register", api.RegisterRegulatedEntity)

		// Endpoint for user login, supporting regulated entity and environmental officer accounts
		api_group.POST("/login", api.Login)

		// Create a subgroup for protected endpoints that require authentication
		protected := api_group.Group("")
		protected.Use(middleware.AuthRequired())
		{
			// Endpoint to retrieve information about the currently authenticated user
			protected.GET("/whoami", api.WhoAmI)

			// Endpoint for regulated entities to request a new environmental permit
			protected.POST("/request-permit", api.RequestPermit)

			// Endpoint for regulated entities to submit payment for a permit request
			protected.POST("/permit-request/:request_id/submit_payment", api.SubmitPermitPayment)

			// Endpoint for environmental officers to list permit requests with submitted payments
			protected.GET("/eo/permit-requests/submitted-payment", api.ListPaymentSubmittedRequests)

			// Endpoint for environmental officers to start reviewing a permit request after payment submission
			protected.POST("/eo/permit-request/:request_id/start-review", api.ReviewPermitPaymentSubmitted)

			// Endpoint for environmental officers to make final decisions on permit requests (accept/reject)
			protected.POST("/review-permit", api.ReviewPermit)
		}
	}

	// Start the HTTP server on the configured port (default 8080)
	// The server listens on all interfaces (0.0.0.0) within the Docker network
	// If server fails to start, log fatal error and exit
	if err := router.Run(fmt.Sprintf("0.0.0.0:%s", database.GetEnv("HTTP_SERVER_PORT", "8080"))); err != nil {
		log.Fatalf("Failed to create HTTP server: %v", err)
	}
}
