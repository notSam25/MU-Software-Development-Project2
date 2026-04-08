package api

import (
	"net/http"

	"project/database"
	"project/middleware"

	"github.com/gin-gonic/gin"
)


//Defines variables needed for structure of a login request 
type loginRequest struct {
	AccountType string `json:"account_type" binding:"required,oneof=regulated_entity environmental_officer ops"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required"`
}

//Reads incoming requests and checks for all required fields 
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

		//If token generation is unsucessful return 500 error and stop execution
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

		//If token generation is unsucessful return 500 error and stop execution
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		//Send login token back to Environmental Officer with 200 OK response
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

	opsAny, _ := ctx.Get(middleware.ContextOPSKey)
	if ops, ok := opsAny.(*database.OPS); ok && ops != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"account_type": middleware.AccountTypeOPS,
			"account_id":   ops.ID,
			"email":        ops.Email,
		})
		return
	}

	//If neither Regulated Entity nor Environmental Object is identified, Unauthorized error is sent
	ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
}
