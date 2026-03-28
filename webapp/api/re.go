package api

import (
	"net/http"
	"project/database"

	"github.com/gin-gonic/gin"
)

type registerRegulatedEntityRequest struct {
	ContactPersonName   string `json:"contact_person_name" binding:"required"`
	Password            string `json:"password" binding:"required"`
	Email               string `json:"email" binding:"required,email"`
	OrganizationName    string `json:"organization_name" binding:"required"`
	OrganizationAddress string `json:"organization_address" binding:"required"`
}

func RegisterRegulatedEntity(ctx *gin.Context) {
	var payload registerRegulatedEntityRequest
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid body format", "details": err.Error()})
		return
	}

	entity := database.RegulatedEntities{
		ContactPersonName:   payload.ContactPersonName,
		Password:            payload.Password,
		Email:               payload.Email,
		OrganizationName:    payload.OrganizationName,
		OrganizationAddress: payload.OrganizationAddress,
	}

	if err := database.DB.Create(&entity).Error; err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "Failed to create account", "error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Account created successfully"})
}
