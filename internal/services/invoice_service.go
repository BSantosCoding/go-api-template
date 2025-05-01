package services

import (
	"context"
	"errors"
	"fmt"
	"go-api-template/internal/models"
	"go-api-template/internal/storage"
	"go-api-template/internal/transport/dto"

	"github.com/google/uuid"
)

type invoiceService struct {
	invoiceRepo storage.InvoiceRepository
	jobRepo storage.JobRepository
}

func NewInvoiceService(invoiceRepo storage.InvoiceRepository, jobRepo storage.JobRepository) InvoiceService {
	return &invoiceService{invoiceRepo: invoiceRepo, jobRepo: jobRepo}
}

func (s *invoiceService) CreateInvoice(ctx context.Context, req *dto.CreateInvoiceRequest) (*models.Invoice, error) {
	jobReq := dto.GetJobByIDRequest{ID: req.JobID}
	job, err := s.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrNotFound
		} else {
			return nil, fmt.Errorf("internal error creating job: %w", err)
		}
	}

	if job.ContractorID == nil || *job.ContractorID != req.UserId {
		return nil, ErrForbidden
	}
	if job.State != models.JobStateOngoing {
		return nil, ErrInvalidState
	}

	intervalReq := &dto.GetMaxIntervalForJobRequest{JobID: req.JobID}
	maxIntervalNum, err := s.invoiceRepo.GetMaxIntervalForJob(ctx, intervalReq)
	if err != nil {
		return nil, fmt.Errorf("internal error creating job: %w", err)
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

	invoice, err := s.invoiceRepo.Create(ctx, invoiceToCreate)
	if err != nil {
		if errors.Is(err, storage.ErrConflict) {
			return nil, ErrConflict
		} else {
			return nil, fmt.Errorf("internal error creating job: %w", err)
		}
	}

	return invoice, nil
}

func (s *invoiceService) GetInvoiceByID(ctx context.Context, req *dto.GetInvoiceByIDRequest) (*models.Invoice, error) {
	// Call s.invoiceRepo.GetByID
	invoice, err := s.invoiceRepo.GetByID(ctx, req)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrNotFound
		} else {
			return nil, fmt.Errorf("internal error getting invoice: %w", err)
		}
	}

	// Fetch associated Job using s.jobRepo.GetByID(invoice.JobID) for auth check
	jobReq := dto.GetJobByIDRequest{ID: invoice.JobID}
	job, err := s.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		return nil, fmt.Errorf("internal error getting job: %w", err)
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
	// Fetch Invoice
	getReq := dto.GetInvoiceByIDRequest{ID: req.ID}
	invoice, err := s.invoiceRepo.GetByID(ctx, &getReq)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrNotFound
		} else {
			return nil, fmt.Errorf("internal error getting invoice: %w", err)
		}
	}

	// Fetch Job for Auth Check
	jobReq := dto.GetJobByIDRequest{ID: invoice.JobID}
	job, err := s.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		return nil, fmt.Errorf("internal error getting job: %w", err)
	}

	// --- Authorization Check: ONLY Contractor ---
	isContractor := job.ContractorID != nil && *job.ContractorID == req.UserId
	if !isContractor {
		return nil, ErrForbidden
	}
	// --- End Auth Check ---

	// Check State Transition
	if !isValidInvoiceStateTransition(invoice.State, req.NewState) {
		return nil, ErrInvalidTransition
	}

	updatedInvoice, err := s.invoiceRepo.UpdateState(ctx, req)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrNotFound
		} else {
			return nil, fmt.Errorf("internal error updating invoice: %w", err)
		}
	}

	return updatedInvoice, nil
}

func (s *invoiceService) DeleteInvoice(ctx context.Context, req *dto.DeleteInvoiceRequest) error {
	// Fetch Invoice
	getReq := dto.GetInvoiceByIDRequest{ID: req.ID}
	invoice, err := s.invoiceRepo.GetByID(ctx, &getReq)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrNotFound
		} else {
			return fmt.Errorf("internal error getting invoice: %w", err)
		}
	}

	// Fetch Job for Auth Check
	jobReq := dto.GetJobByIDRequest{ID: invoice.JobID}
	job, err := s.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		return fmt.Errorf("internal error getting job: %w", err)
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

	// Call Repo Delete
	err = s.invoiceRepo.Delete(ctx, req)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrNotFound
		} else {
			return fmt.Errorf("internal error deleting invoice: %w", err)
		}
	}

	return nil
}

func (s *invoiceService) ListInvoicesByJob(ctx context.Context, req *dto.ListInvoicesByJobRequest) ([]models.Invoice, error) {
	// Fetch Job using s.jobRepo.GetByID(JobID) to verify existence and for auth check.
	jobReq := dto.GetJobByIDRequest{ID: req.JobID}
	job, err := s.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrNotFound
		} else {
			return nil, fmt.Errorf("internal error getting job: %w", err)
		}
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
		return nil, fmt.Errorf("internal error listing invoices: %w", err)
	}

	return invoices, nil
}