package integration_tests

import (
	"context"
	"database/sql"
	"go-api-template/ent"
	"go-api-template/ent/job"
	"go-api-template/internal/storage/postgres"
	"go-api-template/internal/transport/dto"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"entgo.io/ent/dialect"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// Helper to create a pointer to a float64
func ptrFloat64(f float64) *float64 { return &f }

// Helper to create a pointer to an int
func ptrInt(i int) *int { return &i }

// Helper function to create a user for tests
func createTestUser(t *testing.T, ctx context.Context, pool *ent.Client, email, name string) *ent.User {
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
func createTestJob(t *testing.T, ctx context.Context, pool *ent.Client, employerID uuid.UUID, state job.State, contractorID *uuid.UUID) *ent.Job {
	t.Helper()
	jobRepo := postgres.NewJobRepo(pool)
	jobReq := &dto.CreateJobRequest{
		Rate:            50.0,
		Duration:        20,
		InvoiceInterval: 10,
		EmployerID:      employerID,
	}
	resultingJob, err := jobRepo.Create(ctx, jobReq)
	require.NoError(t, err, "Failed to create test job for employer %s", employerID)
	require.NotNil(t, resultingJob)

	// Update state/contractor if needed for the test scenario
	if state != job.StateWaiting || contractorID != nil {
		updateReq := dto.UpdateJobRequest{ID: resultingJob.ID}
		if state != job.StateWaiting {
			updateReq.State = &state
		}
		if contractorID != nil {
			updateReq.ContractorID = contractorID
		}
		updatedJob, updateErr := jobRepo.Update(ctx, &updateReq)
		require.NoError(t, updateErr, "Failed to update test job state/contractor")
		return updatedJob
	}
	return resultingJob
}

var testDB *ent.Client
var testRedisClient *redis.Client

// getTestClients establishes a connection pool to the test database.
// It reads the DSN from the TEST_DATABASE_URL environment variable.
func getTestClients(t *testing.T) (*ent.Client, *redis.Client) {
	// Use t.Helper() to mark this as a test helper function
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL environment variable not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, nil
	}

	entDriver := entsql.OpenDB(dialect.Postgres, db)
	entClient := ent.NewClient(ent.Driver(entDriver))

	testDB = entClient

	// Run migrations before creating the pool to ensure schema exists
	runMigrations(t)

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
	return testDB, testRedisClient
}

// runMigrations runs database migrations up.
func runMigrations(t *testing.T) {
	t.Helper()

	ctx := context.Background()

	err := testDB.Schema.Create(ctx)
	require.NoError(t, err)
	log.Println("Ent client connected and schema created/checked.")
}

// cleanupTables truncates specified tables for test isolation.
func cleanupTables(ctx context.Context, t *testing.T, pool *ent.Client, tables ...string) {
	t.Helper()
	if len(tables) == 0 {
		return // Nothing to clean
	}

	for _, table := range tables {
		switch table {
		case "users":
			_, err := pool.User.Delete().Exec(ctx)
			require.NoError(t, err, "Failed to truncate users table")
		case "jobs":
			_, err := pool.Job.Delete().Exec(ctx)
			require.NoError(t, err, "Failed to truncate jobs table")
		case "invoices":
			_, err := pool.Invoice.Delete().Exec(ctx)
			require.NoError(t, err, "Failed to truncate invoices table")
		case "job_application":
			_, err := pool.JobApplication.Delete().Exec(ctx)
			require.NoError(t, err, "Failed to truncate job_application table")
		default:
		}
	}
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
