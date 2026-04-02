package api

import (
	"net/http"
	"time"

	"project/database"
	"project/middleware"

	"github.com/gin-gonic/gin"
)

func RequestPermit(ctx *gin.Context) {
	reAny, _ := ctx.Get(middleware.ContextRegulatedEntityKey)
	re, ok := reAny.(*database.RegulatedEntities)
	if !ok || re == nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only regulated entities can request permits"})
		return
	}

	type requestPermitBody struct {
		ActivityDescription   string        `json:"activity_description" binding:"required"`
		ActivityStartDate     time.Time     `json:"activity_start_date" binding:"required"`
		ActivityDuration      time.Duration `json:"activity_duration" binding:"required"`
		EnvironmentalPermitID uint          `json:"environmental_permit_id" binding:"required"`
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

func ReviewPermit(ctx *gin.Context) {
	eoAny, _ := ctx.Get(middleware.ContextEnvironmentalOfficerKey)
	re, ok := eoAny.(*database.RegulatedEntities)
	if !ok || re == nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only environmental officers can review permits"})
		return
	}

	type reviewPermitBody struct {
		PermitRequestID uint   `json:"permit_request_id" binding:"required"`
		Decision        string `json:"decision" binding:"required"`
		Description     string `json:"description" binding:"required"`
	}

	var payload reviewPermitBody
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid body format", "details": err.Error()})
		return
	}

	var permitRequest database.PermitRequest
	if err := database.DB.First(&permitRequest, payload.PermitRequestID).Error; err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid permit request reference"})
		return
	}

	permitRequestDecision := database.PermitRequestDecision{
		PermitRequestID: payload.PermitRequestID,
		Decision:        payload.Decision,
		Description:     payload.Description,
	}

	if err := database.DB.Create(&permitRequestDecision).Error; err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create permit request decision", "details": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message": "Permit request decision applied successfully",
		"id":      permitRequestDecision.ID,
	})
}
