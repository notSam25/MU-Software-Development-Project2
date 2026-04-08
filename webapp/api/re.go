package api

import (
	"net/http"
	"project/database"

	"github.com/gin-gonic/gin"
)

// RegisterRegulatedEntity handles the registration of a new regulated entity account
// It validates the input data, creates a new RegulatedEntities record in the database,
// and returns a success response if the registration is successful
func RegisterRegulatedEntity(ctx *gin.Context) {
	// Define the expected structure of the request body for registration
	// All fields are required and validated using Gin's binding tags
	type body struct {
		ContactPersonName   string `json:"contact_person_name" binding:"required"`
		Password            string `json:"password" binding:"required"`
		Email               string `json:"email" binding:"required,email"`
		OrganizationName    string `json:"organization_name" binding:"required"`
		OrganizationAddress string `json:"organization_address" binding:"required"`
	}

	// Declare a variable to hold the parsed request body
	var payload body

	// Parse and validate the JSON request body
	// If validation fails, return a 400 Bad Request with error details
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid body format", "details": err.Error()})
		return
	}

	// Create a new RegulatedEntities struct with the provided data
	entity := database.RegulatedEntities{
		ContactPersonName:   payload.ContactPersonName,
		Password:            payload.Password,
		Email:               payload.Email,
		OrganizationName:    payload.OrganizationName,
		OrganizationAddress: payload.OrganizationAddress,
	}

	// Attempt to save the new entity to the database
	// If creation fails, return a 400 Bad Request with error details
	if err := database.DB.Create(&entity).Error; err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "Failed to create account", "error": err.Error()})
		return
	}

	// Return a 201 Created response indicating successful account creation
	ctx.JSON(http.StatusCreated, gin.H{"message": "Account created successfully"})
}
