package integration_tests

import (
	"context"
	"errors"
	"fmt"
	"go-api-template/internal/models"
	"go-api-template/internal/storage/postgres"
	"go-api-template/internal/transport/dto"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // Driver for postgres
	_ "github.com/golang-migrate/migrate/v4/source/file"       // Driver for file source
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// Helper to create a pointer to a float64
func ptrFloat64(f float64) *float64 { return &f }

// Helper to create a pointer to an int
func ptrInt(i int) *int { return &i }

// Helper function to create a user for tests
func createTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, email, name string) *models.User {
	t.Helper()
	userRepo := postgres.NewUserRepo(pool)
	userReq := &dto.CreateUserRequest{
		Email:    email,
		Name:     name,
		Password: "password",
	}
	user, err := userRepo.Create(ctx, userReq)
	require.NoError(t, err, "Failed to create test user %s", email)
	require.NotNil(t, user)
	return user
}

// Helper function to create a job for tests 
func createTestJob(t *testing.T, ctx context.Context, pool *pgxpool.Pool, employerID uuid.UUID, state models.JobState, contractorID *uuid.UUID) *models.Job {
	t.Helper()
	jobRepo := postgres.NewJobRepo(pool)
	jobReq := &dto.CreateJobRequest{
		Rate:            50.0,
		Duration:        20,
		InvoiceInterval: 10,
		EmployerID:      employerID,
	}
	job, err := jobRepo.Create(ctx, jobReq)
	require.NoError(t, err, "Failed to create test job for employer %s", employerID)
	require.NotNil(t, job)

	// Update state/contractor if needed for the test scenario
	if state != models.JobStateWaiting || contractorID != nil {
		updateReq := dto.UpdateJobRequest{ID: job.ID}
		if state != models.JobStateWaiting {
			updateReq.State = &state
		}
		if contractorID != nil {
			updateReq.ContractorID = contractorID
		}
		updatedJob, updateErr := jobRepo.Update(ctx, &updateReq)
		require.NoError(t, updateErr, "Failed to update test job state/contractor")
		return updatedJob
	}
	return job
}

var testDBPool *pgxpool.Pool
var testRedisClient *redis.Client

// getTestClients establishes a connection pool to the test database.
// It reads the DSN from the TEST_DATABASE_URL environment variable.
func getTestClients(t *testing.T) (*pgxpool.Pool, *redis.Client) {
	// Use t.Helper() to mark this as a test helper function
	t.Helper()

	if testDBPool != nil {
		// Ping existing pool to ensure it's still valid
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := testDBPool.Ping(ctx); err != nil {
			log.Println("Existing test DB pool connection is invalid, creating a new one.")
			testDBPool.Close() // Close the invalid pool
			testDBPool = nil
		}
	}

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL environment variable not set")
	}

	// Run migrations before creating the pool to ensure schema exists
	runMigrations(t, dsn)

	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err, "Failed to parse test database DSN")

	// Optional: Configure pool settings (e.g., max connections)
	// config.MaxConns = 5

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err, "Failed to connect to test database")

	// Ping the database to ensure connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = pool.Ping(ctx)
	require.NoError(t, err, "Failed to ping test database")

	log.Println("Successfully connected to test database.")
	testDBPool = pool // Store the pool globally for reuse within the test run

	// --- Redis Setup ---
	if testRedisClient == nil {
		redisAddr := os.Getenv("TEST_REDIS_URL")
		if redisAddr == "" {
			log.Println("WARN: TEST_REDIS_URL not set. Redis-dependent tests may fail or be skipped.")
			// Keep testRedisClient as nil
		} else {
			rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
			ctxRedis, cancelRedis := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancelRedis()
			if err := rdb.Ping(ctxRedis).Err(); err != nil {
				log.Printf("WARN: Failed to connect to test Redis at %s: %v. Redis-dependent tests may fail.", redisAddr, err)
				// Keep testRedisClient as nil
			} else {
				log.Println("Successfully connected to test Redis.")
				testRedisClient = rdb
			}
		}
	} 
	return testDBPool, testRedisClient
}

// runMigrations runs database migrations up.
func runMigrations(t *testing.T, dsn string) {
	t.Helper()
	migrationsPath := "file://../../database/migrations"

	m, err := migrate.New(migrationsPath, dsn)
	// Handle potential "file does not exist" error more gracefully if path is wrong
	if err != nil {
		// Check if it's a path error
		if os.IsNotExist(err) {
			t.Fatalf("Migrations path error: %v. Check if path '%s' is correct relative to the test execution directory.", err, migrationsPath)
		}
		require.NoError(t, err, "Failed to create migrate instance")
	}

	err = m.Up()
	// migrate.ErrNoChange is not an actual error, just indicates no migrations were run
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		// If there's a different error (like dirty database), fail the test
		require.NoError(t, err, "Failed to run migrations up")
	}
	log.Println("Migrations applied successfully (or no change).")
}

// cleanupTables truncates specified tables for test isolation.
func cleanupTables(t *testing.T, pool *pgxpool.Pool, tables ...string) {
	t.Helper()
	if len(tables) == 0 {
		return // Nothing to clean
	}
	// Use TRUNCATE ... RESTART IDENTITY CASCADE for thorough cleanup
	// Ensure table names are properly quoted if they contain special characters or are case-sensitive
	// For simplicity, assuming standard table names here.
	query := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", strings.Join(tables, ", "))
	_, err := pool.Exec(context.Background(), query)
	require.NoError(t, err, "Failed to truncate tables: %s", strings.Join(tables, ", "))
	log.Printf("Cleaned tables: %s", strings.Join(tables, ", "))
}

// cleanupRedis flushes the test Redis database. Use with caution!
func cleanupRedis(t *testing.T, client *redis.Client) {
	t.Helper()
	if client == nil {
		return // No client to clean
	}
	err := client.FlushDB(context.Background()).Err()
	require.NoError(t, err, "Failed to flush test Redis database")
	log.Println("Cleaned test Redis database (FLUSHDB).")
}