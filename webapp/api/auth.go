package api

import (
	"net/http"

	"project/database"
	"project/middleware"

	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	AccountType string `json:"account_type" binding:"required,oneof=regulated_entity environmental_officer ops"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required"`
}

func Login(ctx *gin.Context) {
	var payload loginRequest
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid body format", "details": err.Error()})
		return
	}

	switch payload.AccountType {
	case middleware.AccountTypeRegulatedEntity:
		var re database.RegulatedEntities
		if err := database.DB.Where("email = ?", payload.Email).First(&re).Error; err != nil || re.Password != payload.Password {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		token, err := middleware.GenerateJWT(re.ID, middleware.AccountTypeRegulatedEntity)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"token": token})
	case middleware.AccountTypeEnvironmentalOfficer:
		var eo database.EnvironmentalOfficer
		if err := database.DB.Where("email = ?", payload.Email).First(&eo).Error; err != nil || eo.Password != payload.Password {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		token, err := middleware.GenerateJWT(eo.ID, middleware.AccountTypeEnvironmentalOfficer)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"token": token})
	case middleware.AccountTypeOPS:
		var ops database.OPS
		if err := database.DB.Where("email = ?", payload.Email).First(&ops).Error; err != nil || ops.Password != payload.Password {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		token, err := middleware.GenerateJWT(ops.ID, middleware.AccountTypeOPS)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"token": token})
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported account type"})
	}
}

func WhoAmI(ctx *gin.Context) {
	reAny, _ := ctx.Get(middleware.ContextRegulatedEntityKey)
	eoAny, _ := ctx.Get(middleware.ContextEnvironmentalOfficerKey)

	if re, ok := reAny.(*database.RegulatedEntities); ok && re != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"account_type": middleware.AccountTypeRegulatedEntity,
			"account_id":   re.ID,
			"email":        re.Email,
		})
		return
	}

	if eo, ok := eoAny.(*database.EnvironmentalOfficer); ok && eo != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"account_type": middleware.AccountTypeEnvironmentalOfficer,
			"account_id":   eo.ID,
			"email":        eo.Email,
		})
		return
	}

	opsAny, _ := ctx.Get(middleware.ContextOPSKey)
	if ops, ok := opsAny.(*database.OPS); ok && ops != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"account_type": middleware.AccountTypeOPS,
			"account_id":   ops.ID,
			"email":        ops.Email,
		})
		return
	}

	ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
}
