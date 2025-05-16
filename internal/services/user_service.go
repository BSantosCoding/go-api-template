package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"time"

	"go-api-template/ent"
	"go-api-template/internal/storage"
	"go-api-template/internal/storage/postgres"
	"go-api-template/internal/transport/dto"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const (
	RefreshTokenBytes       = 32
	RedisRefreshTokenPrefix = "refresh_token:"
)

type userService struct {
	repo                   storage.UserRepository
	redisClient            *redis.Client
	jwtSecret              string
	jwtExpiration          time.Duration
	refreshTokenExpiration time.Duration
	db                     *ent.Client
}

// NewUserService creates a new instance of UserService.
func NewUserService(redisClient *redis.Client, jwtSecret string, jwtExpiration, refreshTokenExpiration time.Duration, db *ent.Client) UserService {
	return &userService{
		repo:                   postgres.NewUserRepo(db),
		redisClient:            redisClient,
		jwtSecret:              jwtSecret,
		jwtExpiration:          jwtExpiration,
		refreshTokenExpiration: refreshTokenExpiration,
		db:                     db,
	}
}

func (s *userService) Register(ctx context.Context, req *dto.CreateUserRequest) (*ent.User, error) {
	user, err := s.repo.Create(ctx, req)
	if err != nil {
		if errors.Is(err, storage.ErrDuplicateEmail) || errors.Is(err, storage.ErrConflict) {
			return nil, fmt.Errorf("%w: %w", ErrConflict, err)
		}
		log.Printf("UserService: Error creating user: %v", err)
		return nil, fmt.Errorf("internal error creating user: %w", err)
	}
	return user, nil
}

func (s *userService) Login(ctx context.Context, req *dto.LoginRequest) (*ent.User, string, string, error) {
	emailReq := dto.GetUserByEmailRequest{Email: req.Email}
	user, err := s.repo.GetByEmail(ctx, &emailReq)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			log.Printf("Login attempt failed for email %s: user not found", req.Email)
			return nil, "", "", ErrInvalidCredentials // Use specific service error
		}
		log.Printf("Error fetching user by email %s during login: %v", req.Email, err)
		return nil, "", "", fmt.Errorf("internal error during login: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		log.Printf("Login attempt failed for email %s: invalid password", req.Email)
		return nil, "", "", ErrInvalidCredentials // Use specific service error
	}

	// Generate JWT Token
	expirationTime := time.Now().Add(s.jwtExpiration)
	claims := &jwt.RegisteredClaims{
		Subject:   user.ID.String(),
		ExpiresAt: jwt.NewNumericDate(expirationTime),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	// Generate Access Token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		log.Printf("Error generating JWT token for user %s: %v", user.Email, err)
		return nil, "", "", fmt.Errorf("failed to generate login token: %w", err)
	}

	// Generate and Store Refresh Token
	refreshToken, err := s.generateAndStoreRefreshToken(ctx, user.ID)
	if err != nil {
		log.Printf("Error generating/storing refresh token for user %s: %v", user.Email, err)
		return nil, "", "", fmt.Errorf("failed to handle refresh token: %w", err)
	}

	return user, tokenString, refreshToken, nil
}

// Refresh generates a new access token and potentially a new refresh token using a valid refresh token.
func (s *userService) Refresh(ctx context.Context, req *dto.RefreshRequest) (string, string, error) {
	userIDStr, err := s.redisClient.Get(ctx, RedisRefreshTokenPrefix+req.RefreshToken).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			log.Printf("Refresh token not found or expired: %s", req)
			return "", "", ErrInvalidCredentials // Treat as invalid credentials/token
		}
		log.Printf("Error retrieving refresh token from Redis: %v", err)
		return "", "", fmt.Errorf("internal error validating refresh token: %w", err)
	}

	// Invalidate the used refresh token (Token Rotation)
	if err := s.redisClient.Del(ctx, RedisRefreshTokenPrefix+req.RefreshToken).Err(); err != nil {
		// Log the error but proceed, as the main goal is issuing new tokens
		log.Printf("WARN: Failed to delete used refresh token %s from Redis: %v", req.RefreshToken, err)
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		log.Printf("Error parsing userID '%s' from Redis for refresh token %s: %v", userIDStr, req.RefreshToken, err)
		return "", "", fmt.Errorf("internal error processing refresh token data: %w", err)
	}

	// Generate new Access Token
	newAccessToken, err := s.generateAccessToken(userID)
	if err != nil {
		log.Printf("Error generating new access token during refresh for user %s: %v", userID, err)
		return "", "", fmt.Errorf("failed to generate new access token: %w", err)
	}

	// Generate and Store new Refresh Token
	newRefreshToken, err := s.generateAndStoreRefreshToken(ctx, userID)
	if err != nil {
		log.Printf("Error generating/storing new refresh token during refresh for user %s: %v", userID, err)
		return "", "", fmt.Errorf("failed to handle new refresh token: %w", err)
	}

	return newAccessToken, newRefreshToken, nil
}

// Logout invalidates a specific refresh token.
func (s *userService) Logout(ctx context.Context, req *dto.LogoutRequest) error {
	err := s.redisClient.Del(ctx, RedisRefreshTokenPrefix+req.RefreshToken).Err()
	if err != nil && !errors.Is(err, redis.Nil) { // Ignore if token already not found
		log.Printf("Error deleting refresh token %s from Redis during logout: %v", req.RefreshToken, err)
		return fmt.Errorf("failed to invalidate session: %w", err)
	}
	log.Printf("Successfully invalidated refresh token: %s", req.RefreshToken)
	return nil
}

func (s *userService) GetAll(ctx context.Context) ([]*ent.User, error) {
	return s.repo.GetAll(ctx)
}

func (s *userService) GetByID(ctx context.Context, req *dto.GetUserByIdRequest) (*ent.User, error) {
	user, err := s.repo.GetByID(ctx, req)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return user, err
}

func (s *userService) GetByEmail(ctx context.Context, req *dto.GetUserByEmailRequest) (*ent.User, error) {
	user, err := s.repo.GetByEmail(ctx, req)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return user, err
}

func (s *userService) Update(ctx context.Context, req *dto.UpdateUserRequest) (*ent.User, error) {
	// --- Transaction Start ---
	tx, err := s.db.Tx(ctx)
	if err != nil {
		log.Printf("UserService.Update: Error beginning transaction: %v", err)
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if anything fails

	// Use transaction-aware repository
	txUserRepo := s.repo.WithTx(tx)
	// --- End Transaction Setup ---

	updatedUser, err := txUserRepo.Update(ctx, req) // Use txUserRepo
	if err != nil {
		return nil, mapRepoError(err, "updating user")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(); err != nil {
		log.Printf("UserService.Update: Error committing transaction: %v", err)
		return nil, fmt.Errorf("internal error committing user update: %w", err)
	}
	// --- End Transaction ---

	return updatedUser, nil
}

func (s *userService) Delete(ctx context.Context, req *dto.DeleteUserRequest) error {
	return s.repo.Delete(ctx, req)
}

// generateAccessToken creates a new JWT access token for the given user ID.
func (s *userService) generateAccessToken(userID uuid.UUID) (string, error) {
	expirationTime := time.Now().Add(s.jwtExpiration)
	claims := &jwt.RegisteredClaims{
		Subject:   userID.String(),
		ExpiresAt: jwt.NewNumericDate(expirationTime),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign access token: %w", err)
	}
	return tokenString, nil
}

// generateAndStoreRefreshToken creates a secure random refresh token and stores it in Redis.
func (s *userService) generateAndStoreRefreshToken(ctx context.Context, userID uuid.UUID) (string, error) {
	rb := make([]byte, RefreshTokenBytes)
	if _, err := rand.Read(rb); err != nil {
		return "", fmt.Errorf("failed to generate random bytes for refresh token: %w", err)
	}
	refreshToken := base64.URLEncoding.EncodeToString(rb)

	// Store in Redis: Key = "refresh_token:<token>", Value = UserID
	err := s.redisClient.Set(ctx, RedisRefreshTokenPrefix+refreshToken, userID.String(), s.refreshTokenExpiration).Err()
	if err != nil {
		return "", fmt.Errorf("failed to store refresh token in Redis: %w", err)
	}

	return refreshToken, nil
}
