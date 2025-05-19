package services

import (
	"context"
	"go-api-template/ent"
	"go-api-template/internal/api"
	"go-api-template/internal/storage"
	"go-api-template/internal/storage/postgres"
	"time"

	"github.com/redis/go-redis/v9"
)

type ServerDefinition struct {
	invoiceRepo             storage.InvoiceRepository
	jobRepo                 storage.JobRepository
	usersRepo               storage.UserRepository
	jobApplicationRepo      storage.JobApplicationRepository
	db                      *ent.Client
	redisClient             *redis.Client
	jwtSecret               string
	jwtExpiration           time.Duration
	refreshTokenExpiration  time.Duration
	redisRefreshTokenPrefix string
	refreshTokenBytes       int
}

func NewServerDefinition(db *ent.Client, redisClient *redis.Client, jwtSecret string, jwtExpiration, refreshTokenExpiration time.Duration) *ServerDefinition {
	return &ServerDefinition{
		invoiceRepo:             postgres.NewInvoiceRepo(db),
		jobRepo:                 postgres.NewJobRepo(db),
		usersRepo:               postgres.NewUserRepo(db),
		jobApplicationRepo:      postgres.NewJobApplicationRepo(db),
		db:                      db,
		redisClient:             redisClient,
		jwtSecret:               jwtSecret,
		jwtExpiration:           jwtExpiration,
		refreshTokenExpiration:  refreshTokenExpiration,
		redisRefreshTokenPrefix: "refresh_token:",
		refreshTokenBytes:       32,
	}
}

var _ api.StrictServerInterface = (api.StrictServerInterface)(nil)

func (s *ServerDefinition) GetHealth(ctx context.Context, request api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	return api.GetHealth200JSONResponse{"Status": "OK"}, nil
}
