package handlers

import (
	"errors"
	"go-api-template/internal/api/middleware"
	"go-api-template/internal/services"
	"go-api-template/internal/transport/dto"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator"
	"github.com/google/uuid"
)

// InvoiceHandler holds dependencies for invoice operations.
type InvoiceHandler struct {
	service   services.InvoiceService
	validator *validator.Validate
}

// NewInvoiceHandler creates a new InvoiceHandler.
func NewInvoiceHandler(service services.InvoiceService, validate *validator.Validate) *InvoiceHandler {
	return &InvoiceHandler{
		service:   service,
		validator: validate,
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
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("CreateInvoice: Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req dto.CreateInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}
	req.UserId = userID

	if err := h.validator.Struct(req); err != nil {
		validationErrors := FormatValidationErrors(err.(validator.ValidationErrors))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": validationErrors})
		return
	}

	createdInvoice, err := h.service.CreateInvoice(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "Invoice for this interval already exists"})
		} else if errors.Is(err, services.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		} else if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "User is not the contractor for this job or job not ongoing"})
		} else if errors.Is(err, services.ErrInvalidState) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Job is not in a valid state for invoice creation"})
		} else if errors.Is(err, services.ErrInvalidInvoiceInterval) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invoice interval exceeds job duration"})
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
	req := dto.GetInvoiceByIDRequest{ID: invoiceID, UserId: userID}

	invoice, err := h.service.GetInvoiceByID(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
		} else if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "User not associated with this invoice's job"})
		} else {
			log.Printf("GetInvoiceByID: Error fetching invoice %s: %v", invoiceID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve invoice"})
		}
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

	// Bind/Validate query params into dto.ListInvoicesByJobRequest
	var req dto.ListInvoicesByJobRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters: " + err.Error()})
		return
	}
	req.JobID = jobID // Set JobID from path
	req.UserId = userID

	if err := h.validator.Struct(req); err != nil {
		validationErrors := FormatValidationErrors(err.(validator.ValidationErrors))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": validationErrors})
		return
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	invoices, err := h.service.ListInvoicesByJob(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		} else if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "User not associated with this job"})
		} else {
			log.Printf("ListInvoicesByJob: Error listing invoices for job %s: %v", jobID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve invoices"})
		}
		return
	}

	// Map results to []dto.InvoiceResponse
	invoiceResponses := make([]dto.InvoiceResponse, 0, len(invoices))
	for _, invoice := range invoices {
		invoiceResponses = append(invoiceResponses, MapInvoiceModelToInvoiceResponse(invoice))
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
	req.UserId = userID

	if err := h.validator.Struct(req); err != nil {
		validationErrors := FormatValidationErrors(err.(validator.ValidationErrors))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": validationErrors})
		return
	}

	updatedInvoice, err := h.service.UpdateInvoiceState(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found during update"})
		} else if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "User is not the contractor for this invoice's job"})
		} else if errors.Is(err, services.ErrInvalidTransition) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid state transition"})
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

	// Create Delete Request DTO
	req := dto.DeleteInvoiceRequest{ID: invoiceID, UserId: userID}

	err = h.service.DeleteInvoice(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found during update"})
		} else if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "User is not the contractor for this invoice's job"})
		} else if errors.Is(err, services.ErrInvalidTransition) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid state transition"})
		} else {
			log.Printf("UpdateInvoiceState: Error updating invoice state %s: %v", invoiceID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update invoice state"})
		}
		return
	}

	// Return Success
	c.Status(http.StatusNoContent)
}
