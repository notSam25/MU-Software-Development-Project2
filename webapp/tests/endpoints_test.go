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
		&database.PermitRequest{},
		&database.EnvironmentalPermits{},
		&database.PermitRequestDecision{},
		&database.Payment{},
		&database.Permit{},
	)
	if err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	database.DB = db
}

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	apiGroup := router.Group("/api")
	{
		apiGroup.GET("/ping", func(ctx *gin.Context) {
			ctx.JSON(http.StatusOK, gin.H{"message_text": "pong!"})
		})

		apiGroup.POST("/register", api.RegisterRegulatedEntity)
		apiGroup.POST("/login", api.Login)

		protected := apiGroup.Group("")
		protected.Use(middleware.AuthRequired())
		protected.GET("/whoami", api.WhoAmI)
	}

	return router
}

func doJSONRequest(router *gin.Engine, method string, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	var reqBody []byte
	if body != nil {
		reqBody, _ = json.Marshal(body)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func TestPingEndpoint(t *testing.T) {
	setupTestDatabase(t)
	router := setupRouter()

	resp := doJSONRequest(router, http.MethodGet, "/api/ping", nil, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if payload["message_text"] != "pong!" {
		t.Fatalf("expected message_text=pong!, got %v", payload["message_text"])
	}
}

func TestRegisterRegulatedEntityRequiresFields(t *testing.T) {
	setupTestDatabase(t)
	router := setupRouter()

	resp := doJSONRequest(router, http.MethodPost, "/api/register", map[string]any{}, nil)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.Code)
	}
}

func TestRegisterRegulatedEntitySuccess(t *testing.T) {
	setupTestDatabase(t)
	router := setupRouter()

	payload := map[string]any{
		"contact_person_name":  "Jane Doe",
		"password":             "password-123",
		"email":                "jane@example.com",
		"organization_name":    "Example Org",
		"organization_address": "123 Main St",
	}

	resp := doJSONRequest(router, http.MethodPost, "/api/register", payload, nil)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", resp.Code, resp.Body.String())
	}

	var count int64
	if err := database.DB.Model(&database.RegulatedEntities{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count entities: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 regulated entity in DB, got %d", count)
	}
}

func TestLoginRegulatedEntityReturnsJWT(t *testing.T) {
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

	payload := map[string]any{
		"account_type": "regulated_entity",
		"email":        "jane@example.com",
		"password":     "password-123",
	}

	resp := doJSONRequest(router, http.MethodPost, "/api/login", payload, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", resp.Code, resp.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode login JSON: %v", err)
	}

	if body["token"] == "" {
		t.Fatal("expected token in login response")
	}
}

func TestProtectedWhoAmIRequiresJWT(t *testing.T) {
	setupTestDatabase(t)
	router := setupRouter()

	resp := doJSONRequest(router, http.MethodGet, "/api/whoami", nil, nil)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.Code)
	}
}

func TestProtectedWhoAmIForRegulatedEntity(t *testing.T) {
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

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}
	resp := doJSONRequest(router, http.MethodGet, "/api/whoami", nil, headers)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", resp.Code, resp.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode whoami JSON: %v", err)
	}

	if body["account_type"] != middleware.AccountTypeRegulatedEntity {
		t.Fatalf("expected account_type=%s, got %v", middleware.AccountTypeRegulatedEntity, body["account_type"])
	}
}

func TestProtectedWhoAmIForEnvironmentalOfficer(t *testing.T) {
	setupTestDatabase(t)
	router := setupRouter()

	eo := database.EnvironmentalOfficer{
		Name:     "Officer Smith",
		Email:    "eo@example.com",
		Password: "password-123",
	}
	if err := database.DB.Create(&eo).Error; err != nil {
		t.Fatalf("failed to seed environmental officer: %v", err)
	}

	token, err := middleware.GenerateJWT(eo.ID, middleware.AccountTypeEnvironmentalOfficer)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}
	resp := doJSONRequest(router, http.MethodGet, "/api/whoami", nil, headers)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", resp.Code, resp.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode whoami JSON: %v", err)
	}

	if body["account_type"] != middleware.AccountTypeEnvironmentalOfficer {
		t.Fatalf("expected account_type=%s, got %v", middleware.AccountTypeEnvironmentalOfficer, body["account_type"])
	}
}
