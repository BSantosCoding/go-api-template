// internal/api/middleware/auth.go
package middleware

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid" // For parsing UUID from claim
)

const (
	authorizationHeader = "Authorization"
	userCtx             = "userID" // Key to store user ID in context
)

// JWTAuthMiddleware creates a Gin middleware for JWT authentication.
func JWTAuthMiddleware(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader(authorizationHeader)
		if authHeader == "" {
			log.Println("Auth middleware: Authorization header missing")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			return
		}

		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 || strings.ToLower(headerParts[0]) != "bearer" {
			log.Println("Auth middleware: Invalid Authorization header format")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization header format"})
			return
		}

		tokenString := headerParts[1]

		// Parse and validate the token
		token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
			// Validate the alg is what you expect:
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(jwtSecret), nil
		})

		if err != nil {
			log.Printf("Auth middleware: Error parsing token: %v", err)
			if errors.Is(err, jwt.ErrTokenExpired) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token has expired"})
			} else {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			}
			return
		}

		if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
			// Token is valid, extract user ID (subject)
			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				log.Printf("Auth middleware: Error parsing user ID from token subject '%s': %v", claims.Subject, err)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid user identifier in token"})
				return
			}

			// Store user ID in context for downstream handlers
			c.Set(userCtx, userID)
			log.Printf("Auth middleware: User %s authenticated", userID)
			c.Next() // Proceed to the next handler
		} else {
			log.Println("Auth middleware: Invalid token claims or token is not valid")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		}
	}
}

// Helper function to get user ID from context (optional but convenient)
func GetUserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	userIDAny, exists := c.Get(userCtx)
	if !exists {
		return uuid.Nil, errors.New("user ID not found in context")
	}

	userID, ok := userIDAny.(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("user ID in context is of invalid type")
	}

	return userID, nil
}
