package middleware

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"project/database"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	ContextRegulatedEntityKey       = "regulated_entity"
	ContextEnvironmentalOfficerKey  = "environmental_officer"
	ContextOPSKey                   = "ops"
	AccountTypeRegulatedEntity      = "regulated_entity"
	AccountTypeEnvironmentalOfficer = "environmental_officer"
	AccountTypeOPS                  = "ops"
)

type Claims struct {
	AccountID   uint   `json:"account_id"`
	AccountType string `json:"account_type"`
	jwt.RegisteredClaims
}

func jwtSecret() []byte {
	return []byte(database.GetEnv("JWT_SECRET", "dev-secret-change-me"))
}

func GenerateJWT(accountID uint, accountType string) (string, error) {
	claims := Claims{
		AccountID:   accountID,
		AccountType: accountType,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret())
}

func parseAuthorizationHeader(header string) (string, error) {
	if header == "" {
		return "", errors.New("missing authorization header")
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("authorization header must be Bearer <token>")
	}

	return parts[1], nil
}

func AuthRequired() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")
		if authHeader == "" {
			authHeader = ctx.GetHeader("Authentication")
		}

		tokenString, err := parseAuthorizationHeader(authHeader)
		if err != nil {
			ctx.JSON(401, gin.H{"error": "Unauthorized", "details": err.Error()})
			ctx.Abort()
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtSecret(), nil
		})
		if err != nil || !token.Valid {
			ctx.JSON(401, gin.H{"error": "Unauthorized", "details": "invalid token"})
			ctx.Abort()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			ctx.JSON(401, gin.H{"error": "Unauthorized", "details": "invalid claims"})
			ctx.Abort()
			return
		}

		switch claims.AccountType {
		case AccountTypeRegulatedEntity:
			var re database.RegulatedEntities
			if err := database.DB.First(&re, claims.AccountID).Error; err != nil {
				ctx.JSON(401, gin.H{"error": "Unauthorized", "details": "account not found"})
				ctx.Abort()
				return
			}
			ctx.Set(ContextRegulatedEntityKey, &re)
			ctx.Set(ContextEnvironmentalOfficerKey, (*database.EnvironmentalOfficer)(nil))
		case AccountTypeEnvironmentalOfficer:
			var eo database.EnvironmentalOfficer
			if err := database.DB.First(&eo, claims.AccountID).Error; err != nil {
				ctx.JSON(401, gin.H{"error": "Unauthorized", "details": "account not found"})
				ctx.Abort()
				return
			}
			ctx.Set(ContextEnvironmentalOfficerKey, &eo)
			ctx.Set(ContextRegulatedEntityKey, (*database.RegulatedEntities)(nil))
			ctx.Set(ContextOPSKey, (*database.OPS)(nil))
		case AccountTypeOPS:
			var ops database.OPS
			if err := database.DB.First(&ops, claims.AccountID).Error; err != nil {
				ctx.JSON(401, gin.H{"error": "Unauthorized", "details": "account not found"})
				ctx.Abort()
				return
			}
			ctx.Set(ContextOPSKey, &ops)
			ctx.Set(ContextRegulatedEntityKey, (*database.RegulatedEntities)(nil))
			ctx.Set(ContextEnvironmentalOfficerKey, (*database.EnvironmentalOfficer)(nil))
		default:
			ctx.JSON(401, gin.H{"error": "Unauthorized", "details": "unknown account type"})
			ctx.Abort()
			return
		}

		ctx.Set("jwt_claims", claims)
		ctx.Next()
	}
}
