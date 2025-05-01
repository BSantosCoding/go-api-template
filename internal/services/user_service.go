package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"go-api-template/internal/models"
	"go-api-template/internal/storage"
	"go-api-template/internal/transport/dto"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

type userService struct {
	repo          storage.UserRepository
	jwtSecret     string
	jwtExpiration time.Duration
}

// NewUserService creates a new instance of UserService.
func NewUserService(repo storage.UserRepository, jwtSecret string, jwtExpiration time.Duration) UserService {
	return &userService{
		repo:          repo,
		jwtSecret:     jwtSecret,
		jwtExpiration: jwtExpiration,
	}
}

func (s *userService) Register(ctx context.Context, req *dto.CreateUserRequest) (*models.User, error) {
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

func (s *userService) Login(ctx context.Context, req *dto.LoginRequest) (*models.User, string, error) {
	emailReq := dto.GetUserByEmailRequest{Email: req.Email}
	user, err := s.repo.GetByEmail(ctx, &emailReq)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			log.Printf("Login attempt failed for email %s: user not found", req.Email)
			return nil, "", ErrInvalidCredentials // Use specific service error
		}
		log.Printf("Error fetching user by email %s during login: %v", req.Email, err)
		return nil, "", fmt.Errorf("internal error during login: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		log.Printf("Login attempt failed for email %s: invalid password", req.Email)
		return nil, "", ErrInvalidCredentials // Use specific service error
	}

	// Generate JWT Token
	expirationTime := time.Now().Add(s.jwtExpiration)
	claims := &jwt.RegisteredClaims{
		Subject:   user.ID.String(),
		ExpiresAt: jwt.NewNumericDate(expirationTime),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		log.Printf("Error generating JWT token for user %s: %v", user.Email, err)
		return nil, "", fmt.Errorf("failed to generate login token: %w", err)
	}

	return user, tokenString, nil
}

func (s *userService) GetAll(ctx context.Context) ([]models.User, error) {
	return s.repo.GetAll(ctx)
}

func (s *userService) GetByID(ctx context.Context, req *dto.GetUserByIdRequest) (*models.User, error) {
	user, err := s.repo.GetByID(ctx, req)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return user, err
}

func (s *userService) GetByEmail(ctx context.Context, req *dto.GetUserByEmailRequest) (*models.User, error) {
	user, err := s.repo.GetByEmail(ctx, req)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return user, err
}

func (s *userService) Update(ctx context.Context, req *dto.UpdateUserRequest) (*models.User, error) {
	//Authenticate the user
	
	return s.repo.Update(ctx, req)
}

func (s *userService) Delete(ctx context.Context, req *dto.DeleteUserRequest) error {
	return s.repo.Delete(ctx, req)
}