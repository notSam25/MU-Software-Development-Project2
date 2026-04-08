package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"project/api"
	"project/database"
	"project/middleware"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDatabase creates an in-memory SQLite database for testing
// It migrates all necessary tables and sets the global database.DB variable
// This ensures each test starts with a clean database state
func setupTestDatabase(t *testing.T) {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "test.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	err = db.AutoMigrate(
		&database.RegulatedEntities{},
		&database.RegulatedEntitySite{},
		&database.EnvironmentalOfficer{},
		&database.OPS{},
		&database.EnvironmentalPermits{},
		&database.PermitRequest{},
		&database.PermitRequestStatus{},
		&database.PermitRequestDecision{},
		&database.Payment{},
		&database.Permit{},
	)
	if err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	database.DB = db
}

// setupRouter creates and configures a Gin router for testing
// It sets up all API routes including public and authenticated endpoints
// Returns the configured router for use in HTTP tests
func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/api/ping", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"message_text": "pong!"})
	})
	router.POST("/api/register", api.RegisterRegulatedEntity)
	router.POST("/api/login", api.Login)

	authenticated := router.Group("/api")
	authenticated.Use(middleware.AuthRequired())
	{
		authenticated.GET("/whoami", api.WhoAmI)
		authenticated.POST("/request-permit", api.RequestPermit)
		authenticated.POST("/permit-request/:request_id/submit_payment", api.SubmitPermitPayment)
		authenticated.GET("/ops/permit-requests/reviewing-payment", api.ListReviewingPaymentRequests)
		authenticated.POST("/ops/permit-request/:request_id/review_payment", api.ReviewPermitPayment)
		authenticated.GET("/eo/permit-requests/submitted-payment", api.ListPaymentSubmittedRequests)
		authenticated.POST("/eo/permit-request/:request_id/start-review", api.ReviewPermitPaymentSubmitted)
		authenticated.POST("/review-permit", api.ReviewPermit)
	}

	return router
}

// doJSONRequest is a helper function for making HTTP requests in tests
// It encodes the payload as JSON, sets appropriate headers, and returns the response recorder
// This simplifies testing API endpoints with JSON payloads
func doJSONRequest(router http.Handler, method, path string, payload any, headers map[string]string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			panic(err)
		}
	}

	req, err := http.NewRequest(method, path, &body)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func TestLoginOPSReturnsJWT(t *testing.T) {
	setupTestDatabase(t)
	router := setupRouter()

	ops := database.OPS{Name: "Operations", Email: "ops@example.com", Password: "password-123"}
	if err := database.DB.Create(&ops).Error; err != nil {
		t.Fatalf("failed to seed OPS account: %v", err)
	}

	resp := doJSONRequest(router, http.MethodPost, "/api/login", map[string]any{
		"email":        "ops@example.com",
		"password":     "password-123",
		"account_type": middleware.AccountTypeOPS,
	}, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected login to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if body["token"] == "" {
		t.Fatal("expected token in OPS login response")
	}
}

func TestPingReturnsPong(t *testing.T) {
	setupTestDatabase(t)
	router := setupRouter()

	resp := doJSONRequest(router, http.MethodGet, "/api/ping", nil, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected ping to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode ping response: %v", err)
	}
	if body["message_text"] != "pong!" {
		t.Fatalf("expected message_text pong!, got %v", body["message_text"])
	}
}

func TestRegisterRegulatedEntityCreatesAccount(t *testing.T) {
	setupTestDatabase(t)
	router := setupRouter()

	resp := doJSONRequest(router, http.MethodPost, "/api/register", map[string]any{
		"contact_person_name":  "Jane Doe",
		"password":             "password-123",
		"email":                "jane@example.com",
		"organization_name":    "Example Org",
		"organization_address": "123 Main St",
	}, nil)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected register to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}

	var count int64
	if err := database.DB.Model(&database.RegulatedEntities{}).Where("email = ?", "jane@example.com").Count(&count).Error; err != nil {
		t.Fatalf("failed to query registered account: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one registered entity, got %d", count)
	}
}

func TestWhoAmIReturnsRegulatedEntityContext(t *testing.T) {
	setupTestDatabase(t)
	router := setupRouter()

	re := database.RegulatedEntities{
		ContactPersonName:   "Jane Doe",
		Password:            "password-123",
		Email:               "jane@example.com",
		OrganizationName:    "Example Org",
		OrganizationAddress: "123 Main St",
	}
	if err := database.DB.Create(&re).Error; err != nil {
		t.Fatalf("failed to seed regulated entity: %v", err)
	}

	token, err := middleware.GenerateJWT(re.ID, middleware.AccountTypeRegulatedEntity)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	resp := doJSONRequest(router, http.MethodGet, "/api/whoami", nil, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)})
	if resp.Code != http.StatusOK {
		t.Fatalf("expected whoami to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode whoami response: %v", err)
	}
	if body["account_type"] != middleware.AccountTypeRegulatedEntity {
		t.Fatalf("expected account_type %s, got %v", middleware.AccountTypeRegulatedEntity, body["account_type"])
	}
}

func TestRequestPermitCreatesPendingPaymentStatus(t *testing.T) {
	// Set up a clean test database
	setupTestDatabase(t)

	// Create a test router
	router := setupRouter()

	// Create and seed a regulated entity for testing
	re := database.RegulatedEntities{
		ContactPersonName:   "Jane Doe",
		Password:            "password-123",
		Email:               "jane@example.com",
		OrganizationName:    "Example Org",
		OrganizationAddress: "123 Main St",
	}
	if err := database.DB.Create(&re).Error; err != nil {
		t.Fatalf("failed to seed regulated entity: %v", err)
	}

	// Create and seed an environmental permit template
	envPermit := database.EnvironmentalPermits{PermitName: "Air Quality Permit", PermitFee: 150.25, Description: "Template permit"}
	if err := database.DB.Create(&envPermit).Error; err != nil {
		t.Fatalf("failed to seed environmental permit template: %v", err)
	}

	// Generate JWT token for the regulated entity
	token, err := middleware.GenerateJWT(re.ID, middleware.AccountTypeRegulatedEntity)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	// Make POST request to create a permit request
	resp := doJSONRequest(router, http.MethodPost, "/api/request-permit", map[string]any{
		"activity_description":    "Routine maintenance",
		"activity_start_date":     "2026-03-28T20:00:00Z",
		"activity_duration":       3600000000000,
		"environmental_permit_id": envPermit.ID,
	}, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)})
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected request permit to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}

	// Parse the response body
	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode request permit response: %v", err)
	}
	if body["permit_fee"] == nil {
		t.Fatal("expected permit_fee in request-permit response")
	}

	// Verify the permit request was created in the database
	var permitRequest database.PermitRequest
	if err := database.DB.Where("id = ?", body["id"]).First(&permitRequest).Error; err != nil {
		t.Fatalf("failed to fetch permit request: %v", err)
	}

	// Verify the initial status is "Pending Payment"
	var statuses []database.PermitRequestStatus
	if err := database.DB.Where("permit_request_id = ?", permitRequest.ID).Order("id asc").Find(&statuses).Error; err != nil {
		t.Fatalf("failed to fetch statuses: %v", err)
	}
	if len(statuses) == 0 || statuses[0].Status != database.PermitRequestStatusPendingPayment {
		t.Fatalf("expected initial status Pending Payment, got %+v", statuses)
	}
}

func TestSubmitPermitPaymentMovesToReviewingPayment(t *testing.T) {
	// Set up test database
	setupTestDatabase(t)

	// Set up test router
	router := setupRouter()

	// Seed regulated entity
	re := database.RegulatedEntities{ContactPersonName: "Jane Doe", Password: "password-123", Email: "jane@example.com", OrganizationName: "Example Org", OrganizationAddress: "123 Main St"}
	if err := database.DB.Create(&re).Error; err != nil {
		t.Fatalf("failed to seed regulated entity: %v", err)
	}

	// Seed environmental permit
	envPermit := database.EnvironmentalPermits{PermitName: "Air Quality Permit", PermitFee: 150.25, Description: "Template permit"}
	if err := database.DB.Create(&envPermit).Error; err != nil {
		t.Fatalf("failed to seed environmental permit template: %v", err)
	}

	// Seed permit request in pending payment status
	permitRequest := database.PermitRequest{RegulatedEntityID: re.ID, EnvironmentalPermitID: envPermit.ID, ActivityDescription: "Routine maintenance", PermitFee: envPermit.PermitFee}
	if err := database.DB.Create(&permitRequest).Error; err != nil {
		t.Fatalf("failed to seed permit request: %v", err)
	}
	if err := database.DB.Create(&database.PermitRequestStatus{PermitRequestID: permitRequest.ID, Status: database.PermitRequestStatusPendingPayment, Description: "Pending payment"}).Error; err != nil {
		t.Fatalf("failed to seed pending payment status: %v", err)
	}

	// Generate JWT for regulated entity
	token, err := middleware.GenerateJWT(re.ID, middleware.AccountTypeRegulatedEntity)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	// Submit payment
	resp := doJSONRequest(router, http.MethodPost, fmt.Sprintf("/api/permit-request/%d/submit_payment", permitRequest.ID), map[string]any{
		"payment_method":           "card",
		"last_four_digits_of_card": "1234",
		"card_holder_name":         "Jane Doe",
	}, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)})
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected payment submission to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}

	// Verify status changed to Reviewing Payment
	var latest database.PermitRequestStatus
	if err := database.DB.Where("permit_request_id = ?", permitRequest.ID).Order("id desc").First(&latest).Error; err != nil {
		t.Fatalf("failed to fetch latest status: %v", err)
	}
	if latest.Status != database.PermitRequestStatusReviewingPayment {
		t.Fatalf("expected latest status Reviewing Payment, got %s", latest.Status)
	}
}

func TestEOFinalDecisionCreatesPermitWhenAccepted(t *testing.T) {
	// Set up test database
	setupTestDatabase(t)

	// Set up test router
	router := setupRouter()

	// Seed regulated entity
	re := database.RegulatedEntities{ContactPersonName: "Jane Doe", Password: "password-123", Email: "jane@example.com", OrganizationName: "Example Org", OrganizationAddress: "123 Main St"}
	if err := database.DB.Create(&re).Error; err != nil {
		t.Fatalf("failed to seed regulated entity: %v", err)
	}

	// Seed OPS account
	ops := database.OPS{Name: "Operations", Email: "ops@example.com", Password: "password-123"}
	if err := database.DB.Create(&ops).Error; err != nil {
		t.Fatalf("failed to seed OPS account: %v", err)
	}

	// Seed environmental officer
	eo := database.EnvironmentalOfficer{Name: "Officer Smith", Email: "eo@example.com", Password: "password-123"}
	if err := database.DB.Create(&eo).Error; err != nil {
		t.Fatalf("failed to seed EO account: %v", err)
	}

	// Seed environmental permit template
	envPermit := database.EnvironmentalPermits{PermitName: "Air Quality Permit", PermitFee: 150.25, Description: "Template permit"}
	if err := database.DB.Create(&envPermit).Error; err != nil {
		t.Fatalf("failed to seed environmental permit template: %v", err)
	}

	// Seed permit request
	permitRequest := database.PermitRequest{RegulatedEntityID: re.ID, EnvironmentalPermitID: envPermit.ID, ActivityDescription: "Routine maintenance", PermitFee: envPermit.PermitFee}
	if err := database.DB.Create(&permitRequest).Error; err != nil {
		t.Fatalf("failed to seed permit request: %v", err)
	}
	if err := database.DB.Create(&database.PermitRequestStatus{PermitRequestID: permitRequest.ID, Status: database.PermitRequestStatusPendingPayment, Description: "Pending payment"}).Error; err != nil {
		t.Fatalf("failed to seed pending payment status: %v", err)
	}

	// Generate JWT tokens for all user types
	reToken, _ := middleware.GenerateJWT(re.ID, middleware.AccountTypeRegulatedEntity)
	opsToken, _ := middleware.GenerateJWT(ops.ID, middleware.AccountTypeOPS)
	eoToken, _ := middleware.GenerateJWT(eo.ID, middleware.AccountTypeEnvironmentalOfficer)

	// Submit payment as regulated entity
	if resp := doJSONRequest(router, http.MethodPost, fmt.Sprintf("/api/permit-request/%d/submit_payment", permitRequest.ID), map[string]any{
		"payment_method":           "card",
		"last_four_digits_of_card": "1234",
		"card_holder_name":         "Jane Doe",
	}, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", reToken)}); resp.Code != http.StatusCreated {
		t.Fatalf("expected payment submission to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}

	// OPS reviews and approves payment
	if resp := doJSONRequest(router, http.MethodPost, fmt.Sprintf("/api/ops/permit-request/%d/review_payment", permitRequest.ID), map[string]any{
		"decision":    "Submitted",
		"description": "Payment approved",
	}, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", opsToken)}); resp.Code != http.StatusCreated {
		t.Fatalf("expected OPS payment review to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}

	// Environmental officer starts review
	if resp := doJSONRequest(router, http.MethodPost, fmt.Sprintf("/api/eo/permit-request/%d/start-review", permitRequest.ID), nil, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", eoToken)}); resp.Code != http.StatusCreated {
		t.Fatalf("expected EO start-review to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}

	// Environmental officer makes final decision to accept
	finalResp := doJSONRequest(router, http.MethodPost, "/api/review-permit", map[string]any{
		"permit_request_id": permitRequest.ID,
		"decision":          "Accepted",
		"description":       "Application approved",
	}, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", eoToken)})
	if finalResp.Code != http.StatusCreated {
		t.Fatalf("expected EO final approval to succeed, got %d body=%s", finalResp.Code, finalResp.Body.String())
	}

	// Verify that a permit was created
	var permitCount int64
	if err := database.DB.Model(&database.Permit{}).Where("permit_request_id = ?", permitRequest.ID).Count(&permitCount).Error; err != nil {
		t.Fatalf("failed to count permits: %v", err)
	}
	if permitCount != 1 {
		t.Fatalf("expected 1 permit after accepted final decision, got %d", permitCount)
	}
}

func TestListReviewingPaymentRequestsRequiresOPSAndFiltersCurrentStatus(t *testing.T) {
	// Set up test database
	setupTestDatabase(t)

	// Set up test router
	router := setupRouter()

	// Seed regulated entity
	re := database.RegulatedEntities{ContactPersonName: "Jane Doe", Password: "password-123", Email: "jane@example.com", OrganizationName: "Example Org", OrganizationAddress: "123 Main St"}
	if err := database.DB.Create(&re).Error; err != nil {
		t.Fatalf("failed to seed regulated entity: %v", err)
	}

	// Seed OPS account
	ops := database.OPS{Name: "Operations", Email: "ops@example.com", Password: "password-123"}
	if err := database.DB.Create(&ops).Error; err != nil {
		t.Fatalf("failed to seed OPS account: %v", err)
	}

	// Seed environmental permit template
	envPermit := database.EnvironmentalPermits{PermitName: "Air Quality Permit", PermitFee: 150.25, Description: "Template permit"}
	if err := database.DB.Create(&envPermit).Error; err != nil {
		t.Fatalf("failed to seed environmental permit template: %v", err)
	}

	// Seed permit request in reviewing payment status
	requestReviewing := database.PermitRequest{RegulatedEntityID: re.ID, EnvironmentalPermitID: envPermit.ID, ActivityDescription: "Reviewing", PermitFee: envPermit.PermitFee}
	if err := database.DB.Create(&requestReviewing).Error; err != nil {
		t.Fatalf("failed to seed reviewing request: %v", err)
	}
	if err := database.DB.Create(&database.PermitRequestStatus{PermitRequestID: requestReviewing.ID, Status: database.PermitRequestStatusReviewingPayment, Description: "Reviewing payment"}).Error; err != nil {
		t.Fatalf("failed to seed reviewing status: %v", err)
	}

	// Seed permit request in pending payment status
	requestPending := database.PermitRequest{RegulatedEntityID: re.ID, EnvironmentalPermitID: envPermit.ID, ActivityDescription: "Pending", PermitFee: envPermit.PermitFee}
	if err := database.DB.Create(&requestPending).Error; err != nil {
		t.Fatalf("failed to seed pending request: %v", err)
	}
	if err := database.DB.Create(&database.PermitRequestStatus{PermitRequestID: requestPending.ID, Status: database.PermitRequestStatusPendingPayment, Description: "Pending payment"}).Error; err != nil {
		t.Fatalf("failed to seed pending status: %v", err)
	}

	// Generate JWT tokens for OPS and regulated entity
	opsToken, _ := middleware.GenerateJWT(ops.ID, middleware.AccountTypeOPS)
	reToken, _ := middleware.GenerateJWT(re.ID, middleware.AccountTypeRegulatedEntity)

	// Verify that non-OPS users are forbidden from accessing the endpoint
	forbidden := doJSONRequest(router, http.MethodGet, "/api/ops/permit-requests/reviewing-payment", nil, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", reToken)})
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("expected non-OPS to be forbidden, got %d body=%s", forbidden.Code, forbidden.Body.String())
	}

	// OPS user can access the endpoint
	resp := doJSONRequest(router, http.MethodGet, "/api/ops/permit-requests/reviewing-payment", nil, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", opsToken)})
	if resp.Code != http.StatusOK {
		t.Fatalf("expected OPS list to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}

	// Parse response and verify only reviewing payment requests are returned
	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode OPS list response: %v", err)
	}
	items, ok := body["items"].([]any)
	if !ok {
		t.Fatalf("expected items array, got %T", body["items"])
	}
	if len(items) != 1 {
		t.Fatalf("expected exactly one reviewing-payment request, got %d", len(items))
	}
}

func TestReviewPermitPaymentRequiresOPS(t *testing.T) {
	// Set up test database
	setupTestDatabase(t)

	// Set up test router
	router := setupRouter()

	// Seed regulated entity
	re := database.RegulatedEntities{ContactPersonName: "Jane Doe", Password: "password-123", Email: "jane@example.com", OrganizationName: "Example Org", OrganizationAddress: "123 Main St"}
	if err := database.DB.Create(&re).Error; err != nil {
		t.Fatalf("failed to seed regulated entity: %v", err)
	}

	// Seed environmental permit template
	envPermit := database.EnvironmentalPermits{PermitName: "Air Quality Permit", PermitFee: 150.25, Description: "Template permit"}
	if err := database.DB.Create(&envPermit).Error; err != nil {
		t.Fatalf("failed to seed environmental permit template: %v", err)
	}

	// Seed permit request in reviewing payment status
	request := database.PermitRequest{RegulatedEntityID: re.ID, EnvironmentalPermitID: envPermit.ID, ActivityDescription: "Routine maintenance", PermitFee: envPermit.PermitFee}
	if err := database.DB.Create(&request).Error; err != nil {
		t.Fatalf("failed to seed permit request: %v", err)
	}
	if err := database.DB.Create(&database.PermitRequestStatus{PermitRequestID: request.ID, Status: database.PermitRequestStatusReviewingPayment, Description: "Reviewing payment"}).Error; err != nil {
		t.Fatalf("failed to seed reviewing status: %v", err)
	}

	// Generate JWT token for regulated entity (non-OPS)
	reToken, _ := middleware.GenerateJWT(re.ID, middleware.AccountTypeRegulatedEntity)

	// Attempt to review payment as non-OPS user - should be forbidden
	resp := doJSONRequest(router, http.MethodPost, fmt.Sprintf("/api/ops/permit-request/%d/review_payment", request.ID), map[string]any{
		"decision":    "Submitted",
		"description": "Payment approved",
	}, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", reToken)})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected non-OPS payment review to be forbidden, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestEOListSubmittedAndStartReviewMovesToBeingReviewed(t *testing.T) {
	// Set up test database
	setupTestDatabase(t)

	// Set up test router
	router := setupRouter()

	// Seed regulated entity
	re := database.RegulatedEntities{ContactPersonName: "Jane Doe", Password: "password-123", Email: "jane@example.com", OrganizationName: "Example Org", OrganizationAddress: "123 Main St"}
	if err := database.DB.Create(&re).Error; err != nil {
		t.Fatalf("failed to seed regulated entity: %v", err)
	}

	// Seed environmental officer
	eo := database.EnvironmentalOfficer{Name: "Officer Smith", Email: "eo@example.com", Password: "password-123"}
	if err := database.DB.Create(&eo).Error; err != nil {
		t.Fatalf("failed to seed EO account: %v", err)
	}

	// Seed environmental permit template
	envPermit := database.EnvironmentalPermits{PermitName: "Air Quality Permit", PermitFee: 150.25, Description: "Template permit"}
	if err := database.DB.Create(&envPermit).Error; err != nil {
		t.Fatalf("failed to seed environmental permit template: %v", err)
	}

	// Seed permit request in submitted status
	submittedRequest := database.PermitRequest{RegulatedEntityID: re.ID, EnvironmentalPermitID: envPermit.ID, ActivityDescription: "Submitted request", PermitFee: envPermit.PermitFee}
	if err := database.DB.Create(&submittedRequest).Error; err != nil {
		t.Fatalf("failed to seed submitted request: %v", err)
	}
	if err := database.DB.Create(&database.PermitRequestStatus{PermitRequestID: submittedRequest.ID, Status: database.PermitRequestStatusSubmitted, Description: "Submitted"}).Error; err != nil {
		t.Fatalf("failed to seed submitted status: %v", err)
	}

	// Seed permit request in reviewing payment status (should not appear in submitted list)
	reviewingPaymentRequest := database.PermitRequest{RegulatedEntityID: re.ID, EnvironmentalPermitID: envPermit.ID, ActivityDescription: "Not submitted", PermitFee: envPermit.PermitFee}
	if err := database.DB.Create(&reviewingPaymentRequest).Error; err != nil {
		t.Fatalf("failed to seed non-submitted request: %v", err)
	}
	if err := database.DB.Create(&database.PermitRequestStatus{PermitRequestID: reviewingPaymentRequest.ID, Status: database.PermitRequestStatusReviewingPayment, Description: "Reviewing payment"}).Error; err != nil {
		t.Fatalf("failed to seed reviewing-payment status: %v", err)
	}

	// Generate JWT token for environmental officer
	eoToken, _ := middleware.GenerateJWT(eo.ID, middleware.AccountTypeEnvironmentalOfficer)

	// List submitted permit requests
	listResp := doJSONRequest(router, http.MethodGet, "/api/eo/permit-requests/submitted-payment", nil, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", eoToken)})
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected EO submitted list to succeed, got %d body=%s", listResp.Code, listResp.Body.String())
	}

	// Parse response and verify only submitted requests are returned
	var listBody map[string]any
	if err := json.Unmarshal(listResp.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("failed to decode EO list response: %v", err)
	}
	items, ok := listBody["items"].([]any)
	if !ok {
		t.Fatalf("expected items array, got %T", listBody["items"])
	}
	if len(items) != 1 {
		t.Fatalf("expected exactly one submitted request, got %d", len(items))
	}

	// Start review on the submitted request
	startResp := doJSONRequest(router, http.MethodPost, fmt.Sprintf("/api/eo/permit-request/%d/start-review", submittedRequest.ID), nil, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", eoToken)})
	if startResp.Code != http.StatusCreated {
		t.Fatalf("expected EO start-review to succeed, got %d body=%s", startResp.Code, startResp.Body.String())
	}

	// Verify status changed to Being Reviewed
	var latest database.PermitRequestStatus
	if err := database.DB.Where("permit_request_id = ?", submittedRequest.ID).Order("id desc").First(&latest).Error; err != nil {
		t.Fatalf("failed to fetch latest status: %v", err)
	}
	if latest.Status != database.PermitRequestStatusBeingReviewed {
		t.Fatalf("expected latest status Being Reviewed, got %s", latest.Status)
	}
}

func TestReviewPermitRequiresEnvironmentalOfficer(t *testing.T) {
	// Set up test database
	setupTestDatabase(t)

	// Set up test router
	router := setupRouter()

	// Seed regulated entity
	re := database.RegulatedEntities{ContactPersonName: "Jane Doe", Password: "password-123", Email: "jane@example.com", OrganizationName: "Example Org", OrganizationAddress: "123 Main St"}
	if err := database.DB.Create(&re).Error; err != nil {
		t.Fatalf("failed to seed regulated entity: %v", err)
	}

	// Seed OPS account
	ops := database.OPS{Name: "Operations", Email: "ops@example.com", Password: "password-123"}
	if err := database.DB.Create(&ops).Error; err != nil {
		t.Fatalf("failed to seed OPS account: %v", err)
	}

	// Seed environmental permit template
	envPermit := database.EnvironmentalPermits{PermitName: "Air Quality Permit", PermitFee: 150.25, Description: "Template permit"}
	if err := database.DB.Create(&envPermit).Error; err != nil {
		t.Fatalf("failed to seed environmental permit template: %v", err)
	}

	// Seed permit request in being reviewed status
	request := database.PermitRequest{RegulatedEntityID: re.ID, EnvironmentalPermitID: envPermit.ID, ActivityDescription: "Routine maintenance", PermitFee: envPermit.PermitFee}
	if err := database.DB.Create(&request).Error; err != nil {
		t.Fatalf("failed to seed permit request: %v", err)
	}
	if err := database.DB.Create(&database.PermitRequestStatus{PermitRequestID: request.ID, Status: database.PermitRequestStatusBeingReviewed, Description: "Being reviewed by EO"}).Error; err != nil {
		t.Fatalf("failed to seed being reviewed status: %v", err)
	}

	// Generate JWT tokens for regulated entity and OPS
	reToken, _ := middleware.GenerateJWT(re.ID, middleware.AccountTypeRegulatedEntity)
	opsToken, _ := middleware.GenerateJWT(ops.ID, middleware.AccountTypeOPS)

	// Prepare review payload
	payload := map[string]any{
		"permit_request_id": request.ID,
		"decision":          "Accepted",
		"description":       "Application approved",
	}

	// Attempt to review permit as regulated entity - should be forbidden
	reResp := doJSONRequest(router, http.MethodPost, "/api/review-permit", payload, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", reToken)})
	if reResp.Code != http.StatusForbidden {
		t.Fatalf("expected RE review-permit to be forbidden, got %d body=%s", reResp.Code, reResp.Body.String())
	}

	// Attempt to review permit as OPS - should be forbidden
	opsResp := doJSONRequest(router, http.MethodPost, "/api/review-permit", payload, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", opsToken)})
	if opsResp.Code != http.StatusForbidden {
		t.Fatalf("expected OPS review-permit to be forbidden, got %d body=%s", opsResp.Code, opsResp.Body.String())
	}
}

func TestEOFinalDecisionRejectedDoesNotCreatePermit(t *testing.T) {
	// Set up test database
	setupTestDatabase(t)

	// Set up test router
	router := setupRouter()

	// Seed regulated entity
	re := database.RegulatedEntities{ContactPersonName: "Jane Doe", Password: "password-123", Email: "jane@example.com", OrganizationName: "Example Org", OrganizationAddress: "123 Main St"}
	if err := database.DB.Create(&re).Error; err != nil {
		t.Fatalf("failed to seed regulated entity: %v", err)
	}

	// Seed environmental officer
	eo := database.EnvironmentalOfficer{Name: "Officer Smith", Email: "eo@example.com", Password: "password-123"}
	if err := database.DB.Create(&eo).Error; err != nil {
		t.Fatalf("failed to seed EO account: %v", err)
	}

	// Seed environmental permit template
	envPermit := database.EnvironmentalPermits{PermitName: "Air Quality Permit", PermitFee: 150.25, Description: "Template permit"}
	if err := database.DB.Create(&envPermit).Error; err != nil {
		t.Fatalf("failed to seed environmental permit template: %v", err)
	}

	// Seed permit request in being reviewed status
	permitRequest := database.PermitRequest{RegulatedEntityID: re.ID, EnvironmentalPermitID: envPermit.ID, ActivityDescription: "Routine maintenance", PermitFee: envPermit.PermitFee}
	if err := database.DB.Create(&permitRequest).Error; err != nil {
		t.Fatalf("failed to seed permit request: %v", err)
	}
	if err := database.DB.Create(&database.PermitRequestStatus{PermitRequestID: permitRequest.ID, Status: database.PermitRequestStatusBeingReviewed, Description: "Being reviewed by EO"}).Error; err != nil {
		t.Fatalf("failed to seed being reviewed status: %v", err)
	}

	// Generate JWT token for environmental officer
	eoToken, _ := middleware.GenerateJWT(eo.ID, middleware.AccountTypeEnvironmentalOfficer)

	// Environmental officer makes final decision to reject
	finalResp := doJSONRequest(router, http.MethodPost, "/api/review-permit", map[string]any{
		"permit_request_id": permitRequest.ID,
		"decision":          "Rejected",
		"description":       "Application rejected",
	}, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", eoToken)})
	if finalResp.Code != http.StatusCreated {
		t.Fatalf("expected EO final rejection to succeed, got %d body=%s", finalResp.Code, finalResp.Body.String())
	}

	// Verify decision record was created
	var decision database.PermitRequestDecision
	if err := database.DB.Where("permit_request_id = ?", permitRequest.ID).First(&decision).Error; err != nil {
		t.Fatalf("expected decision record to exist: %v", err)
	}
	if decision.Decision != database.PermitRequestStatusRejected {
		t.Fatalf("expected decision Rejected, got %s", decision.Decision)
	}

	// Verify status changed to Rejected
	var latestStatus database.PermitRequestStatus
	if err := database.DB.Where("permit_request_id = ?", permitRequest.ID).Order("id desc").First(&latestStatus).Error; err != nil {
		t.Fatalf("failed to fetch latest status: %v", err)
	}
	if latestStatus.Status != database.PermitRequestStatusRejected {
		t.Fatalf("expected latest status Rejected, got %s", latestStatus.Status)
	}

	// Verify no permit was created
	var permitCount int64
	if err := database.DB.Model(&database.Permit{}).Where("permit_request_id = ?", permitRequest.ID).Count(&permitCount).Error; err != nil {
		t.Fatalf("failed to count permits: %v", err)
	}
	if permitCount != 0 {
		t.Fatalf("expected no permit after rejected final decision, got %d", permitCount)
	}
}

func TestPermitWorkflowSequentialValidation(t *testing.T) {
	// Set up test database
	setupTestDatabase(t)

	// Set up test router
	router := setupRouter()

	// Seed all required user accounts
	re := database.RegulatedEntities{ContactPersonName: "Jane Doe", Password: "password-123", Email: "jane@example.com", OrganizationName: "Example Org", OrganizationAddress: "123 Main St"}
	if err := database.DB.Create(&re).Error; err != nil {
		t.Fatalf("failed to seed regulated entity: %v", err)
	}
	ops := database.OPS{Name: "Operations", Email: "ops@example.com", Password: "password-123"}
	if err := database.DB.Create(&ops).Error; err != nil {
		t.Fatalf("failed to seed OPS account: %v", err)
	}
	eo := database.EnvironmentalOfficer{Name: "Officer Smith", Email: "eo@example.com", Password: "password-123"}
	if err := database.DB.Create(&eo).Error; err != nil {
		t.Fatalf("failed to seed EO account: %v", err)
	}
	envPermit := database.EnvironmentalPermits{PermitName: "Air Quality Permit", PermitFee: 150.25, Description: "Template permit"}
	if err := database.DB.Create(&envPermit).Error; err != nil {
		t.Fatalf("failed to seed environmental permit template: %v", err)
	}

	// Generate JWT tokens for all user types
	reToken, _ := middleware.GenerateJWT(re.ID, middleware.AccountTypeRegulatedEntity)
	opsToken, _ := middleware.GenerateJWT(ops.ID, middleware.AccountTypeOPS)
	eoToken, _ := middleware.GenerateJWT(eo.ID, middleware.AccountTypeEnvironmentalOfficer)

	// Request a permit as regulated entity
	requestResp := doJSONRequest(router, http.MethodPost, "/api/request-permit", map[string]any{
		"activity_description":    "Routine maintenance",
		"activity_start_date":     "2026-03-28T20:00:00Z",
		"activity_duration":       3600000000000,
		"environmental_permit_id": envPermit.ID,
	}, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", reToken)})
	if requestResp.Code != http.StatusCreated {
		t.Fatalf("expected request permit to succeed, got %d body=%s", requestResp.Code, requestResp.Body.String())
	}

	// Extract request ID from response
	var requestBody map[string]any
	if err := json.Unmarshal(requestResp.Body.Bytes(), &requestBody); err != nil {
		t.Fatalf("failed to decode request-permit response: %v", err)
	}

	requestIDFloat, ok := requestBody["id"].(float64)
	if !ok {
		t.Fatalf("expected numeric request id in response, got %T", requestBody["id"])
	}
	requestID := uint(requestIDFloat)

	// Verify EO cannot start review before payment is submitted
	if resp := doJSONRequest(router, http.MethodPost, fmt.Sprintf("/api/eo/permit-request/%d/start-review", requestID), nil, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", eoToken)}); resp.Code != http.StatusBadRequest {
		t.Fatalf("expected EO start-review before Submitted to fail, got %d body=%s", resp.Code, resp.Body.String())
	}

	// Verify OPS cannot review payment before payment is submitted
	if resp := doJSONRequest(router, http.MethodPost, fmt.Sprintf("/api/ops/permit-request/%d/review_payment", requestID), map[string]any{
		"decision":    "Submitted",
		"description": "Payment approved",
	}, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", opsToken)}); resp.Code != http.StatusBadRequest {
		t.Fatalf("expected OPS payment review before payment submission to fail, got %d body=%s", resp.Code, resp.Body.String())
	}

	// Verify only regulated entity can submit payment
	if resp := doJSONRequest(router, http.MethodPost, fmt.Sprintf("/api/permit-request/%d/submit_payment", requestID), map[string]any{
		"payment_method":           "card",
		"last_four_digits_of_card": "1234",
		"card_holder_name":         "Jane Doe",
	}, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", eoToken)}); resp.Code != http.StatusForbidden {
		t.Fatalf("expected non-RE payment submit to be forbidden, got %d body=%s", resp.Code, resp.Body.String())
	}

	// Submit payment as regulated entity
	paymentResp := doJSONRequest(router, http.MethodPost, fmt.Sprintf("/api/permit-request/%d/submit_payment", requestID), map[string]any{
		"payment_method":           "card",
		"last_four_digits_of_card": "1234",
		"card_holder_name":         "Jane Doe",
	}, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", reToken)})
	if paymentResp.Code != http.StatusCreated {
		t.Fatalf("expected payment submission to succeed, got %d body=%s", paymentResp.Code, paymentResp.Body.String())
	}

	// Verify EO cannot make final decision before being reviewed
	if resp := doJSONRequest(router, http.MethodPost, "/api/review-permit", map[string]any{
		"permit_request_id": requestID,
		"decision":          "Accepted",
		"description":       "Premature decision",
	}, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", eoToken)}); resp.Code != http.StatusBadRequest {
		t.Fatalf("expected EO final decision before Being Reviewed to fail, got %d body=%s", resp.Code, resp.Body.String())
	}

	// OPS can now list reviewing payment requests
	opsListResp := doJSONRequest(router, http.MethodGet, "/api/ops/permit-requests/reviewing-payment", nil, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", opsToken)})
	if opsListResp.Code != http.StatusOK {
		t.Fatalf("expected OPS reviewing-payment list to succeed, got %d body=%s", opsListResp.Code, opsListResp.Body.String())
	}
	var opsListBody map[string]any
	if err := json.Unmarshal(opsListResp.Body.Bytes(), &opsListBody); err != nil {
		t.Fatalf("failed to decode OPS list response: %v", err)
	}
	opsItems, ok := opsListBody["items"].([]any)
	if !ok || len(opsItems) != 1 {
		t.Fatalf("expected exactly one reviewing-payment item, got %v", opsListBody["items"])
	}

	// OPS reviews and approves payment
	opsReviewResp := doJSONRequest(router, http.MethodPost, fmt.Sprintf("/api/ops/permit-request/%d/review_payment", requestID), map[string]any{
		"decision":    "Submitted",
		"description": "Payment approved",
	}, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", opsToken)})
	if opsReviewResp.Code != http.StatusCreated {
		t.Fatalf("expected OPS payment review to succeed, got %d body=%s", opsReviewResp.Code, opsReviewResp.Body.String())
	}

	// EO can now list submitted payment requests
	eoListResp := doJSONRequest(router, http.MethodGet, "/api/eo/permit-requests/submitted-payment", nil, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", eoToken)})
	if eoListResp.Code != http.StatusOK {
		t.Fatalf("expected EO submitted-payment list to succeed, got %d body=%s", eoListResp.Code, eoListResp.Body.String())
	}
	var eoListBody map[string]any
	if err := json.Unmarshal(eoListResp.Body.Bytes(), &eoListBody); err != nil {
		t.Fatalf("failed to decode EO list response: %v", err)
	}
	eoItems, ok := eoListBody["items"].([]any)
	if !ok || len(eoItems) != 1 {
		t.Fatalf("expected exactly one submitted-payment item, got %v", eoListBody["items"])
	}

	// EO starts review
	startReviewResp := doJSONRequest(router, http.MethodPost, fmt.Sprintf("/api/eo/permit-request/%d/start-review", requestID), nil, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", eoToken)})
	if startReviewResp.Code != http.StatusCreated {
		t.Fatalf("expected EO start-review to succeed, got %d body=%s", startReviewResp.Code, startReviewResp.Body.String())
	}

	// EO makes final decision to accept
	finalResp := doJSONRequest(router, http.MethodPost, "/api/review-permit", map[string]any{
		"permit_request_id": requestID,
		"decision":          "Accepted",
		"description":       "Application approved",
	}, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", eoToken)})
	if finalResp.Code != http.StatusCreated {
		t.Fatalf("expected EO final decision to succeed, got %d body=%s", finalResp.Code, finalResp.Body.String())
	}

	// Verify permit was created
	var permitCount int64
	if err := database.DB.Model(&database.Permit{}).Where("permit_request_id = ?", requestID).Count(&permitCount).Error; err != nil {
		t.Fatalf("failed to count permits: %v", err)
	}
	if permitCount != 1 {
		t.Fatalf("expected one permit after accepted final decision, got %d", permitCount)
	}

	// Verify duplicate final decisions are not allowed
	if resp := doJSONRequest(router, http.MethodPost, "/api/review-permit", map[string]any{
		"permit_request_id": requestID,
		"decision":          "Accepted",
		"description":       "Duplicate final decision",
	}, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", eoToken)}); resp.Code != http.StatusBadRequest {
		t.Fatalf("expected duplicate final decision to fail, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestWhoAmIRejectsInvalidJWT(t *testing.T) {
	// Set up test database
	setupTestDatabase(t)

	// Set up test router
	router := setupRouter()

	// Attempt to access whoami endpoint with invalid JWT
	resp := doJSONRequest(router, http.MethodGet, "/api/whoami", nil, map[string]string{"Authorization": "Bearer invalid.jwt.token"})
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected invalid JWT to be unauthorized, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	setupTestDatabase(t)
	router := setupRouter()

	first := doJSONRequest(router, http.MethodPost, "/api/register", map[string]any{
		"contact_person_name":  "Jane Doe",
		"password":             "password-123",
		"email":                "jane@example.com",
		"organization_name":    "Example Org",
		"organization_address": "123 Main St",
	}, nil)
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first register request to succeed, got %d body=%s", first.Code, first.Body.String())
	}

	second := doJSONRequest(router, http.MethodPost, "/api/register", map[string]any{
		"contact_person_name":  "Jane Doe",
		"password":             "password-123",
		"email":                "jane@example.com",
		"organization_name":    "Example Org",
		"organization_address": "123 Main St",
	}, nil)
	if second.Code != http.StatusBadRequest {
		t.Fatalf("expected duplicate email register to fail, got %d body=%s", second.Code, second.Body.String())
	}
}

func TestReviewPermitPaymentSubmittedRequiresPaymentRecord(t *testing.T) {
	setupTestDatabase(t)
	router := setupRouter()

	re := database.RegulatedEntities{ContactPersonName: "Jane Doe", Password: "password-123", Email: "jane@example.com", OrganizationName: "Example Org", OrganizationAddress: "123 Main St"}
	if err := database.DB.Create(&re).Error; err != nil {
		t.Fatalf("failed to seed regulated entity: %v", err)
	}
	ops := database.OPS{Name: "Operations", Email: "ops@example.com", Password: "password-123"}
	if err := database.DB.Create(&ops).Error; err != nil {
		t.Fatalf("failed to seed OPS account: %v", err)
	}
	envPermit := database.EnvironmentalPermits{PermitName: "Air Quality Permit", PermitFee: 150.25, Description: "Template permit"}
	if err := database.DB.Create(&envPermit).Error; err != nil {
		t.Fatalf("failed to seed environmental permit template: %v", err)
	}

	request := database.PermitRequest{RegulatedEntityID: re.ID, EnvironmentalPermitID: envPermit.ID, ActivityDescription: "Routine maintenance", PermitFee: envPermit.PermitFee}
	if err := database.DB.Create(&request).Error; err != nil {
		t.Fatalf("failed to seed permit request: %v", err)
	}
	if err := database.DB.Create(&database.PermitRequestStatus{PermitRequestID: request.ID, Status: database.PermitRequestStatusReviewingPayment, Description: "Reviewing payment"}).Error; err != nil {
		t.Fatalf("failed to seed reviewing payment status: %v", err)
	}

	opsToken, _ := middleware.GenerateJWT(ops.ID, middleware.AccountTypeOPS)
	resp := doJSONRequest(router, http.MethodPost, fmt.Sprintf("/api/ops/permit-request/%d/review_payment", request.ID), map[string]any{
		"decision":    "Submitted",
		"description": "Payment approved",
	}, map[string]string{"Authorization": fmt.Sprintf("Bearer %s", opsToken)})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected Submitted decision without payment record to fail, got %d body=%s", resp.Code, resp.Body.String())
	}

	var latest database.PermitRequestStatus
	if err := database.DB.Where("permit_request_id = ?", request.ID).Order("id desc").First(&latest).Error; err != nil {
		t.Fatalf("failed to fetch latest status: %v", err)
	}
	if latest.Status != database.PermitRequestStatusReviewingPayment {
		t.Fatalf("expected status to remain Reviewing Payment, got %s", latest.Status)
	}
}
