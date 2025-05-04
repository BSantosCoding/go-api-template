// /home/bsant/testing/go-api-template/internal/api/handlers/jobs.go
package handlers

import (
	"errors"
	"log"
	"net/http"

	"go-api-template/internal/api/middleware" // Import middleware for GetUserIDFromContext
	// Import models for mapping
	"go-api-template/internal/services"
	"go-api-template/internal/transport/dto" // Import DTOs

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator"
	"github.com/google/uuid"
)

// JobHandler holds dependencies for job operations.
type JobHandler struct {
	service services.JobService 
	validator *validator.Validate
}

// NewJobHandler creates a new JobHandler.
func NewJobHandler(service services.JobService, validate *validator.Validate) *JobHandler {
	return &JobHandler{
		service: service,
		validator: validate,
	}
}

// CreateJob godoc
// @Summary      Create a new job posting
// @Description  Adds a new job available for contractors. Employer ID is taken from auth context.
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        job body      dto.CreateJobRequest true  "Job details (EmployerID ignored)"
// @Success      201 {object}  dto.JobResponse "Job created successfully"
// @Failure      400 {object}  map[string]string "Bad Request - Invalid input"
// @Failure      401 {object}  map[string]string "Unauthorized"
// @Failure      500 {object}  map[string]string "Internal Server Error"
// @Router       /jobs [post]
// @Security     BearerAuth
func (h *JobHandler) CreateJob(c *gin.Context) {
	// Get EmployerID from auth context
	employerID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"}) // Or Internal Server Error if context missing is unexpected
		return
	}

	var req dto.CreateJobRequest
	// Bind/Validate dto.CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}
	if err := h.validator.Struct(req); err != nil {
		validationErrors := FormatValidationErrors(err.(validator.ValidationErrors))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": validationErrors})
		return
	}

	// Set EmployerID from context
	req.EmployerID = employerID

	// Call h.repo.Create
	createdJob, err := h.service.CreateJob(c.Request.Context(), &req)
	if err != nil {
		// Handle potential repo errors (e.g., conflict, db error)
		log.Printf("Error creating job in repository: %v", err)
		// Check for specific errors if repo returns them (e.g., services.ErrConflict)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create job"})
		return
	}

	// Map result to dto.JobResponse
	jobResponse := MapJobModelToJobResponse(createdJob)

	// Return JSON response
	c.JSON(http.StatusCreated, jobResponse)
}

// GetJobByID godoc
// @Summary      Get a job by ID
// @Description  Retrieves details for a specific job by its ID.
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        id path      string true  "Job ID" Format(uuid)
// @Success      200 {object}  dto.JobResponse "Successfully retrieved job"
// @Failure      400 {object}  map[string]string "Invalid ID format"
// @Failure      401 {object}  map[string]string "Unauthorized"
// @Failure      404 {object}  map[string]string "Job Not Found"
// @Failure      500 {object}  map[string]string "Internal Server Error"
// @Router       /jobs/{id} [get]
// @Security     BearerAuth
func (h *JobHandler) GetJobByID(c *gin.Context) {
	// Parse ID from path
	idStr := c.Param("id")
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID format"})
		return
	}

	// Create dto.GetJobByIDRequest
	req := dto.GetJobByIDRequest{ID: jobID}

	// Call h.repo.GetByID
	job, err := h.service.GetJobByID(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		} else {
			log.Printf("Error fetching job by ID %s: %v", idStr, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve job"})
		}
		return
	}

	// Map result to dto.JobResponse
	jobResponse := MapJobModelToJobResponse(job)

	// Return JSON response
	c.JSON(http.StatusOK, jobResponse)
}

// ListAvailableJobs godoc
// @Summary      List available jobs
// @Description  Retrieves a list of jobs that are 'Waiting' and have no contractor assigned. Supports filtering and pagination.
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        limit query int false "Pagination limit" default(10)
// @Param        offset query int false "Pagination offset" default(0)
// @Param        min_rate query number false "Minimum rate filter"
// @Param        max_rate query number false "Maximum rate filter"
// @Success      200 {array}   dto.JobResponse "Successfully retrieved list of available jobs"
// @Failure      400 {object}  map[string]string "Bad Request - Invalid query parameters"
// @Failure      401 {object}  map[string]string "Unauthorized"
// @Failure      500 {object}  map[string]string "Internal Server Error"
// @Router       /jobs/available [get]
// @Security     BearerAuth
func (h *JobHandler) ListAvailableJobs(c *gin.Context) {
	var req dto.ListAvailableJobsRequest

	// Bind/Validate query params into dto.ListAvailableJobsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters: " + err.Error()})
		return
	}

	// Explicitly validate the struct if needed 
	if err := h.validator.Struct(req); err != nil {
		validationErrors := FormatValidationErrors(err.(validator.ValidationErrors))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": validationErrors})
		return
	}
	// Set defaults if binding didn't 
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	// Call h.repo.ListAvailable
	jobs, err := h.service.ListAvailableJobs(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error listing available jobs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve available jobs"})
		return
	}

	// Map results to []dto.JobResponse
	jobResponses := make([]dto.JobResponse, 0, len(jobs))
	for _, job := range jobs {
		jobResponses = append(jobResponses, MapJobModelToJobResponse(&job))
	}

	// Return JSON response
	c.JSON(http.StatusOK, jobResponses)
}

// ListEmployerJobs godoc
// @Summary      List jobs posted by the authenticated employer
// @Description  Retrieves a list of jobs posted by the currently authenticated user (employer). Supports filtering and pagination.
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        limit query int false "Pagination limit" default(10)
// @Param        offset query int false "Pagination offset" default(0)
// @Param        state query string false "Filter by state (Waiting, Ongoing, Complete, Archived)" Enums(Waiting, Ongoing, Complete, Archived)
// @Param        min_rate query number false "Minimum rate filter"
// @Param        max_rate query number false "Maximum rate filter"
// @Success      200 {array}   dto.JobResponse "Successfully retrieved list of employer's jobs"
// @Failure      400 {object}  map[string]string "Bad Request - Invalid query parameters"
// @Failure      401 {object}  map[string]string "Unauthorized"
// @Failure      500 {object}  map[string]string "Internal Server Error"
// @Router       /jobs/my/employer [get] // Example route
// @Security     BearerAuth
func (h *JobHandler) ListEmployerJobs(c *gin.Context) {
	// Get EmployerID from auth context
	employerID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req dto.ListJobsByEmployerRequest
	// Bind/Validate query params
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

	// Set EmployerID on DTO
	req.EmployerID = employerID

	// Call h.repo.ListByEmployer
	jobs, err := h.service.ListJobsByEmployer(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error listing employer jobs for user %s: %v", employerID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve employer jobs"})
		return
	}

	// Map results to []dto.JobResponse
	jobResponses := make([]dto.JobResponse, 0, len(jobs))
	for _, job := range jobs {
		jobResponses = append(jobResponses, MapJobModelToJobResponse(&job))
	}

	// Return JSON response
	c.JSON(http.StatusOK, jobResponses)
}

// ListContractorJobs godoc
// @Summary      List jobs taken by the authenticated contractor
// @Description  Retrieves a list of jobs assigned to the currently authenticated user (contractor). Supports filtering and pagination.
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        limit query int false "Pagination limit" default(10)
// @Param        offset query int false "Pagination offset" default(0)
// @Param        state query string false "Filter by state (Ongoing, Complete, Archived)" Enums(Ongoing, Complete, Archived)
// @Param        min_rate query number false "Minimum rate filter"
// @Param        max_rate query number false "Maximum rate filter"
// @Success      200 {array}   dto.JobResponse "Successfully retrieved list of contractor's jobs"
// @Failure      400 {object}  map[string]string "Bad Request - Invalid query parameters"
// @Failure      401 {object}  map[string]string "Unauthorized"
// @Failure      500 {object}  map[string]string "Internal Server Error"
// @Router       /jobs/my/contractor [get] // Example route
// @Security     BearerAuth
func (h *JobHandler) ListContractorJobs(c *gin.Context) {
	// Get ContractorID from auth context
	contractorID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req dto.ListJobsByContractorRequest
	// Bind/Validate query params
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

	// Set ContractorID on DTO
	req.ContractorID = contractorID

	// Call h.repo.ListByContractor
	jobs, err := h.service.ListJobsByContractor(c.Request.Context(), &req)
	if err != nil {
		log.Printf("Error listing contractor jobs for user %s: %v", contractorID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve contractor jobs"})
		return
	}

	// Map results to []dto.JobResponse
	jobResponses := make([]dto.JobResponse, 0, len(jobs))
	for _, job := range jobs {
		jobResponses = append(jobResponses, MapJobModelToJobResponse(&job))
	}

	// Return JSON response
	c.JSON(http.StatusOK, jobResponses)
}

// UpdateJobDetails godoc
// @Summary      Update job rate or duration
// @Description  Allows the employer to update the rate or duration ONLY if the job is in 'Waiting' state and has no contractor assigned.
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        id path      string true  "Job ID" Format(uuid)
// @Param        details body dto.UpdateJobDetailsRequest true "Rate and/or Duration to update"
// @Success      200 {object}  dto.JobResponse "Job details updated successfully"
// @Failure      400 {object}  map[string]string "Bad Request - Invalid input"
// @Failure      401 {object}  map[string]string "Unauthorized"
// @Failure      403 {object}  map[string]string "Forbidden - User cannot update details or job state prevents it"
// @Failure      404 {object}  map[string]string "Job Not Found"
// @Failure      500 {object}  map[string]string "Internal Server Error"
// @Router       /jobs/{id}/details [patch]
// @Security     BearerAuth
func (h *JobHandler) UpdateJobDetails(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("UpdateJobDetails: Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	idStr := c.Param("id")
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID format"})
		return
	}

	var req dto.UpdateJobDetailsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}
	if err := h.validator.Struct(req); err != nil {
		validationErrors := FormatValidationErrors(err.(validator.ValidationErrors))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": validationErrors})
		return
	}
	if req.Rate == nil && req.Duration == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No update fields (rate, duration) provided"})
		return
	}
	req.UserID = userID
	req.JobID = jobID

	updatedJob, err := h.service.UpdateJobDetails(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found during update"})
		} else if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Cannot update job in its current state"})
		} else {
			log.Printf("UpdateJobDetails: Error updating job %s: %v", jobID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update job details"})
		}
		return
	}

	c.JSON(http.StatusOK, MapJobModelToJobResponse(updatedJob))
}

// UpdateJobState godoc
// @Summary      Update job state
// @Description  Allows the employer or the assigned contractor to update the job state according to valid transitions (Waiting -> Ongoing -> Complete -> Archived).
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        id path      string true  "Job ID" Format(uuid)
// @Param        state body dto.UpdateJobStateRequest true "New state for the job"
// @Success      200 {object}  dto.JobResponse "Job state updated successfully"
// @Failure      400 {object}  map[string]string "Bad Request - Invalid input or invalid state transition"
// @Failure      401 {object}  map[string]string "Unauthorized"
// @Failure      403 {object}  map[string]string "Forbidden - User cannot update state for this job"
// @Failure      404 {object}  map[string]string "Job Not Found"
// @Failure      500 {object}  map[string]string "Internal Server Error"
// @Router       /jobs/{id}/state [patch]
// @Security     BearerAuth
func (h *JobHandler) UpdateJobState(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("UpdateJobState: Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	idStr := c.Param("id")
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID format"})
		return
	}

	var req dto.UpdateJobStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}
	if err := h.validator.Struct(req); err != nil {
		validationErrors := FormatValidationErrors(err.(validator.ValidationErrors))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": validationErrors})
		return
	}

	req.JobID = jobID
	req.UserID = userID

	updatedJob, err := h.service.UpdateJobState(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found during update"})
		} else if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Cannot update job state in current state"})
		} else {
			log.Printf("UpdateJobState: Error updating job state %s: %v", jobID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update job state"})
		}
		return
	}

	c.JSON(http.StatusOK, MapJobModelToJobResponse(updatedJob))
}

// DeleteJob
// @Summary      Delete a job
// @Description  Deletes a job posting. Allowed only by the employer if the job is in 'Waiting' state and has no contractor.
// @Tags         jobs
// @Accept       json
// @Produce      json
// @Param        id path      string true  "Job ID" Format(uuid)
// @Success      204 {object}  nil "Job deleted successfully"
// @Failure      400 {object}  map[string]string "Invalid ID format"
// @Failure      401 {object}  map[string]string "Unauthorized"
// @Failure      403 {object}  map[string]string "Forbidden - User cannot delete this job or job state prevents deletion"
// @Failure      404 {object}  map[string]string "Job Not Found"
// @Failure      500 {object}  map[string]string "Internal Server Error"
// @Router       /jobs/{id} [delete]
// @Security     BearerAuth
func (h *JobHandler) DeleteJob(c *gin.Context) {
	// Get UserID from auth context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse JobID from path
	idStr := c.Param("id")
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID format"})
		return
	}

	// Create dto.DeleteJobRequest
	req := dto.DeleteJobRequest{ID: jobID, UserID: userID}

	// Call h.repo.Delete
	err = h.service.DeleteJob(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrNotFound) { 
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		} else if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Cannot delete job in current state"})
		} else if errors.Is(err, services.ErrInvalidState) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Job is not in a deletable state"})
		} else {
			log.Printf("Error deleting job %s: %v", jobID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete job"})
		}
		return
	}

	// Return 204 No Content
	c.Status(http.StatusNoContent)
}
