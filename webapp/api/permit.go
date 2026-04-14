package api

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"project/database"
	"project/email"
	"project/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// latestPermitRequestStatus retrieves the most recent status for a given permit request ID
// It queries the database for the status with the highest ID (most recent) for the specified permit request
// Returns the status record or an error if not found
func latestPermitRequestStatus(tx *gorm.DB, permitRequestID uint) (database.PermitRequestStatus, error) {
	var status database.PermitRequestStatus
	if err := tx.Where("permit_request_id = ?", permitRequestID).Order("id desc").First(&status).Error; err != nil {
		return database.PermitRequestStatus{}, err
	}
	return status, nil
}

// appendPermitRequestStatus creates a new status entry for a permit request in the database
// This is used to track the workflow progression of permit requests
// Takes the permit request ID, status string, and description of the status change
func appendPermitRequestStatus(tx *gorm.DB, permitRequestID uint, status string, description string) error {
	return tx.Create(&database.PermitRequestStatus{
		PermitRequestID: permitRequestID,
		Status:          status,
		Description:     description,
	}).Error
}

// currentPermitRequestsWithStatus loads all permit requests from the database with their related data
// and filters them to return only those whose latest status matches the specified status
// This is used for listing permit requests in specific workflow states
func currentPermitRequestsWithStatus(status string) ([]database.PermitRequest, error) {
	var requests []database.PermitRequest
	if err := database.DB.Preload("RegulatedEntity").Preload("EnvironmentalPermit").Preload("Statuses", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("id asc")
	}).Preload("Decision").Preload("Payment").Preload("Permit").Find(&requests).Error; err != nil {
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

// allPermitRequests loads all permit requests with related entities for EO reporting/review screens.
func allPermitRequests() ([]database.PermitRequest, error) {
	var requests []database.PermitRequest
	if err := database.DB.
		Preload("Statuses", func(tx *gorm.DB) *gorm.DB { return tx.Order("id asc") }).
		Preload("Decision").
		Preload("Payment").
		Preload("Permit").
		Preload("RegulatedEntity").
		Preload("EnvironmentalPermit").
		Order("id desc").
		Find(&requests).Error; err != nil {
		return nil, err
	}

	return requests, nil
}

// currentPermitRequestsForRegulatedEntity loads every permit request owned by the given regulated entity.
// This powers the RE workflow tabs so they can refresh status, notes, and final decision history from the backend.
func currentPermitRequestsForRegulatedEntity(regulatedEntityID uint) ([]database.PermitRequest, error) {
	var requests []database.PermitRequest
	if err := database.DB.
		Preload("RegulatedEntity").
		Preload("EnvironmentalPermit").
		Preload("Statuses", func(tx *gorm.DB) *gorm.DB { return tx.Order("id asc") }).
		Preload("Decision").
		Preload("Payment").
		Preload("Permit").
		Where("regulated_entity_id = ?", regulatedEntityID).
		Order("id desc").
		Find(&requests).Error; err != nil {
		return nil, err
	}

	return requests, nil
}

// latestStatusFromStatuses returns the newest workflow status string from a status list.
func latestStatusFromStatuses(statuses []database.PermitRequestStatus) string {
	if len(statuses) == 0 {
		return ""
	}

	latest := statuses[0]
	for _, status := range statuses[1:] {
		if status.ID > latest.ID {
			latest = status
		}
	}

	return latest.Status
}

// statusHistoryString returns a human-readable status chain ordered by creation.
func statusHistoryString(statuses []database.PermitRequestStatus) string {
	if len(statuses) == 0 {
		return ""
	}

	sorted := append([]database.PermitRequestStatus(nil), statuses...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	parts := make([]string, 0, len(sorted))
	for _, status := range sorted {
		parts = append(parts, status.Status)
	}

	return strings.Join(parts, " -> ")
}

// RequestPermit allows a regulated entity to submit a new permit request
// It validates the user's identity, checks the environmental permit exists,
// creates a permit request record, and sets the initial status to "Pending Payment"
func RequestPermit(ctx *gin.Context) {
	// Retrieve the authenticated regulated entity from the request context
	// Only regulated entities can request permits
	reAny, _ := ctx.Get(middleware.ContextRegulatedEntityKey)
	re, ok := reAny.(*database.RegulatedEntities)

	// Verify that the authenticated user is a regulated entity
	// Return forbidden error if not
	if !ok || re == nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only regulated entities can request permits"})
		return
	}

	// Define the expected structure of the permit request payload
	// All fields are required for creating a valid permit request
	type requestPermitBody struct {
		ActivityDescription   string        `json:"activity_description" binding:"required"`
		ActivityStartDate     time.Time     `json:"activity_start_date" binding:"required"`
		ActivityDuration      time.Duration `json:"activity_duration" binding:"required"`
		EnvironmentalPermitID uint          `json:"environmental_permit_id" binding:"required"`
	}

	// Parse and validate the JSON request body
	var payload requestPermitBody

	//Verifies the request body contains all required fields
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid body format", "details": err.Error()})
		return
	}

	// Retrieve the environmental permit template from the database
	// This ensures the referenced permit exists
	var envPermit database.EnvironmentalPermits

	//Verify the Environmental Permit exists in the database
	if err := database.DB.First(&envPermit, payload.EnvironmentalPermitID).Error; err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environmental permit reference"})
		return
	}

	// Create a new permit request record with all required information
	permitRequest := database.PermitRequest{
		RegulatedEntityID:     re.ID,
		EnvironmentalPermitID: envPermit.ID,
		ActivityDescription:   payload.ActivityDescription,
		ActivityStartDate:     payload.ActivityStartDate,
		ActivityDuration:      payload.ActivityDuration,
		PermitFee:             envPermit.PermitFee,
	}

	// Use a database transaction to create the permit request and its initial status
	// This ensures both operations succeed or both fail
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&permitRequest).Error; err != nil {
			return err
		}
		return appendPermitRequestStatus(tx, permitRequest.ID, database.PermitRequestStatusPendingPayment, "Pending payment")
	}); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create permit request workflow", "details": err.Error()})
		return
	}

	if err := email.NotifyPendingPayment(re.Email, permitRequest.ID); err != nil {
		fmt.Println("Failed to send pending payment email notification:", err)
	}

	// Return success response with permit request details and fee information
	ctx.JSON(http.StatusCreated, gin.H{
		"message":    "Permit request created successfully",
		"id":         permitRequest.ID,
		"permit_fee": permitRequest.PermitFee,
	})
}

// SubmitPermitPayment allows a regulated entity to submit payment information for a pending permit request
// It validates the request, creates a payment record, and immediately marks the payment as approved
// so the permit request can advance directly to the Submitted state without OPS review.
func SubmitPermitPayment(ctx *gin.Context) {
	// Verify the authenticated user is a regulated entity
	reAny, _ := ctx.Get(middleware.ContextRegulatedEntityKey)
	re, ok := reAny.(*database.RegulatedEntities)
	if !ok || re == nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only regulated entities can submit permit payments"})
		return
	}

	// Extract the permit request ID from the URL parameter
	requestIDRaw := ctx.Param("request_id")
	requestID, err := strconv.ParseUint(requestIDRaw, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid permit request id"})
		return
	}

	// Define the expected structure for payment submission payload
	type submitPaymentBody struct {
		PaymentMethod        string `json:"payment_method" binding:"required"`
		LastFourDigitsOfCard string `json:"last_four_digits_of_card" binding:"required,len=4"`
		CardHolderName       string `json:"card_holder_name" binding:"required"`
	}
	var payload submitPaymentBody

	// Parse and validate the payment information
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid body format", "details": err.Error()})
		return
	}

	// Retrieve the permit request with associated payment and status information
	var permitRequest database.PermitRequest
	if err := database.DB.Preload("Payment").Preload("Statuses").First(&permitRequest, uint(requestID)).Error; err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid permit request reference"})
		return
	}

	// Ensure the permit request belongs to the authenticated regulated entity
	if permitRequest.RegulatedEntityID != re.ID {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Cannot submit payment for another regulated entity"})
		return
	}

	// Check the current status of the permit request
	latestStatus, err := latestPermitRequestStatus(database.DB, permitRequest.ID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Permit request has no workflow status"})
		return
	}

	// Only allow payment submission if the request is in "Pending Payment" status
	if latestStatus.Status != database.PermitRequestStatusPendingPayment {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Payment can only be submitted when request is Pending Payment"})
		return
	}

	// Ensure no payment has already been submitted for this request
	if permitRequest.Payment != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Payment already exists for this permit request"})
		return
	}

	// Create a new payment record with the provided information
	payment := database.Payment{
		PermitRequestID:      permitRequest.ID,
		PaymentMethod:        payload.PaymentMethod,
		LastFourDigitsOfCard: payload.LastFourDigitsOfCard,
		CardHolderName:       payload.CardHolderName,
		PaymentApproved:      true,
	}

	// Use a transaction to create the payment and update the status
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&payment).Error; err != nil {
			return err
		}
		return appendPermitRequestStatus(tx, permitRequest.ID, database.PermitRequestStatusSubmitted, "Payment automatically approved")
	}); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Failed to submit payment", "details": err.Error()})
		return
	}

	if err := email.NotifyPaymentDecision(re.Email, permitRequest.ID, "Approved"); err != nil {
		fmt.Println("Failed to send payment decision email notification:", err)
	}

	// Return success response with updated status
	ctx.JSON(http.StatusCreated, gin.H{
		"message":           "Payment submitted successfully",
		"permit_request_id": permitRequest.ID,
		"status":            database.PermitRequestStatusSubmitted,
	})
}

// ReviewPermitPaymentSubmitted allows environmental officers to start reviewing permit requests
// that have had their payments approved and submitted
func ReviewPermitPaymentSubmitted(ctx *gin.Context) {
	// Verify the authenticated user is an environmental officer
	eoAny, _ := ctx.Get(middleware.ContextEnvironmentalOfficerKey)
	eo, ok := eoAny.(*database.EnvironmentalOfficer)
	if !ok || eo == nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only environmental officers can advance submitted payment requests"})
		return
	}
	_ = eo

	// Extract permit request ID from URL
	requestIDRaw := ctx.Param("request_id")
	requestID, err := strconv.ParseUint(requestIDRaw, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid permit request id"})
		return
	}

	// Retrieve permit request with status information
	var permitRequest database.PermitRequest
	if err := database.DB.Preload("Statuses").First(&permitRequest, uint(requestID)).Error; err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid permit request reference"})
		return
	}

	// Check that the request is in the correct status for EO review
	latestStatus, err := latestPermitRequestStatus(database.DB, permitRequest.ID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Permit request has no workflow status"})
		return
	}
	if latestStatus.Status != database.PermitRequestStatusSubmitted {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Permit request must be Submitted before EO review"})
		return
	}

	// Advance the status to "Being Reviewed" by EO
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		return appendPermitRequestStatus(tx, permitRequest.ID, database.PermitRequestStatusBeingReviewed, "Being reviewed by EO")
	}); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Failed to advance permit request", "details": err.Error()})
		return
	}

	var re database.RegulatedEntities
	if err := database.DB.First(&re, permitRequest.RegulatedEntityID).Error; err != nil {
		fmt.Println("Failed to look up regulated entity for email notification:", err)
	} else if err := email.NotifyBeingReviewed(re.Email, permitRequest.ID); err != nil {
		fmt.Println("Failed to send being reviewed email notification:", err)
	}

	// Return success response
	ctx.JSON(http.StatusCreated, gin.H{
		"message":           "Permit request is now being reviewed",
		"permit_request_id": permitRequest.ID,
		"status":            database.PermitRequestStatusBeingReviewed,
	})
}

// ListAllPermitRequests returns all permit requests with workflow details.
// This endpoint is restricted to environmental officers only.
func ListAllPermitRequests(ctx *gin.Context) {
	eoAny, _ := ctx.Get(middleware.ContextEnvironmentalOfficerKey)
	if eo, ok := eoAny.(*database.EnvironmentalOfficer); !ok || eo == nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only environmental officers can list permit requests"})
		return
	}

	requests, err := allPermitRequests()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list permit requests", "details": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"items": requests})
}

// ExportPermitRequestsCSV exports permit request data to CSV for EO review workflows.
func ExportPermitRequestsCSV(ctx *gin.Context) {
	eoAny, _ := ctx.Get(middleware.ContextEnvironmentalOfficerKey)
	if eo, ok := eoAny.(*database.EnvironmentalOfficer); !ok || eo == nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only environmental officers can export permit requests"})
		return
	}

	requests, err := allPermitRequests()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export permit requests", "details": err.Error()})
		return
	}

	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)

	if err := writer.Write([]string{
		"request_id",
		"regulated_entity_email",
		"regulated_entity_id",
		"permit_template_id",
		"permit_template_name",
		"activity_description",
		"permit_fee",
		"latest_status",
		"status_history",
		"final_decision",
		"permit_issued",
		"payment_approved",
		"created_at",
	}); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to build CSV header", "details": err.Error()})
		return
	}

	for _, request := range requests {
		reEmail := ""
		if request.RegulatedEntity != nil {
			reEmail = request.RegulatedEntity.Email
		}

		permitName := ""
		if request.EnvironmentalPermit != nil {
			permitName = request.EnvironmentalPermit.PermitName
		}

		decision := ""
		if request.Decision != nil {
			decision = request.Decision.Decision
		}

		permitIssued := "No"
		if request.Permit != nil {
			permitIssued = "Yes"
		}

		paymentApproved := "No"
		if request.Payment != nil && request.Payment.PaymentApproved {
			paymentApproved = "Yes"
		}

		if err := writer.Write([]string{
			fmt.Sprintf("%d", request.ID),
			reEmail,
			fmt.Sprintf("%d", request.RegulatedEntityID),
			fmt.Sprintf("%d", request.EnvironmentalPermitID),
			permitName,
			request.ActivityDescription,
			fmt.Sprintf("%.2f", request.PermitFee),
			latestStatusFromStatuses(request.Statuses),
			statusHistoryString(request.Statuses),
			decision,
			permitIssued,
			paymentApproved,
			request.CreatedAt.UTC().Format(time.RFC3339),
		}); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to build CSV rows", "details": err.Error()})
			return
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to finalize CSV export", "details": err.Error()})
		return
	}

	ctx.Header("Content-Disposition", "attachment; filename=permit-applications.csv")
	ctx.Header("Cache-Control", "no-store")
	ctx.Data(http.StatusOK, "text/csv; charset=utf-8", buffer.Bytes())
}

// ReviewPermit allows environmental officers to make final decisions on permit requests
// They can accept or reject the permit, creating a decision record and potentially issuing a permit
func ReviewPermit(ctx *gin.Context) {
	// Verify the authenticated user is an environmental officer
	eoAny, _ := ctx.Get(middleware.ContextEnvironmentalOfficerKey)
	eo, ok := eoAny.(*database.EnvironmentalOfficer)
	if !ok || eo == nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only environmental officers can review permits"})
		return
	}
	_ = eo

	// Define expected payload for permit review decision
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

	// Retrieve permit request with all related data
	var permitRequest database.PermitRequest
	if err := database.DB.Preload("Statuses").Preload("Decision").Preload("Permit").First(&permitRequest, payload.PermitRequestID).Error; err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid permit request reference"})
		return
	}

	// Ensure no final decision has already been made
	if permitRequest.Decision != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Final permit request decision already exists"})
		return
	}

	// Check that the request is in the correct status for final review
	latestStatus, err := latestPermitRequestStatus(database.DB, permitRequest.ID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Permit request has no workflow status"})
		return
	}
	if latestStatus.Status != database.PermitRequestStatusBeingReviewed {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "EO final decision can only be applied when request is Being Reviewed"})
		return
	}

	// Create the final decision and update status in a transaction
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Create the decision record
		decision := database.PermitRequestDecision{
			PermitRequestID: payload.PermitRequestID,
			Decision:        payload.Decision,
			Description:     payload.Description,
		}
		if err := tx.Create(&decision).Error; err != nil {
			return err
		}

		// Update the status to reflect the final decision
		if err := appendPermitRequestStatus(tx, permitRequest.ID, payload.Decision, payload.Description); err != nil {
			return err
		}

		// If accepted, create a permit record
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

	var re database.RegulatedEntities
	if err := database.DB.First(&re, permitRequest.RegulatedEntityID).Error; err != nil {
		fmt.Println("Failed to look up regulated entity for email notification:", err)
	} else if err := email.NotifyFinalDecision(re.Email, permitRequest.ID, payload.Decision); err != nil {
		fmt.Println("Failed to send final decision email notification:", err)
	}

	// Return success response with the final decision
	ctx.JSON(http.StatusCreated, gin.H{
		"message":           "Permit request decision applied successfully",
		"permit_request_id": permitRequest.ID,
		"decision":          payload.Decision,
	})
}

// ListPaymentSubmittedRequests returns all permit requests with approved payments waiting for EO review
// This endpoint is restricted to environmental officers only
func ListPaymentSubmittedRequests(ctx *gin.Context) {
	// Verify the authenticated user is an environmental officer
	eoAny, _ := ctx.Get(middleware.ContextEnvironmentalOfficerKey)
	if eo, ok := eoAny.(*database.EnvironmentalOfficer); !ok || eo == nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only environmental officers can list submitted payment requests"})
		return
	} else {
		_ = eo
	}

	// Retrieve all permit requests with "Submitted" status
	requests, err := currentPermitRequestsWithStatus(database.PermitRequestStatusSubmitted)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list requests", "details": err.Error()})
		return
	}

	// Return the list of requests
	ctx.JSON(http.StatusOK, gin.H{"items": requests})
}

// ListMyPermitRequests returns every permit request owned by the authenticated regulated entity.
// The frontend uses this to refresh workflow history, latest notes, and final decision states.
func ListMyPermitRequests(ctx *gin.Context) {
	reAny, _ := ctx.Get(middleware.ContextRegulatedEntityKey)
	re, ok := reAny.(*database.RegulatedEntities)
	if !ok || re == nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only regulated entities can list their permit requests"})
		return
	}

	requests, err := currentPermitRequestsForRegulatedEntity(re.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list permit requests", "details": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"items": requests})
}

// ListEnvironmentalPermits returns the permit templates available for new applications.
// These records are seeded on startup and exposed publicly so the frontend can populate
// the permit selector without keeping its own hardcoded copy.
func ListEnvironmentalPermits(ctx *gin.Context) {
	var permits []database.EnvironmentalPermits
	if err := database.DB.Order("id asc").Find(&permits).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list environmental permits", "details": err.Error()})
		return
	}

	items := make([]gin.H, 0, len(permits))
	for _, permit := range permits {
		items = append(items, gin.H{
			"id":          permit.ID,
			"permit_name": permit.PermitName,
			"permit_fee":  permit.PermitFee,
			"description": permit.Description,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{"items": items})
}
