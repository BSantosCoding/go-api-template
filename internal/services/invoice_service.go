package services

import (
	"context"
	"errors"
	"fmt"
	"go-api-template/ent"
	"go-api-template/ent/invoice"
	"go-api-template/ent/job"
	"go-api-template/internal/api"
	"go-api-template/internal/api/middleware"
	"go-api-template/internal/storage"
	"go-api-template/internal/transport/dto"
	"log"

	"github.com/google/uuid"
	oapi_middleware "github.com/oapi-codegen/gin-middleware"
)

func (sd *ServerDefinition) PostInvoices(ctx context.Context, request api.PostInvoicesRequestObject) (api.PostInvoicesResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	jobReq := dto.GetJobByIDRequest{ID: uuid.MustParse(request.Body.JobId)}
	jobFound, err := sd.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		log.Printf("CreateInvoice: Error fetching job %s: %v", request.Body.JobId, err)
		return nil, MapRepoError(err, "fetching job for invoice creation")
	}

	// Authorization & State checks
	if jobFound.ContractorID == uuid.Nil || jobFound.ContractorID != userID {
		log.Printf("CreateInvoice: Forbidden attempt by user %s on job %s (Contractor: %v)", userID, request.Body.JobId, jobFound.ContractorID)
		return nil, ErrForbidden
	}
	if jobFound.State != job.StateOngoing {
		log.Printf("CreateInvoice: Attempt to create invoice for job %s in state %s", request.Body.JobId, jobFound.State)
		return nil, ErrInvalidState // Correct error type
	}

	// --- Transaction Start ---
	tx, err := sd.db.Tx(ctx)
	if err != nil {
		log.Printf("CreateInvoice: Error beginning transaction: %v", err)
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if anything fails

	txInvoiceRepo := sd.invoiceRepo.WithTx(tx)
	intervalReq := &dto.GetMaxIntervalForJobRequest{JobID: uuid.MustParse(request.Body.JobId)}
	maxIntervalNum, err := txInvoiceRepo.GetMaxIntervalForJob(ctx, intervalReq) // Use txInvoiceRepo
	if err != nil {
		return nil, MapRepoError(err, "getting max interval for job")
	}
	nextIntervalNumber := maxIntervalNum + 1

	if jobFound.InvoiceInterval <= 0 {
		return nil, ErrInvalidInvoiceInterval
	}

	maxPossibleIntervals := jobFound.Duration / jobFound.InvoiceInterval
	remainderHours := jobFound.Duration % jobFound.InvoiceInterval
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
		hoursForThisInterval = jobFound.InvoiceInterval
	}

	baseValue := jobFound.Rate * float64(hoursForThisInterval) // Use calculated hours
	finalValue := baseValue
	if request.Body.Adjustment != nil {
		finalValue += float64(*request.Body.Adjustment)
	}
	if finalValue < 0 { // Ensure non-negative value
		finalValue = 0
	}

	invoiceToCreate := &ent.Invoice{
		JobID:          uuid.MustParse(request.Body.JobId),
		IntervalNumber: nextIntervalNumber,
		Value:          finalValue,
		State:          invoice.StateWaiting,
		ID:             uuid.New(), // Generate a new UUID for the invoice
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
	if err := tx.Commit(); err != nil {
		log.Printf("CreateInvoice: Error committing transaction: %v", err)
		return nil, MapRepoError(err, "committing invoice creation")
	}
	// --- End Transaction ---

	mappedInvoice := MapEntInvoiceToResponse(invoice)

	return api.PostInvoices201JSONResponse(mappedInvoice), nil
}

func (sd *ServerDefinition) GetInvoicesId(ctx context.Context, request api.GetInvoicesIdRequestObject) (api.GetInvoicesIdResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	// Call sd.invoiceRepo.GetByID
	invoice, err := sd.invoiceRepo.GetByID(ctx, &dto.GetInvoiceByIDRequest{ID: request.Id})
	if err != nil {
		return nil, MapRepoError(err, "getting invoice")
	}

	// Fetch associated Job using sd.jobRepo.GetByID(invoice.JobID) for auth check
	jobReq := dto.GetJobByIDRequest{ID: invoice.JobID}
	job, err := sd.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		return nil, MapRepoError(err, "getting job")
	}

	// Authorization Check: Verify userID matches job.EmployerID or job.ContractorID.
	isEmployer := job.EmployerID == userID
	isContractor := job.ContractorID != uuid.Nil && job.ContractorID == userID
	if !(isEmployer || isContractor) {
		return nil, ErrForbidden
	}

	mappedInvoice := MapEntInvoiceToResponse(invoice)

	return api.GetInvoicesId200JSONResponse(mappedInvoice), nil
}

func (sd *ServerDefinition) PatchInvoicesIdState(ctx context.Context, request api.PatchInvoicesIdStateRequestObject) (api.PatchInvoicesIdStateResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	// --- Transaction Start ---
	tx, err := sd.db.Tx(ctx)
	if err != nil {
		log.Printf("UpdateInvoiceState: Error beginning transaction: %v", err)
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if anything fails

	txInvoiceRepo := sd.invoiceRepo.WithTx(tx)
	txJobRepo := sd.jobRepo.WithTx(tx)
	// --- End Transaction Setup ---

	// Fetch Invoice
	getReq := dto.GetInvoiceByIDRequest{ID: request.Id}
	invoiceFound, err := txInvoiceRepo.GetByID(ctx, &getReq) // Use txInvoiceRepo
	if err != nil {
		return nil, MapRepoError(err, "getting invoice")
	}

	// Fetch Job for Auth Check
	jobReq := dto.GetJobByIDRequest{ID: invoiceFound.JobID}
	job, err := txJobRepo.GetByID(ctx, &jobReq) // Use txJobRepo
	if err != nil {
		return nil, MapRepoError(err, "getting job")
	}

	// --- Authorization Check: ONLY Employer ---
	isEmployer := job.EmployerID == userID
	if !isEmployer {
		log.Printf("UpdateInvoiceState: Forbidden attempt by user %s on invoice %s (Job Employer: %v)", userID, request.Id, job.EmployerID)
		return nil, ErrForbidden
	}
	// --- End Auth Check ---

	// Check State Transition
	if !isValidInvoiceStateTransition(invoiceFound.State, invoice.State(request.Body.State)) {
		return nil, ErrInvalidTransition
	}

	updatedInvoice, err := txInvoiceRepo.UpdateState(ctx, &dto.UpdateInvoiceStateRequest{ID: request.Id, NewState: invoice.State(request.Body.State)}) // Use txInvoiceRepo
	if err != nil {
		return nil, MapRepoError(err, "updating invoice state")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(); err != nil {
		log.Printf("UpdateInvoiceState: Error committing transaction: %v", err)
		return nil, fmt.Errorf("internal error committing invoice update: %w", err)
	}
	// --- End Transaction ---

	mappedInvoice := MapEntInvoiceToResponse(updatedInvoice)

	return api.PatchInvoicesIdState200JSONResponse(mappedInvoice), nil
}

func (sd *ServerDefinition) DeleteInvoicesId(ctx context.Context, request api.DeleteInvoicesIdRequestObject) (api.DeleteInvoicesIdResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	// Fetch Invoice
	getReq := dto.GetInvoiceByIDRequest{ID: request.Id}
	invoiceFound, err := sd.invoiceRepo.GetByID(ctx, &getReq)
	// Note: No transaction needed here yet, just checking existence and state first.
	if err != nil {
		return nil, MapRepoError(err, "getting invoice for deletion check")
	}

	// Fetch Job for Auth Check
	jobReq := dto.GetJobByIDRequest{ID: invoiceFound.JobID}
	job, err := sd.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		return nil, MapRepoError(err, "getting job for deletion check")
	}

	// --- Transaction Start (only needed for the delete itself, but good practice) ---
	tx, err := sd.db.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}

	// --- Authorization Check: ONLY Contractor + State Waiting ---
	isContractor := job.ContractorID != uuid.Nil && job.ContractorID == userID
	if !isContractor {
		return nil, ErrForbidden
	}
	if invoiceFound.State != invoice.StateWaiting {
		return nil, ErrInvalidState
	}
	// --- End Auth Check ---

	txInvoiceRepo := sd.invoiceRepo.WithTx(tx)

	// Call Repo Delete
	err = txInvoiceRepo.Delete(ctx, &dto.DeleteInvoiceRequest{ID: request.Id}) // Use txInvoiceRepo
	if err != nil {
		tx.Rollback() // Rollback on error
		return nil, MapRepoError(err, "deleting invoice")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(); err != nil {
		log.Printf("DeleteInvoice: Error committing transaction: %v", err)
		return nil, fmt.Errorf("internal error committing invoice deletion: %w", err)
	}

	return api.DeleteInvoicesId204Response{}, nil
}

func (sd *ServerDefinition) GetJobsIdInvoices(ctx context.Context, request api.GetJobsIdInvoicesRequestObject) (api.GetJobsIdInvoicesResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	// Fetch Job using sd.jobRepo.GetByID(JobID) to verify existence and for auth check.
	jobReq := dto.GetJobByIDRequest{ID: request.Id}
	job, err := sd.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		return nil, MapRepoError(err, "getting job for listing invoices")
	}

	// Authorization Check: Verify userID matches job.EmployerID or job.ContractorID.
	isEmployer := job.EmployerID == userID
	isContractor := job.ContractorID != uuid.Nil && job.ContractorID == userID
	if !(isEmployer || isContractor) {
		return nil, ErrForbidden
	}

	// Call sd.invoiceRepo.ListByJob
	invoices, err := sd.invoiceRepo.ListByJob(ctx, &dto.ListInvoicesByJobRequest{
		JobID:  request.Id,
		Limit:  *request.Params.Limit,
		Offset: *request.Params.Offset,
		State:  (*invoice.State)(request.Params.State),
	})
	if err != nil {
		return nil, MapRepoError(err, "listing invoices")
	}

	mappedInvoices := make([]api.DtoInvoiceResponse, len(invoices))
	for i, invoice := range invoices {
		mappedInvoices[i] = MapEntInvoiceToResponse(invoice)
	}
	return api.GetJobsIdInvoices200JSONResponse(mappedInvoices), nil
}
