package handlers

import (
	"errors"
	"fmt"
	"go-api-template/internal/api/middleware"
	"go-api-template/internal/models"
	"go-api-template/internal/storage"
	"go-api-template/internal/transport/dto"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator"
	"github.com/google/uuid"
)

// InvoiceHandler holds dependencies for invoice operations.
type InvoiceHandler struct {
	invoiceRepo storage.InvoiceRepository
	jobRepo     storage.JobRepository // Needed for Create logic and Auth checks
	validator   *validator.Validate
}

// NewInvoiceHandler creates a new InvoiceHandler.
func NewInvoiceHandler(invoiceRepo storage.InvoiceRepository, jobRepo storage.JobRepository, validate *validator.Validate) *InvoiceHandler {
	return &InvoiceHandler{
		invoiceRepo: invoiceRepo,
		jobRepo:     jobRepo,
		validator:   validate,
	}
}

// CreateInvoice godoc
// @Summary      Create an invoice for a job
// @Description  Creates the next sequential invoice for a specified job, calculating value based on job rate/interval and applying optional adjustment. Handles partial final intervals. Requires user to be the assigned contractor and job to be 'Ongoing'.
// @Tags         invoices
// @Accept       json
// @Produce      json
// @Param        invoice body      dto.CreateInvoiceRequest true  "Invoice creation details (JobID and optional Adjustment)"
// @Success      201 {object}  dto.InvoiceResponse "Invoice created successfully"
// @Failure      400 {object}  map[string]string "Bad Request - Invalid input, job not found, or invoice not allowed (e.g., max intervals reached)"
// @Failure      401 {object}  map[string]string "Unauthorized"
// @Failure      403 {object}  map[string]string "Forbidden - User is not the contractor for this job or job not ongoing"
// @Failure      409 {object}  map[string]string "Conflict - Invoice for this interval already exists"
// @Failure      500 {object}  map[string]string "Internal Server Error"
// @Router       /invoices [post]
// @Security     BearerAuth
func (h *InvoiceHandler) CreateInvoice(c *gin.Context) {
	// 1. Get UserID (ContractorID)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("CreateInvoice: Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// 2. Bind/Validate Request
	var req dto.CreateInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}
	if err := h.validator.Struct(req); err != nil {
		validationErrors := FormatValidationErrors(err.(validator.ValidationErrors))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": validationErrors})
		return
	}

	// 3. Fetch Job
	jobReq := dto.GetJobByIDRequest{ID: req.JobID}
	job, err := h.jobRepo.GetByID(c.Request.Context(), &jobReq)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Job not found"})
		} else {
			log.Printf("CreateInvoice: Error fetching job %s: %v", req.JobID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve job details"})
		}
		return
	}

	// 4. Authorization Check
	if job.ContractorID == nil || *job.ContractorID != userID {
		log.Printf("CreateInvoice: Forbidden attempt by user %s (not contractor %v) for job %s", userID, job.ContractorID, req.JobID)
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Only the assigned contractor can create invoices for this job"})
		return
	}
	if job.State != models.JobStateOngoing {
		log.Printf("CreateInvoice: Forbidden attempt for job %s in state %s", req.JobID, job.State)
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Invoices can only be created for 'Ongoing' jobs"})
		return
	}

	// 5. Determine next interval number
	intervalReq := &dto.GetMaxIntervalForJobRequest{JobID: req.JobID}
	maxIntervalNum, err := h.invoiceRepo.GetMaxIntervalForJob(c.Request.Context(), intervalReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to determine next invoice interval"})
		return
	}
	nextIntervalNumber := maxIntervalNum + 1

	// 6. Check interval validity and calculate hours for this interval
	if job.InvoiceInterval <= 0 {
		log.Printf("CreateInvoice: Invalid InvoiceInterval (%d) for job %s", job.InvoiceInterval, req.JobID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid job configuration prevents invoice creation"})
		return
	}

	maxPossibleIntervals := job.Duration / job.InvoiceInterval
	remainderHours := job.Duration % job.InvoiceInterval
	isPartialLastInterval := remainderHours != 0
	if isPartialLastInterval {
		maxPossibleIntervals++
	}

	if nextIntervalNumber > maxPossibleIntervals {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Cannot create invoice for interval %d: exceeds maximum possible intervals (%d) for job duration", nextIntervalNumber, maxPossibleIntervals)})
		return
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

	// 7. Calculate base value using hoursForThisInterval and apply adjustment
	baseValue := job.Rate * float64(hoursForThisInterval) // Use calculated hours
	finalValue := baseValue
	if req.Adjustment != nil {
		finalValue += *req.Adjustment
	}
	if finalValue < 0 { // Ensure non-negative value
		finalValue = 0
	}

	// 8. Construct models.Invoice object
	invoiceToCreate := &models.Invoice{
		ID:             uuid.New(),
		JobID:          req.JobID,
		Value:          finalValue,
		IntervalNumber: nextIntervalNumber,
		State:          models.InvoiceStateWaiting,
	}

	// 9. Call repo Create
	createdInvoice, err := h.invoiceRepo.Create(c.Request.Context(), invoiceToCreate)
	if err != nil {
		if errors.Is(err, storage.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Invoice for interval %d already exists for this job", nextIntervalNumber)})
		} else {
			log.Printf("CreateInvoice: Error saving invoice for job %s: %v", req.JobID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create invoice"})
		}
		return
	}

	// 10. Map and Return
	c.JSON(http.StatusCreated, MapInvoiceModelToInvoiceResponse(createdInvoice))
}

// GetInvoiceByID godoc
// @Summary      Get an invoice by ID
// @Description  Retrieves details for a specific invoice by its ID. Requires user to be associated with the job (employer or contractor).
// @Tags         invoices
// @Accept       json
// @Produce      json
// @Param        id path      string true  "Invoice ID" Format(uuid)
// @Success      200 {object}  dto.InvoiceResponse "Successfully retrieved invoice"
// @Failure      400 {object}  map[string]string "Invalid ID format"
// @Failure      401 {object}  map[string]string "Unauthorized"
// @Failure      403 {object}  map[string]string "Forbidden - User not associated with this invoice's job"
// @Failure      404 {object}  map[string]string "Invoice Not Found"
// @Failure      500 {object}  map[string]string "Internal Server Error"
// @Router       /invoices/{id} [get]
// @Security     BearerAuth
func (h *InvoiceHandler) GetInvoiceByID(c *gin.Context) {
	// Get UserID from auth context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("GetInvoiceByID: Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse InvoiceID from path
	idStr := c.Param("id")
	invoiceID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid invoice ID format"})
		return
	}

	// Create dto.GetInvoiceByIDRequest
	req := dto.GetInvoiceByIDRequest{ID: invoiceID}

	// Call h.invoiceRepo.GetByID
	invoice, err := h.invoiceRepo.GetByID(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
		} else {
			log.Printf("GetInvoiceByID: Error fetching invoice %s: %v", invoiceID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve invoice"})
		}
		return
	}

	// Fetch associated Job using h.jobRepo.GetByID(invoice.JobID) for auth check
	jobReq := dto.GetJobByIDRequest{ID: invoice.JobID}
	job, err := h.jobRepo.GetByID(c.Request.Context(), &jobReq)
	if err != nil {
		// If job not found, something is wrong with data integrity or invoice shouldn't exist
		log.Printf("GetInvoiceByID: Error fetching job %s associated with invoice %s: %v", invoice.JobID, invoiceID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify invoice association"})
		return
	}

	// Authorization Check: Verify UserID matches job.EmployerID or job.ContractorID.
	isEmployer := job.EmployerID == userID
	isContractor := job.ContractorID != nil && *job.ContractorID == userID
	if !(isEmployer || isContractor) {
		log.Printf("GetInvoiceByID: Forbidden attempt by user %s on invoice %s (job %s)", userID, invoiceID, job.ID)
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: You are not associated with this invoice's job"})
		return
	}

	// Map result to dto.InvoiceResponse
	invoiceResponse := MapInvoiceModelToInvoiceResponse(invoice)

	// Return JSON response
	c.JSON(http.StatusOK, invoiceResponse)
}

// ListInvoicesByJob godoc
// @Summary      List invoices for a specific job
// @Description  Retrieves a list of invoices associated with a given job ID. Requires user to be associated with the job. Supports filtering and pagination.
// @Tags         invoices
// @Accept       json
// @Produce      json
// @Param        id path string true "Job ID" Format(uuid)
// @Param        limit query int false "Pagination limit" default(10)
// @Param        offset query int false "Pagination offset" default(0)
// @Param        state query string false "Filter by state (Waiting, Complete)" Enums(Waiting, Complete)
// @Success      200 {array}   dto.InvoiceResponse "Successfully retrieved list of invoices"
// @Failure      400 {object}  map[string]string "Bad Request - Invalid Job ID format or query parameters"
// @Failure      401 {object}  map[string]string "Unauthorized"
// @Failure      403 {object}  map[string]string "Forbidden - User not associated with this job"
// @Failure      404 {object}  map[string]string "Job Not Found" // Check if job exists first
// @Failure      500 {object}  map[string]string "Internal Server Error"
// @Router       /jobs/{jobId}/invoices [get] // Example route nesting
// @Security     BearerAuth
func (h *InvoiceHandler) ListInvoicesByJob(c *gin.Context) {
	// Get UserID from auth context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("ListInvoicesByJob: Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse JobID from path
	jobIdStr := c.Param("id") // Matches route param name
	jobID, err := uuid.Parse(jobIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID format"})
		return
	}

	// Fetch Job using h.jobRepo.GetByID(JobID) to verify existence and for auth check.
	jobReq := dto.GetJobByIDRequest{ID: jobID}
	job, err := h.jobRepo.GetByID(c.Request.Context(), &jobReq)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		} else {
			log.Printf("ListInvoicesByJob: Error fetching job %s: %v", jobID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve job details"})
		}
		return
	}

	// Authorization Check: Verify UserID matches job.EmployerID or job.ContractorID.
	isEmployer := job.EmployerID == userID
	isContractor := job.ContractorID != nil && *job.ContractorID == userID
	if !(isEmployer || isContractor) {
		log.Printf("ListInvoicesByJob: Forbidden attempt by user %s on job %s", userID, jobID)
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: You are not associated with this job"})
		return
	}

	// Bind/Validate query params into dto.ListInvoicesByJobRequest
	var req dto.ListInvoicesByJobRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters: " + err.Error()})
		return
	}
	if err := h.validator.Struct(req); err != nil {
		validationErrors := FormatValidationErrors(err.(validator.ValidationErrors))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": validationErrors})
		return
	}
	if req.Limit <= 0 { req.Limit = 10 }
	if req.Offset < 0 { req.Offset = 0 }

	// Set JobID on DTO from path param
	req.JobID = jobID

	// Call h.invoiceRepo.ListByJob
	invoices, err := h.invoiceRepo.ListByJob(c.Request.Context(), &req)
	if err != nil {
		log.Printf("ListInvoicesByJob: Error listing invoices for job %s: %v", jobID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve invoices"})
		return
	}

	// Map results to []dto.InvoiceResponse
	invoiceResponses := make([]dto.InvoiceResponse, 0, len(invoices))
	for _, invoice := range invoices {
		invoiceResponses = append(invoiceResponses, MapInvoiceModelToInvoiceResponse(&invoice))
	}

	// Return JSON response
	c.JSON(http.StatusOK, invoiceResponses)
}

// UpdateInvoiceState godoc
// @Summary      Update invoice state
// @Description  Updates the state of an invoice (e.g., from 'Waiting' to 'Complete'). ONLY allowed by the assigned contractor.
// @Tags         invoices
// @Accept       json
// @Produce      json
// @Param        id path      string true  "Invoice ID" Format(uuid)
// @Param        state body      dto.UpdateInvoiceStateRequest true  "New state for the invoice"
// @Success      200 {object}  dto.InvoiceResponse "Invoice state updated successfully"
// @Failure      400 {object}  map[string]string "Bad Request - Invalid input or state transition not allowed"
// @Failure      401 {object}  map[string]string "Unauthorized"
// @Failure      403 {object}  map[string]string "Forbidden - User is not the contractor for this invoice's job"
// @Failure      404 {object}  map[string]string "Invoice Not Found"
// @Failure      500 {object}  map[string]string "Internal Server Error"
// @Router       /invoices/{id}/state [patch]
// @Security     BearerAuth
func (h *InvoiceHandler) UpdateInvoiceState(c *gin.Context) {
	// Get UserID
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("UpdateInvoiceState: Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse InvoiceID
	idStr := c.Param("id")
	invoiceID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid invoice ID format"})
		return
	}

	// Bind/Validate Request Body
	var req dto.UpdateInvoiceStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}
	req.ID = invoiceID // Set ID from path
	if err := h.validator.Struct(req); err != nil {
		validationErrors := FormatValidationErrors(err.(validator.ValidationErrors))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": validationErrors})
		return
	}

	// Fetch Invoice
	getReq := dto.GetInvoiceByIDRequest{ID: invoiceID}
	invoice, err := h.invoiceRepo.GetByID(c.Request.Context(), &getReq)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
		} else {
			log.Printf("UpdateInvoiceState: Error fetching invoice %s: %v", invoiceID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve invoice"})
		}
		return
	}

	// Fetch Job for Auth Check
	jobReq := dto.GetJobByIDRequest{ID: invoice.JobID}
	job, err := h.jobRepo.GetByID(c.Request.Context(), &jobReq)
	if err != nil {
		log.Printf("UpdateInvoiceState: Error fetching job %s associated with invoice %s: %v", invoice.JobID, invoiceID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify invoice association"})
		return
	}

	// --- Authorization Check: ONLY Contractor ---
	isContractor := job.ContractorID != nil && *job.ContractorID == userID
	if !isContractor {
		log.Printf("UpdateInvoiceState: Forbidden attempt by user %s (not contractor %v) for invoice %s", userID, job.ContractorID, invoiceID)
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Only the assigned contractor can update the invoice state"})
		return
	}
	// --- End Auth Check ---

	// Check State Transition
	if !isValidInvoiceStateTransition(invoice.State, req.NewState) {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid state transition from %s to %s", invoice.State, req.NewState)})
		return
	}

	// Call Repo UpdateState
	updatedInvoice, err := h.invoiceRepo.UpdateState(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found during update"})
		} else {
			log.Printf("UpdateInvoiceState: Error updating invoice state %s: %v", invoiceID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update invoice state"})
		}
		return
	}

	// Map and Return
	c.JSON(http.StatusOK, MapInvoiceModelToInvoiceResponse(updatedInvoice))
}


// DeleteInvoice godoc
// @Summary      Delete an invoice
// @Description  Deletes an invoice. Only allowed by the assigned contractor if the invoice state is 'Waiting'.
// @Tags         invoices
// @Accept       json
// @Produce      json
// @Param        id path      string true  "Invoice ID" Format(uuid)
// @Success      204 {object}  nil "Invoice deleted successfully"
// @Failure      400 {object}  map[string]string "Invalid ID format"
// @Failure      401 {object}  map[string]string "Unauthorized"
// @Failure      403 {object}  map[string]string "Forbidden - User is not contractor or invoice state prevents deletion"
// @Failure      404 {object}  map[string]string "Invoice Not Found"
// @Failure      500 {object}  map[string]string "Internal Server Error"
// @Router       /invoices/{id} [delete]
// @Security     BearerAuth
func (h *InvoiceHandler) DeleteInvoice(c *gin.Context) {
	// Get UserID
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("DeleteInvoice: Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse InvoiceID
	idStr := c.Param("id")
	invoiceID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid invoice ID format"})
		return
	}

	// Fetch Invoice
	getReq := dto.GetInvoiceByIDRequest{ID: invoiceID}
	invoice, err := h.invoiceRepo.GetByID(c.Request.Context(), &getReq)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
		} else {
			log.Printf("DeleteInvoice: Error fetching invoice %s: %v", invoiceID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve invoice"})
		}
		return
	}

	// Fetch Job for Auth Check
	jobReq := dto.GetJobByIDRequest{ID: invoice.JobID}
	job, err := h.jobRepo.GetByID(c.Request.Context(), &jobReq)
	if err != nil {
		log.Printf("DeleteInvoice: Error fetching job %s associated with invoice %s: %v", invoice.JobID, invoiceID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify invoice association"})
		return
	}

	// --- Authorization Check: ONLY Contractor + State Waiting ---
	isContractor := job.ContractorID != nil && *job.ContractorID == userID
	if !isContractor {
		log.Printf("DeleteInvoice: Forbidden attempt by user %s (not contractor %v) for invoice %s", userID, job.ContractorID, invoiceID)
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Only the assigned contractor can delete this invoice"})
		return
	}
	if invoice.State != models.InvoiceStateWaiting {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Cannot delete an invoice that is not in 'Waiting' state"})
		return
	}
	// --- End Auth Check ---

	// Create Delete Request DTO
	req := dto.DeleteInvoiceRequest{ID: invoiceID}

	// Call Repo Delete
	err = h.invoiceRepo.Delete(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
		} else {
			log.Printf("DeleteInvoice: Error deleting invoice %s: %v", invoiceID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete invoice"})
		}
		return
	}

	// Return Success
	c.Status(http.StatusNoContent)
}