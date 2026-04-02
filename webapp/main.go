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
	fmt.Println("Hello, World!")

	if err := database.ConnectDatabase(); err != nil {
		fmt.Println("Failed to connect to database:", err)
		return
	}
	if err := database.SeedDefaultEntries(); err != nil {
		fmt.Println("Failed to seed database:", err)
		return
	}
	fmt.Println("Connected to database successfully!")

	router := gin.Default()
	api_group := router.Group("/api")
	{
		// The HTTP router equivalent of Hello, World
		api_group.GET("/ping", func(ctx *gin.Context) {
			ctx.JSON(http.StatusOK, gin.H{"message_text": "pong!"})
		})

		api_group.POST("/register", api.RegisterRegulatedEntity)
		api_group.POST("/login", api.Login)

		protected := api_group.Group("")
		protected.Use(middleware.AuthRequired())
		{
			protected.GET("/whoami", api.WhoAmI)
			protected.POST("/request-permit", api.RequestPermit)
			protected.POST("/review-permit", api.ReviewPermit)
		}
	}

	// Serve our endpoints on 0.0.0.0:8080. Note that these routes are under the same network as Docker.
	if err := router.Run(fmt.Sprintf("0.0.0.0:%s", database.GetEnv("HTTP_SERVER_PORT", "8080"))); err != nil {
		log.Fatalf("Failed to create HTTP server: %v", err)
	}
}
