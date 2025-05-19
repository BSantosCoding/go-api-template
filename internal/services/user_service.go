package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"time"

	"go-api-template/internal/api"
	"go-api-template/internal/storage"
	"go-api-template/internal/transport/dto"

	"github.com/go-redis/redis"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func (sd *ServerDefinition) PostAuthRegister(ctx context.Context, request api.PostAuthRegisterRequestObject) (api.PostAuthRegisterResponseObject, error) {
	user, err := sd.usersRepo.Create(ctx, &dto.CreateUserRequest{Email: request.Body.Email, Password: request.Body.Password, Name: *request.Body.Name})
	if err != nil {
		if errors.Is(err, storage.ErrDuplicateEmail) || errors.Is(err, storage.ErrConflict) {
			log.Printf("Registration conflict: %v", err)
			return nil, fmt.Errorf("%w: %w", ErrConflict, err)
		}
		log.Printf("UserService.PostAuthRegister: Error creating user: %v", err)
		return nil, fmt.Errorf("internal error creating user: %w", err)
	}

	mappedUser := MapEntUserToResponse(user)

	return api.PostAuthRegister201JSONResponse(mappedUser), nil
}

func (sd *ServerDefinition) PostAuthLogin(ctx context.Context, request api.PostAuthLoginRequestObject) (api.PostAuthLoginResponseObject, error) {
	user, err := sd.usersRepo.GetByEmail(ctx, &dto.GetUserByEmailRequest{Email: request.Body.Email})
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			log.Printf("Login attempt failed for email %s: user not found", request.Body.Email)
			return nil, ErrInvalidCredentials // Use specific service error
		}
		log.Printf("Error fetching user by email %s during login: %v", request.Body.Email, err)
		return nil, fmt.Errorf("internal error during login: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(request.Body.Password))
	if err != nil {
		log.Printf("Login attempt failed for email %s: invalid password", request.Body.Email)
		return nil, ErrInvalidCredentials // Use specific service error
	}

	// Generate JWT Token
	expirationTime := time.Now().Add(sd.jwtExpiration)
	claims := &jwt.RegisteredClaims{
		Subject:   user.ID.String(),
		ExpiresAt: jwt.NewNumericDate(expirationTime),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	// Generate Access Token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(sd.jwtSecret))
	if err != nil {
		log.Printf("Error generating JWT token for user %s: %v", user.Email, err)
		return nil, fmt.Errorf("failed to generate login token: %w", err)
	}

	// Generate and Store Refresh Token
	refreshToken, err := sd.generateAndStoreRefreshToken(ctx, user.ID)
	if err != nil {
		log.Printf("Error generating/storing refresh token for user %s: %v", user.Email, err)
		return nil, fmt.Errorf("failed to handle refresh token: %w", err)
	}

	mappedUser := MapEntUserToResponse(user)

	response := api.DtoLoginResponse{
		User:         &mappedUser,
		AccessToken:  &tokenString,
		RefreshToken: &refreshToken,
	}

	return api.PostAuthLogin200JSONResponse(response), nil
}

// Refresh generates a new access token and potentially a new refresh token using a valid refresh token.
func (sd *ServerDefinition) PostAuthRefresh(ctx context.Context, request api.PostAuthRefreshRequestObject) (api.PostAuthRefreshResponseObject, error) {
	userIDStr, err := sd.redisClient.Get(ctx, sd.redisRefreshTokenPrefix+request.Body.RefreshToken).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			log.Printf("Refresh token not found or expired: %s", request.Body)
			return nil, ErrInvalidCredentials // Treat as invalid credentials/token
		}
		log.Printf("Error retrieving refresh token from Redis: %v", err)
		return nil, fmt.Errorf("internal error validating refresh token: %w", err)
	}

	// Invalidate the used refresh token (Token Rotation)
	if err := sd.redisClient.Del(ctx, sd.redisRefreshTokenPrefix+request.Body.RefreshToken).Err(); err != nil {
		// Log the error but proceed, as the main goal is issuing new tokens
		log.Printf("WARN: Failed to delete used refresh token %s from Redis: %v", request.Body.RefreshToken, err)
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		log.Printf("Error parsing userID '%s' from Redis for refresh token %s: %v", userIDStr, request.Body.RefreshToken, err)
		return nil, fmt.Errorf("internal error processing refresh token data: %w", err)
	}

	// Generate new Access Token
	newAccessToken, err := sd.generateAccessToken(userID)
	if err != nil {
		log.Printf("Error generating new access token during refresh for user %s: %v", userID, err)
		return nil, fmt.Errorf("failed to generate new access token: %w", err)
	}

	// Generate and Store new Refresh Token
	newRefreshToken, err := sd.generateAndStoreRefreshToken(ctx, userID)
	if err != nil {
		log.Printf("Error generating/storing new refresh token during refresh for user %s: %v", userID, err)
		return nil, fmt.Errorf("failed to handle new refresh token: %w", err)
	}

	response := api.DtoRefreshResponse{
		AccessToken:  &newAccessToken,
		RefreshToken: &newRefreshToken,
	}

	return api.PostAuthRefresh200JSONResponse(response), nil
}

// Logout invalidates a specific refresh token.
func (sd *ServerDefinition) PostAuthLogout(ctx context.Context, request api.PostAuthLogoutRequestObject) (api.PostAuthLogoutResponseObject, error) {
	err := sd.redisClient.Del(ctx, sd.redisRefreshTokenPrefix+request.Body.RefreshToken).Err()
	if err != nil && !errors.Is(err, redis.Nil) { // Ignore if token already not found
		log.Printf("Error deleting refresh token %s from Redis during logout: %v", request.Body.RefreshToken, err)
		return nil, fmt.Errorf("failed to invalidate session: %w", err)
	}
	log.Printf("Successfully invalidated refresh token: %s", request.Body.RefreshToken)
	return api.PostAuthLogout204Response{}, nil
}

func (sd *ServerDefinition) GetUsers(ctx context.Context, request api.GetUsersRequestObject) (api.GetUsersResponseObject, error) {
	users, err := sd.usersRepo.GetAll(ctx)
	if err != nil {
		return nil, MapRepoError(err, "getting users")
	}
	mappedResponse := make([]api.DtoUserResponse, len(users))
	for i, user := range users {
		mappedResponse[i] = MapEntUserToResponse(user)
	}
	return api.GetUsers200JSONResponse(mappedResponse), nil
}

func (sd *ServerDefinition) GetUsersId(ctx context.Context, request api.GetUsersIdRequestObject) (api.GetUsersIdResponseObject, error) {
	user, err := sd.usersRepo.GetByID(ctx, &dto.GetUserByIdRequest{ID: request.Id})
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	mappedResponse := MapEntUserToResponse(user)

	return api.GetUsersId200JSONResponse(mappedResponse), err
}

func (sd *ServerDefinition) PutUsersId(ctx context.Context, request api.PutUsersIdRequestObject) (api.PutUsersIdResponseObject, error) {
	// --- Transaction Start ---
	tx, err := sd.db.Tx(ctx)
	if err != nil {
		log.Printf("UserService.Update: Error beginning transaction: %v", err)
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if anything fails

	// Use transaction-aware usersRepository
	txUserusersRepo := sd.usersRepo.WithTx(tx)
	// --- End Transaction Setup ---

	updatedUser, err := txUserusersRepo.Update(ctx, &dto.UpdateUserRequest{Name: request.Body.Name, ID: request.Id}) // Use txUserusersRepo
	if err != nil {
		return nil, MapRepoError(err, "updating user")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(); err != nil {
		log.Printf("UserService.Update: Error committing transaction: %v", err)
		return nil, fmt.Errorf("internal error committing user update: %w", err)
	}
	// --- End Transaction ---

	mappedResponse := MapEntUserToResponse(updatedUser)

	return api.PutUsersId200JSONResponse(mappedResponse), nil
}

func (sd *ServerDefinition) DeleteUsersId(ctx context.Context, request api.DeleteUsersIdRequestObject) (api.DeleteUsersIdResponseObject, error) {
	err := sd.usersRepo.Delete(ctx, &dto.DeleteUserRequest{ID: request.Id})
	return api.DeleteUsersId204Response{}, err
}

// generateAccessToken creates a new JWT access token for the given user ID.
func (sd *ServerDefinition) generateAccessToken(userID uuid.UUID) (string, error) {
	expirationTime := time.Now().Add(sd.jwtExpiration)
	claims := &jwt.RegisteredClaims{
		Subject:   userID.String(),
		ExpiresAt: jwt.NewNumericDate(expirationTime),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(sd.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign access token: %w", err)
	}
	return tokenString, nil
}

// generateAndStoreRefreshToken creates a secure random refresh token and stores it in redis.
func (sd *ServerDefinition) generateAndStoreRefreshToken(ctx context.Context, userID uuid.UUID) (string, error) {
	rb := make([]byte, sd.refreshTokenBytes)
	if _, err := rand.Read(rb); err != nil {
		return "", fmt.Errorf("failed to generate random bytes for refresh token: %w", err)
	}
	refreshToken := base64.URLEncoding.EncodeToString(rb)

	// Store in Redis: Key = "refresh_token:<token>", Value = userID
	err := sd.redisClient.Set(ctx, sd.redisRefreshTokenPrefix+refreshToken, userID.String(), sd.refreshTokenExpiration).Err()
	if err != nil {
		return "", fmt.Errorf("failed to store refresh token in Redis: %w", err)
	}

	return refreshToken, nil
}
