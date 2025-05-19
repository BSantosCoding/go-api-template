package middleware

import (
	"context" // Import context
	"errors"
	"fmt"
	"log" // Import http for accessing request headers
	"strings"

	"github.com/getkin/kin-openapi/openapi3filter" // Import openapi3filter
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	authorizationHeader            = "Authorization"
	userCtx                        = "userID"
	userIDContextKey    contextKey = "userID"
)

type contextKey string // Custom type for context key to avoid collisions
func JWTAuthenticationFunc(jwtSecret string) openapi3filter.AuthenticationFunc {
	return func(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
		// The http.Request is available via input.Request
		req := input.RequestValidationInput
		authHeader := req.Request.Header.Get(authorizationHeader)
		if authHeader == "" {
			log.Println("Auth middleware (openapi3filter): Authorization header missing")
			// Return an authentication error
			return openapi3filter.ErrInvalidEmptyValue
		}

		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 || strings.ToLower(headerParts[0]) != "bearer" {
			log.Println("Auth middleware (openapi3filter): Invalid Authorization header format")
			return openapi3filter.ErrInvalidRequired
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
			log.Printf("Auth middleware (openapi3filter): Error parsing token: %v", err)
			// Return appropriate errors based on the JWT error
			if errors.Is(err, jwt.ErrTokenExpired) {
				// Indicate authentication failed specifically due to expiration
				return fmt.Errorf("%w: token expired", openapi3filter.ErrInvalidRequired)
			}
			// Wrap other errors in ErrAuthenticationFailed
			return fmt.Errorf("%w: %v", openapi3filter.ErrInvalidRequired, err)
		}

		if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
			// Token is valid, extract user ID (subject)
			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				log.Printf("Auth middleware (openapi3filter): Error parsing user ID from token subject '%s': %v", claims.Subject, err)
				return fmt.Errorf("%w: invalid user identifier in token", openapi3filter.ErrInvalidRequired)
			}

			req.Request.Clone(context.WithValue(ctx, userIDContextKey, userID))
			log.Printf("Auth middleware (openapi3filter): User %s authenticated", userID)

			// Return nil to indicate success
			return nil // Authentication successful
		} else {
			log.Println("Auth middleware (openapi3filter): Invalid token claims or token is not valid")
			return openapi3filter.ErrInvalidRequired
		}
	}
}

// Helper function to get user ID from Gin context after authentication.
// Assumes that JWTAuthenticationFunc and oapi-codegen's Gin middleware
// have successfully placed the userID into the Gin context using the
// userIDContextKey.
func GetUserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	// Retrieve using the context key defined for the standard context
	userIDAny, exists := c.Get(userCtx)
	if !exists {
		// This should ideally not happen if the middleware ran correctly
		return uuid.Nil, errors.New("user ID not found in Gin context after authentication")
	}

	userID, ok := userIDAny.(uuid.UUID)
	if !ok {
		// This should ideally not happen if the correct type was stored
		return uuid.Nil, errors.New("user ID in Gin context is of invalid type")
	}

	return userID, nil
}
