package services

import (
	"context"
	"errors"
	"fmt"
	"go-api-template/internal/models"
	"go-api-template/internal/storage"
	"go-api-template/internal/storage/postgres"
	"go-api-template/internal/transport/dto"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type invoiceService struct {
	invoiceRepo storage.InvoiceRepository
	jobRepo storage.JobRepository
	db          *pgxpool.Pool
}

func NewInvoiceService(db *pgxpool.Pool) InvoiceService {
	return &invoiceService{
		invoiceRepo: postgres.NewInvoiceRepo(db),
		jobRepo:     postgres.NewJobRepo(db),
		db:          db,
	}
}

func (s *invoiceService) CreateInvoice(ctx context.Context, req *dto.CreateInvoiceRequest) (*models.Invoice, error) {
	jobReq := dto.GetJobByIDRequest{ID: req.JobID}
	job, err := s.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		log.Printf("CreateInvoice: Error fetching job %s: %v", req.JobID, err)
		return nil, mapRepoError(err, "fetching job for invoice creation")
	}

	// Authorization & State checks
	if job.ContractorID == nil || *job.ContractorID != req.UserId {
		log.Printf("CreateInvoice: Forbidden attempt by user %s on job %s (Contractor: %v)", req.UserId, req.JobID, job.ContractorID)
		return nil, ErrForbidden
	}
	if job.State != models.JobStateOngoing {
		log.Printf("CreateInvoice: Attempt to create invoice for job %s in state %s", req.JobID, job.State)
		return nil, ErrInvalidState // Correct error type
	}

	// --- Transaction Start ---
	tx, err := s.db.Begin(ctx)
	if err != nil {
		log.Printf("CreateInvoice: Error beginning transaction: %v", err)
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback if anything fails

	txInvoiceRepo := s.invoiceRepo.WithTx(tx)
	intervalReq := &dto.GetMaxIntervalForJobRequest{JobID: req.JobID}
	maxIntervalNum, err := txInvoiceRepo.GetMaxIntervalForJob(ctx, intervalReq) // Use txInvoiceRepo
	if err != nil {
		return nil, mapRepoError(err, "getting max interval for job")
	}
	nextIntervalNumber := maxIntervalNum + 1

	if job.InvoiceInterval <= 0 {
		return nil, ErrInvalidInvoiceInterval
	}

	maxPossibleIntervals := job.Duration / job.InvoiceInterval
	remainderHours := job.Duration % job.InvoiceInterval
	isPartialLastInterval := remainderHours != 0
	if isPartialLastInterval {
		maxPossibleIntervals++
	}

	if nextIntervalNumber > maxPossibleIntervals {
		return nil, ErrInvalidInvoiceInterval
	}

	// Determine hours for this specific invoice (in case of a partial last interval)
	var hoursForThisInterval int
	isLastInterval := (nextIntervalNumber == maxPossibleIntervals)

	if isLastInterval && isPartialLastInterval {
		hoursForThisInterval = remainderHours
	} else {
		// It's either not the last interval, or the last interval is a full one
		hoursForThisInterval = job.InvoiceInterval
	}

	baseValue := job.Rate * float64(hoursForThisInterval) // Use calculated hours
	finalValue := baseValue
	if req.Adjustment != nil {
		finalValue += *req.Adjustment
	}
	if finalValue < 0 { // Ensure non-negative value
		finalValue = 0
	}

	invoiceToCreate := &models.Invoice{
		JobID:          req.JobID,
		IntervalNumber: nextIntervalNumber,
		Value:          finalValue,
		State:          models.InvoiceStateWaiting,
		ID:			 uuid.New(), // Generate a new UUID for the invoice
	}

	invoice, err := txInvoiceRepo.Create(ctx, invoiceToCreate) // Use txInvoiceRepo
	if err != nil {
		if errors.Is(err, storage.ErrConflict) {
			return nil, ErrConflict
		}
		log.Printf("CreateInvoice: Error saving invoice in repo: %v", err)
		return nil, fmt.Errorf("internal error saving invoice: %w", err)
	}

	// --- Commit Transaction ---
	if err := tx.Commit(ctx); err != nil {
		log.Printf("CreateInvoice: Error committing transaction: %v", err)
		return nil, mapRepoError(err, "committing invoice creation")
	}
	// --- End Transaction ---
	return invoice, nil
}

func (s *invoiceService) GetInvoiceByID(ctx context.Context, req *dto.GetInvoiceByIDRequest) (*models.Invoice, error) {
	// Call s.invoiceRepo.GetByID
	invoice, err := s.invoiceRepo.GetByID(ctx, req)
	if err != nil {
		return nil, mapRepoError(err, "getting invoice")
	}

	// Fetch associated Job using s.jobRepo.GetByID(invoice.JobID) for auth check
	jobReq := dto.GetJobByIDRequest{ID: invoice.JobID}
	job, err := s.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		return nil, mapRepoError(err, "getting job")
	}

	// Authorization Check: Verify UserID matches job.EmployerID or job.ContractorID.
	isEmployer := job.EmployerID == req.UserId
	isContractor := job.ContractorID != nil && *job.ContractorID == req.UserId
	if !(isEmployer || isContractor) {
		return nil, ErrForbidden
	}

	return invoice, nil
}

func (s *invoiceService) UpdateInvoiceState(ctx context.Context, req *dto.UpdateInvoiceStateRequest) (*models.Invoice, error) {
	// --- Transaction Start ---
	tx, err := s.db.Begin(ctx)
	if err != nil {
		log.Printf("UpdateInvoiceState: Error beginning transaction: %v", err)
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback if anything fails

	txInvoiceRepo := s.invoiceRepo.WithTx(tx)
	txJobRepo := s.jobRepo.WithTx(tx)
	// --- End Transaction Setup ---

	// Fetch Invoice
	getReq := dto.GetInvoiceByIDRequest{ID: req.ID}
	invoice, err := txInvoiceRepo.GetByID(ctx, &getReq) // Use txInvoiceRepo
	if err != nil {
		return nil, mapRepoError(err, "getting invoice")
	}

	// Fetch Job for Auth Check
	jobReq := dto.GetJobByIDRequest{ID: invoice.JobID}
	job, err := txJobRepo.GetByID(ctx, &jobReq) // Use txJobRepo
	if err != nil {
		return nil, mapRepoError(err, "getting job")
	}

	// --- Authorization Check: ONLY Employer ---
	isEmployer := job.EmployerID == req.UserId
	if !isEmployer {
		log.Printf("UpdateInvoiceState: Forbidden attempt by user %s on invoice %s (Job Employer: %v)", req.UserId, req.ID, job.EmployerID)
		return nil, ErrForbidden
	}
	// --- End Auth Check ---

	// Check State Transition
	if !isValidInvoiceStateTransition(invoice.State, req.NewState) {
		return nil, ErrInvalidTransition
	}

	updatedInvoice, err := txInvoiceRepo.UpdateState(ctx, req) // Use txInvoiceRepo
	if err != nil {
		return nil, mapRepoError(err, "updating invoice state")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(ctx); err != nil {
		log.Printf("UpdateInvoiceState: Error committing transaction: %v", err)
		return nil, fmt.Errorf("internal error committing invoice update: %w", err)
	}
	// --- End Transaction ---

	// TODO: Consider if updating the Job state to Complete should happen here
	// if all invoices are Complete and the last interval is reached.
	// This would require another transaction or careful coordination.
	return updatedInvoice, nil
}

func (s *invoiceService) DeleteInvoice(ctx context.Context, req *dto.DeleteInvoiceRequest) error {
	// Fetch Invoice
	getReq := dto.GetInvoiceByIDRequest{ID: req.ID}
	invoice, err := s.invoiceRepo.GetByID(ctx, &getReq)
	// Note: No transaction needed here yet, just checking existence and state first.
	if err != nil {
		return mapRepoError(err, "getting invoice for deletion check")
	}

	// Fetch Job for Auth Check
	jobReq := dto.GetJobByIDRequest{ID: invoice.JobID}
	job, err := s.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		return mapRepoError(err, "getting job for deletion check")
	}

	// --- Transaction Start (only needed for the delete itself, but good practice) ---
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("internal error starting transaction: %w", err)
	}

	// --- Authorization Check: ONLY Contractor + State Waiting ---
	isContractor := job.ContractorID != nil && *job.ContractorID == req.UserId
	if !isContractor {
		return ErrForbidden
	}
	if invoice.State != models.InvoiceStateWaiting {
		return ErrInvalidState
	}
	// --- End Auth Check ---

	txInvoiceRepo := s.invoiceRepo.WithTx(tx)

	// Call Repo Delete
	err = txInvoiceRepo.Delete(ctx, req) // Use txInvoiceRepo
	if err != nil {
		tx.Rollback(ctx) // Rollback on error
		return mapRepoError(err, "deleting invoice")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(ctx); err != nil {
		log.Printf("DeleteInvoice: Error committing transaction: %v", err)
		return fmt.Errorf("internal error committing invoice deletion: %w", err)
	}

	return nil
}

func (s *invoiceService) ListInvoicesByJob(ctx context.Context, req *dto.ListInvoicesByJobRequest) ([]models.Invoice, error) {
	// Fetch Job using s.jobRepo.GetByID(JobID) to verify existence and for auth check.
	jobReq := dto.GetJobByIDRequest{ID: req.JobID}
	job, err := s.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		return nil, mapRepoError(err, "getting job for listing invoices")
	}

	// Authorization Check: Verify UserID matches job.EmployerID or job.ContractorID.
	isEmployer := job.EmployerID == req.UserId
	isContractor := job.ContractorID != nil && *job.ContractorID == req.UserId
	if !(isEmployer || isContractor) {
		return nil, ErrForbidden
	}
	
	// Call s.invoiceRepo.ListByJob
	invoices, err := s.invoiceRepo.ListByJob(ctx, req)
	if err != nil {
		return nil, mapRepoError(err, "listing invoices")
	}

	return invoices, nil
}