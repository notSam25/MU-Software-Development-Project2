package api

import (
	"net/http"
	"strconv"
	"time"

	"project/database"
	"project/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func latestPermitRequestStatus(tx *gorm.DB, permitRequestID uint) (database.PermitRequestStatus, error) {
	var status database.PermitRequestStatus
	if err := tx.Where("permit_request_id = ?", permitRequestID).Order("id desc").First(&status).Error; err != nil {
		return database.PermitRequestStatus{}, err
	}
	return status, nil
}

func appendPermitRequestStatus(tx *gorm.DB, permitRequestID uint, status string, description string) error {
	return tx.Create(&database.PermitRequestStatus{
		PermitRequestID: permitRequestID,
		Status:          status,
		Description:     description,
	}).Error
}

func currentPermitRequestsWithStatus(status string) ([]database.PermitRequest, error) {
	var requests []database.PermitRequest
	if err := database.DB.Preload("Statuses").Preload("Decision").Preload("Payment").Preload("Permit").Find(&requests).Error; err != nil {
		return nil, err
	}

	filtered := make([]database.PermitRequest, 0)
	for _, req := range requests {
		if len(req.Statuses) == 0 {
			continue
		}
		latest := req.Statuses[0]
		for _, s := range req.Statuses[1:] {
			if s.ID > latest.ID {
				latest = s
			}
		}
		if latest.Status == status {
			filtered = append(filtered, req)
		}
	}

	return filtered, nil
}

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

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&permitRequest).Error; err != nil {
			return err
		}
		return appendPermitRequestStatus(tx, permitRequest.ID, database.PermitRequestStatusPendingPayment, "Pending payment")
	}); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create permit request workflow", "details": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message":    "Permit request created successfully",
		"id":         permitRequest.ID,
		"permit_fee": permitRequest.PermitFee,
	})
}

func SubmitPermitPayment(ctx *gin.Context) {
	reAny, _ := ctx.Get(middleware.ContextRegulatedEntityKey)
	re, ok := reAny.(*database.RegulatedEntities)
	if !ok || re == nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only regulated entities can submit permit payments"})
		return
	}

	requestIDRaw := ctx.Param("request_id")
	requestID, err := strconv.ParseUint(requestIDRaw, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid permit request id"})
		return
	}

	type submitPaymentBody struct {
		PaymentMethod        string `json:"payment_method" binding:"required"`
		LastFourDigitsOfCard string `json:"last_four_digits_of_card" binding:"required,len=4"`
		CardHolderName       string `json:"card_holder_name" binding:"required"`
	}

	var payload submitPaymentBody
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid body format", "details": err.Error()})
		return
	}

	var permitRequest database.PermitRequest
	if err := database.DB.Preload("Payment").Preload("Statuses").First(&permitRequest, uint(requestID)).Error; err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid permit request reference"})
		return
	}

	if permitRequest.RegulatedEntityID != re.ID {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Cannot submit payment for another regulated entity"})
		return
	}

	latestStatus, err := latestPermitRequestStatus(database.DB, permitRequest.ID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Permit request has no workflow status"})
		return
	}
	if latestStatus.Status != database.PermitRequestStatusPendingPayment {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Payment can only be submitted when request is Pending Payment"})
		return
	}

	if permitRequest.Payment != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Payment already exists for this permit request"})
		return
	}

	payment := database.Payment{
		PermitRequestID:      permitRequest.ID,
		PaymentMethod:        payload.PaymentMethod,
		LastFourDigitsOfCard: payload.LastFourDigitsOfCard,
		CardHolderName:       payload.CardHolderName,
		PaymentApproved:      false,
	}

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&payment).Error; err != nil {
			return err
		}
		return appendPermitRequestStatus(tx, permitRequest.ID, database.PermitRequestStatusReviewingPayment, "Reviewing payment")
	}); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Failed to submit payment", "details": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message":           "Payment submitted successfully",
		"permit_request_id": permitRequest.ID,
		"status":            database.PermitRequestStatusReviewingPayment,
	})
}

func ReviewPermitPayment(ctx *gin.Context) {
	opsAny, _ := ctx.Get(middleware.ContextOPSKey)
	ops, ok := opsAny.(*database.OPS)
	if !ok || ops == nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only OPS can review payments"})
		return
	}
	_ = ops

	requestIDRaw := ctx.Param("request_id")
	requestID, err := strconv.ParseUint(requestIDRaw, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid permit request id"})
		return
	}

	type reviewPaymentBody struct {
		Decision    string `json:"decision" binding:"required,oneof=Submitted Rejected"`
		Description string `json:"description" binding:"required"`
	}

	var payload reviewPaymentBody
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid body format", "details": err.Error()})
		return
	}

	var permitRequest database.PermitRequest
	if err := database.DB.Preload("Statuses").Preload("Payment").First(&permitRequest, uint(requestID)).Error; err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid permit request reference"})
		return
	}

	latestStatus, err := latestPermitRequestStatus(database.DB, permitRequest.ID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Permit request has no workflow status"})
		return
	}
	if latestStatus.Status != database.PermitRequestStatusReviewingPayment {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Payment can only be reviewed when request is Reviewing Payment"})
		return
	}

	if payload.Decision == database.PermitRequestStatusSubmitted && permitRequest.Payment == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Cannot mark payment as Submitted without a payment record"})
		return
	}

	newStatus := payload.Decision
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := appendPermitRequestStatus(tx, permitRequest.ID, newStatus, payload.Description); err != nil {
			return err
		}
		if newStatus == database.PermitRequestStatusSubmitted && permitRequest.Payment != nil {
			return tx.Model(&database.Payment{}).Where("permit_request_id = ?", permitRequest.ID).Update("payment_approved", true).Error
		}
		return nil
	}); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Failed to update payment review", "details": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message":           "Payment review status applied successfully",
		"permit_request_id": permitRequest.ID,
		"status":            newStatus,
	})
}

func ReviewPermitPaymentSubmitted(ctx *gin.Context) {
	eoAny, _ := ctx.Get(middleware.ContextEnvironmentalOfficerKey)
	eo, ok := eoAny.(*database.EnvironmentalOfficer)
	if !ok || eo == nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only environmental officers can advance submitted payment requests"})
		return
	}
	_ = eo

	requestIDRaw := ctx.Param("request_id")
	requestID, err := strconv.ParseUint(requestIDRaw, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid permit request id"})
		return
	}

	var permitRequest database.PermitRequest
	if err := database.DB.Preload("Statuses").First(&permitRequest, uint(requestID)).Error; err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid permit request reference"})
		return
	}

	latestStatus, err := latestPermitRequestStatus(database.DB, permitRequest.ID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Permit request has no workflow status"})
		return
	}
	if latestStatus.Status != database.PermitRequestStatusSubmitted {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Permit request must be Submitted before EO review"})
		return
	}

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		return appendPermitRequestStatus(tx, permitRequest.ID, database.PermitRequestStatusBeingReviewed, "Being reviewed by EO")
	}); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Failed to advance permit request", "details": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message":           "Permit request is now being reviewed",
		"permit_request_id": permitRequest.ID,
		"status":            database.PermitRequestStatusBeingReviewed,
	})
}

func ReviewPermit(ctx *gin.Context) {
	eoAny, _ := ctx.Get(middleware.ContextEnvironmentalOfficerKey)
	eo, ok := eoAny.(*database.EnvironmentalOfficer)
	if !ok || eo == nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only environmental officers can review permits"})
		return
	}
	_ = eo

	type reviewPermitBody struct {
		PermitRequestID uint   `json:"permit_request_id" binding:"required"`
		Decision        string `json:"decision" binding:"required,oneof=Accepted Rejected"`
		Description     string `json:"description" binding:"required"`
	}

	var payload reviewPermitBody
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid body format", "details": err.Error()})
		return
	}

	var permitRequest database.PermitRequest
	if err := database.DB.Preload("Statuses").Preload("Decision").Preload("Permit").First(&permitRequest, payload.PermitRequestID).Error; err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid permit request reference"})
		return
	}

	if permitRequest.Decision != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Final permit request decision already exists"})
		return
	}

	latestStatus, err := latestPermitRequestStatus(database.DB, permitRequest.ID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Permit request has no workflow status"})
		return
	}
	if latestStatus.Status != database.PermitRequestStatusBeingReviewed {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "EO final decision can only be applied when request is Being Reviewed"})
		return
	}

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		decision := database.PermitRequestDecision{
			PermitRequestID: payload.PermitRequestID,
			Decision:        payload.Decision,
			Description:     payload.Description,
		}
		if err := tx.Create(&decision).Error; err != nil {
			return err
		}

		if err := appendPermitRequestStatus(tx, permitRequest.ID, payload.Decision, payload.Description); err != nil {
			return err
		}

		if payload.Decision == database.PermitRequestStatusAccepted {
			if err := tx.Create(&database.Permit{PermitRequestID: permitRequest.ID}).Error; err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create final permit decision", "details": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message":           "Permit request decision applied successfully",
		"permit_request_id": permitRequest.ID,
		"decision":          payload.Decision,
	})
}

func ListReviewingPaymentRequests(ctx *gin.Context) {
	opsAny, _ := ctx.Get(middleware.ContextOPSKey)
	if ops, ok := opsAny.(*database.OPS); !ok || ops == nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only OPS can list reviewing payment requests"})
		return
	} else {
		_ = ops
	}

	requests, err := currentPermitRequestsWithStatus(database.PermitRequestStatusReviewingPayment)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list requests", "details": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"items": requests})
}

func ListPaymentSubmittedRequests(ctx *gin.Context) {
	eoAny, _ := ctx.Get(middleware.ContextEnvironmentalOfficerKey)
	if eo, ok := eoAny.(*database.EnvironmentalOfficer); !ok || eo == nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only environmental officers can list submitted payment requests"})
		return
	} else {
		_ = eo
	}

	requests, err := currentPermitRequestsWithStatus(database.PermitRequestStatusSubmitted)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list requests", "details": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"items": requests})
}
