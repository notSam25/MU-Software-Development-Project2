package api

import (
	"net/http"
	"strings"

	"project/database"
	"project/middleware"

	"github.com/gin-gonic/gin"
)

// Defines variables needed for structure of a login request
type loginRequest struct {
	AccountType string `json:"account_type" binding:"required,oneof=regulated_entity environmental_officer"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

func normalizeAccountEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// Reads incoming requests and checks for all required fields
func Login(ctx *gin.Context) {
	var payload loginRequest

	//Populates the payload variable with fields from incoming JSON request
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid body format", "details": err.Error()})
		return
	}

	//Switch statement to verify each type of account (RE and EO)
	switch payload.AccountType {

	//Verifies login credentials for the Regulated Entity account type
	case middleware.AccountTypeRegulatedEntity:
		var re database.RegulatedEntities

		//Verifies Regulated Entity email and password
		//If credentials are incorrect "Invalid credentials" error is shown
		if err := database.DB.Where("email = ?", payload.Email).First(&re).Error; err != nil || re.Password != payload.Password {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		//Generates login token containing the Regulated Entity's ID and account type
		token, err := middleware.GenerateJWT(re.ID, middleware.AccountTypeRegulatedEntity)

		//If token generation is unsuccessful return 500 error and stop execution
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		//Send login token back to Regulated Entity with 200 OK response
		ctx.JSON(http.StatusOK, gin.H{"token": token})

	//Verifies login credentials for the Environmental Officer account type
	case middleware.AccountTypeEnvironmentalOfficer:
		var eo database.EnvironmentalOfficer

		//Verifies Environmental Officer email and password
		//If credentials are incorrect "Invalid credentials" error is shown
		if err := database.DB.Where("email = ?", payload.Email).First(&eo).Error; err != nil || eo.Password != payload.Password {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		//Generates login token containing the Environmental Officer's ID and account type
		token, err := middleware.GenerateJWT(eo.ID, middleware.AccountTypeEnvironmentalOfficer)

		//If token generation is unsuccessful return 500 error and stop execution
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		//Send login token back to Environmental Officer with 200 OK response
		ctx.JSON(http.StatusOK, gin.H{"token": token})
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported account type"})
	}
}

func WhoAmI(ctx *gin.Context) {
	reAny, _ := ctx.Get(middleware.ContextRegulatedEntityKey)
	eoAny, _ := ctx.Get(middleware.ContextEnvironmentalOfficerKey)

	//Checks to see if the stored user is a Regulated Entity Object
	//If the object is a Regulated Entity returns their account info
	if re, ok := reAny.(*database.RegulatedEntities); ok && re != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"account_type": middleware.AccountTypeRegulatedEntity,
			"account_id":   re.ID,
			"email":        re.Email,
		})
		return
	}

	//Checks to see if the stored user is an Environmental Officer Object
	//If the object is an Environmental Officer returns their account info
	if eo, ok := eoAny.(*database.EnvironmentalOfficer); ok && eo != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"account_type": middleware.AccountTypeEnvironmentalOfficer,
			"account_id":   eo.ID,
			"email":        eo.Email,
		})
		return
	}

	//If neither Regulated Entity nor Environmental Object is identified, Unauthorized error is sent
	ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
}

func GetAccount(ctx *gin.Context) {
	reAny, _ := ctx.Get(middleware.ContextRegulatedEntityKey)
	eoAny, _ := ctx.Get(middleware.ContextEnvironmentalOfficerKey)

	if re, ok := reAny.(*database.RegulatedEntities); ok && re != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"account_type":         middleware.AccountTypeRegulatedEntity,
			"account_id":           re.ID,
			"email":                re.Email,
			"contact_person_name":  re.ContactPersonName,
			"organization_name":    re.OrganizationName,
			"organization_address": re.OrganizationAddress,
		})
		return
	}

	if eo, ok := eoAny.(*database.EnvironmentalOfficer); ok && eo != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"account_type": middleware.AccountTypeEnvironmentalOfficer,
			"account_id":   eo.ID,
			"email":        eo.Email,
			"name":         eo.Name,
		})
		return
	}

	ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
}

func UpdateAccount(ctx *gin.Context) {
	reAny, _ := ctx.Get(middleware.ContextRegulatedEntityKey)
	eoAny, _ := ctx.Get(middleware.ContextEnvironmentalOfficerKey)

	if re, ok := reAny.(*database.RegulatedEntities); ok && re != nil {
		type updateRegulatedEntityAccountBody struct {
			ContactPersonName   *string `json:"contact_person_name"`
			Email               *string `json:"email" binding:"omitempty,email"`
			OrganizationName    *string `json:"organization_name"`
			OrganizationAddress *string `json:"organization_address"`
		}

		var payload updateRegulatedEntityAccountBody
		if err := ctx.ShouldBindJSON(&payload); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid body format", "details": err.Error()})
			return
		}

		changed := false

		if payload.ContactPersonName != nil {
			value := strings.TrimSpace(*payload.ContactPersonName)
			if value == "" {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "contact_person_name cannot be empty"})
				return
			}
			re.ContactPersonName = value
			changed = true
		}

		if payload.Email != nil {
			value := normalizeAccountEmail(*payload.Email)
			if value == "" {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "email cannot be empty"})
				return
			}
			re.Email = value
			changed = true
		}

		if payload.OrganizationName != nil {
			value := strings.TrimSpace(*payload.OrganizationName)
			if value == "" {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "organization_name cannot be empty"})
				return
			}
			re.OrganizationName = value
			changed = true
		}

		if payload.OrganizationAddress != nil {
			value := strings.TrimSpace(*payload.OrganizationAddress)
			if value == "" {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "organization_address cannot be empty"})
				return
			}
			re.OrganizationAddress = value
			changed = true
		}

		if !changed {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "No account fields provided for update"})
			return
		}

		if err := database.DB.Save(re).Error; err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Failed to update account", "details": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"message":              "Account updated successfully",
			"account_type":         middleware.AccountTypeRegulatedEntity,
			"account_id":           re.ID,
			"email":                re.Email,
			"contact_person_name":  re.ContactPersonName,
			"organization_name":    re.OrganizationName,
			"organization_address": re.OrganizationAddress,
		})
		return
	}

	if eo, ok := eoAny.(*database.EnvironmentalOfficer); ok && eo != nil {
		type updateEnvironmentalOfficerAccountBody struct {
			Name  *string `json:"name"`
			Email *string `json:"email" binding:"omitempty,email"`
		}

		var payload updateEnvironmentalOfficerAccountBody
		if err := ctx.ShouldBindJSON(&payload); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid body format", "details": err.Error()})
			return
		}

		changed := false

		if payload.Name != nil {
			value := strings.TrimSpace(*payload.Name)
			if value == "" {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "name cannot be empty"})
				return
			}
			eo.Name = value
			changed = true
		}

		if payload.Email != nil {
			value := normalizeAccountEmail(*payload.Email)
			if value == "" {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "email cannot be empty"})
				return
			}
			eo.Email = value
			changed = true
		}

		if !changed {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "No account fields provided for update"})
			return
		}

		if err := database.DB.Save(eo).Error; err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Failed to update account", "details": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"message":      "Account updated successfully",
			"account_type": middleware.AccountTypeEnvironmentalOfficer,
			"account_id":   eo.ID,
			"email":        eo.Email,
			"name":         eo.Name,
		})
		return
	}

	ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
}

func ChangePassword(ctx *gin.Context) {
	reAny, _ := ctx.Get(middleware.ContextRegulatedEntityKey)
	eoAny, _ := ctx.Get(middleware.ContextEnvironmentalOfficerKey)

	var payload changePasswordRequest
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid body format", "details": err.Error()})
		return
	}

	if payload.CurrentPassword == payload.NewPassword {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "New password must be different from current password"})
		return
	}

	if re, ok := reAny.(*database.RegulatedEntities); ok && re != nil {
		if re.Password != payload.CurrentPassword {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid current password"})
			return
		}

		if err := database.DB.Model(re).Update("password", payload.NewPassword).Error; err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Failed to update password", "details": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
		return
	}

	if eo, ok := eoAny.(*database.EnvironmentalOfficer); ok && eo != nil {
		if eo.Password != payload.CurrentPassword {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid current password"})
			return
		}

		if err := database.DB.Model(eo).Update("password", payload.NewPassword).Error; err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Failed to update password", "details": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
		return
	}

	ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
}
