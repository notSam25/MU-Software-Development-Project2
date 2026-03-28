package api

import (
	"net/http"
	"time"

	"project/database"
	"project/middleware"

	"github.com/gin-gonic/gin"
)

type requestPermitBody struct {
	RequestNumber         string        `json:"request_number" binding:"required"`
	ActivityDescription   string        `json:"activity_description" binding:"required"`
	ActivityStartDate     time.Time     `json:"activity_start_date" binding:"required"`
	ActivityDuration      time.Duration `json:"activity_duration" binding:"required"`
	EnvironmentalPermitID uint          `json:"environmental_permit_id" binding:"required"`
}

func RequestPermit(ctx *gin.Context) {
	reAny, _ := ctx.Get(middleware.ContextRegulatedEntityKey)
	re, ok := reAny.(*database.RegulatedEntities)
	if !ok || re == nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only regulated entities can request permits"})
		return
	}

	var payload requestPermitBody
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid body format", "details": err.Error()})
		return
	}

	var envPermit database.EnvironmentalPermits
	if err := database.DB.First(&envPermit, payload.EnvironmentalPermitID).Error; err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environmental permit reference"})
		return
	}

	permitRequest := database.PermitRequest{
		RegulatedEntityID:     re.ID,
		EnvironmentalPermitID: envPermit.ID,
		RequestNumber:         payload.RequestNumber,
		ActivityDescription:   payload.ActivityDescription,
		ActivityStartDate:     payload.ActivityStartDate,
		ActivityDuration:      payload.ActivityDuration,
		PermitFee:             envPermit.PermitFee,
	}

	if err := database.DB.Create(&permitRequest).Error; err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create permit request", "details": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message": "Permit request created successfully",
		"id":      permitRequest.ID,
	})
}
