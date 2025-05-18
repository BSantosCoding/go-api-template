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

// JobApplicationHandler holds dependencies for job application operations.
type JobApplicationHandler struct {
	service   services.JobApplicationService
	validator *validator.Validate
}

// NewJobApplicationHandler creates a new JobApplicationHandler.
func NewJobApplicationHandler(service services.JobApplicationService, validate *validator.Validate) *JobApplicationHandler {
	return &JobApplicationHandler{
		service:   service,
		validator: validate,
	}
}

// Compile-time check to ensure JobApplicationHandler implements JobApplicationHandlerInterface
var _ JobApplicationHandlerInterface = (*JobApplicationHandler)(nil)

// ApplyToJob godoc
//	@Summary		Apply for a job
//	@Description	Allows a logged-in user (contractor) to apply for a specific job.
//	@Tags			job_applications
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string						true	"Job ID to apply for"	Format(uuid)
//	@Success		201	{object}	dto.JobApplicationResponse	"Application created successfully"
//	@Failure		400	{object}	map[string]string			"Bad Request - Invalid Job ID or already applied"
//	@Failure		401	{object}	map[string]string			"Unauthorized"
//	@Failure		403	{object}	map[string]string			"Forbidden - Cannot apply (e.g., employer applying to own job, job not available)"
//	@Failure		404	{object}	map[string]string			"Not Found - Job not found"
//	@Failure		500	{object}	map[string]string			"Internal Server Error"
//	@Router			/jobs/{id}/apply [post]
//	@Security		BearerAuth
func (h *JobApplicationHandler) ApplyToJob(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("ApplyToJob: Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	jobIDStr := c.Param("id") // Assuming the job ID is in the path like /jobs/{id}/apply
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID format"})
		return
	}

	req := dto.ApplyToJobRequest{
		JobID:        jobID,
		ContractorID: userID, // Set the contractor ID from the authenticated user
	}

	// No need to validate req here as fields are set internally or from path param

	application, err := h.service.ApplyToJob(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		} else if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()}) // Use specific error message from service
		} else if errors.Is(err, services.ErrInvalidState) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()}) // Use 409 Conflict for state issues like job not available
		} else if errors.Is(err, services.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()}) // Use 409 Conflict for already applied
		} else {
			log.Printf("ApplyToJob: Error applying to job %s: %v", jobID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply for job"})
		}
		return
	}

	// Map result to dto.JobApplicationResponse (Need to add this mapping function)
	appResponse := MapJobApplicationModelToResponse(application) // Assuming this function exists/will be created

	c.JSON(http.StatusCreated, appResponse)
}

// GetApplicationByID godoc
//	@Summary		Get a job application by ID
//	@Description	Retrieves details for a specific job application. Requires user to be the applicant or the job employer.
//	@Tags			job_applications
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string						true	"Application ID"	Format(uuid)
//	@Success		200	{object}	dto.JobApplicationResponse	"Successfully retrieved application"
//	@Failure		400	{object}	map[string]string			"Invalid ID format"
//	@Failure		401	{object}	map[string]string			"Unauthorized"
//	@Failure		403	{object}	map[string]string			"Forbidden - User not associated with this application"
//	@Failure		404	{object}	map[string]string			"Application Not Found"
//	@Failure		500	{object}	map[string]string			"Internal Server Error"
//	@Router			/applications/{id} [get]
//	@Security		BearerAuth
func (h *JobApplicationHandler) GetApplicationByID(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("GetApplicationByID: Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID format"})
		return
	}

	req := dto.GetJobApplicationByIDRequest{
		ID:     appID,
		UserID: userID,
	}

	application, err := h.service.GetApplicationByID(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		} else if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "You are not authorized to view this application"})
		} else {
			log.Printf("GetApplicationByID: Error fetching application %s: %v", appID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve application"})
		}
		return
	}

	appResponse := MapJobApplicationModelToResponse(application)
	c.JSON(http.StatusOK, appResponse)
}

// ListApplicationsByContractor godoc
//	@Summary		List applications submitted by the authenticated user
//	@Description	Retrieves a list of job applications submitted by the currently authenticated user (contractor). Supports pagination.
//	@Tags			job_applications
//	@Accept			json
//	@Produce		json
//	@Param			limit	query		int							false	"Pagination limit"	default(10)
//	@Param			offset	query		int							false	"Pagination offset"	default(0)
//	@Success		200		{array}		dto.JobApplicationResponse	"Successfully retrieved list of applications"
//	@Failure		400		{object}	map[string]string			"Bad Request - Invalid query parameters"
//	@Failure		401		{object}	map[string]string			"Unauthorized"
//	@Failure		500		{object}	map[string]string			"Internal Server Error"
//	@Router			/applications/my [get] // Example route
//	@Security		BearerAuth
func (h *JobApplicationHandler) ListApplicationsByContractor(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("ListApplicationsByContractor: Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req dto.ListJobApplicationsByContractorRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters: " + err.Error()})
		return
	}
	req.ContractorID = userID // Set the contractor ID from context

	if err := h.validator.Struct(req); err != nil {
		validationErrors := FormatValidationErrors(err.(validator.ValidationErrors))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": validationErrors})
		return
	}
	// Ensure defaults if not provided by binding
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	applications, err := h.service.ListApplicationsByContractor(c.Request.Context(), &req)
	if err != nil {
		log.Printf("ListApplicationsByContractor: Error listing applications for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve applications"})
		return
	}

	appResponses := make([]dto.JobApplicationResponse, 0, len(applications))
	for _, app := range applications {
		appResponses = append(appResponses, MapJobApplicationModelToResponse(app))
	}

	c.JSON(http.StatusOK, appResponses)
}

// ListApplicationsByJob godoc
//	@Summary		List applications for a specific job
//	@Description	Retrieves a list of applications for a specific job. Only allowed for the employer who posted the job. Supports pagination.
//	@Tags			job_applications
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string						true	"Job ID"			Format(uuid)
//	@Param			limit	query		int							false	"Pagination limit"	default(10)
//	@Param			offset	query		int							false	"Pagination offset"	default(0)
//	@Success		200		{array}		dto.JobApplicationResponse	"Successfully retrieved list of applications"
//	@Failure		400		{object}	map[string]string			"Bad Request - Invalid Job ID or query parameters"
//	@Failure		401		{object}	map[string]string			"Unauthorized"
//	@Failure		403		{object}	map[string]string			"Forbidden - User is not the employer for this job"
//	@Failure		404		{object}	map[string]string			"Not Found - Job not found"
//	@Failure		500		{object}	map[string]string			"Internal Server Error"
//	@Router			/jobs/{id}/applications [get] // Example route
//	@Security		BearerAuth
func (h *JobApplicationHandler) ListApplicationsByJob(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("ListApplicationsByJob: Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	jobIDStr := c.Param("id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID format"})
		return
	}

	var req dto.ListJobApplicationsByJobRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters: " + err.Error()})
		return
	}
	req.JobID = jobID
	req.UserID = userID // Pass UserID for authorization check in service

	if err := h.validator.Struct(req); err != nil {
		validationErrors := FormatValidationErrors(err.(validator.ValidationErrors))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": validationErrors})
		return
	}
	// Ensure defaults if not provided by binding
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	applications, err := h.service.ListApplicationsByJob(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		} else if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "You are not authorized to view applications for this job"})
		} else {
			log.Printf("ListApplicationsByJob: Error listing applications for job %s: %v", jobID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve applications"})
		}
		return
	}

	appResponses := make([]dto.JobApplicationResponse, 0, len(applications))
	for _, app := range applications {
		appResponses = append(appResponses, MapJobApplicationModelToResponse(app))
	}

	c.JSON(http.StatusOK, appResponses)
}

// AcceptApplication godoc
//	@Summary		Accept a job application
//	@Description	Allows the employer to accept a 'Waiting' application. This assigns the contractor to the job, changes the job state to 'Ongoing', and rejects other pending applications for the same job.
//	@Tags			job_applications
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string				true	"Application ID"	Format(uuid)
//	@Success		200	{object}	dto.JobResponse		"Application accepted, job updated"
//	@Failure		400	{object}	map[string]string	"Bad Request - Invalid ID format"
//	@Failure		401	{object}	map[string]string	"Unauthorized"
//	@Failure		403	{object}	map[string]string	"Forbidden - User is not the employer or job/application state is invalid"
//	@Failure		404	{object}	map[string]string	"Not Found - Application or Job not found"
//	@Failure		409	{object}	map[string]string	"Conflict - Job/Application state prevents acceptance"
//	@Failure		500	{object}	map[string]string	"Internal Server Error"
//	@Router			/applications/{id}/accept [patch]
//	@Security		BearerAuth
func (h *JobApplicationHandler) AcceptApplication(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("AcceptApplication: Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID format"})
		return
	}

	req := dto.AcceptApplicationRequest{
		ApplicationID: appID,
		UserID:        userID,
	}

	updatedJob, err := h.service.AcceptApplication(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()}) // Could be app or job not found
		} else if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: You are not the employer for this job"})
		} else if errors.Is(err, services.ErrInvalidState) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()}) // Use 409 Conflict for state issues
		} else {
			log.Printf("AcceptApplication: Error accepting application %s: %v", appID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to accept application"})
		}
		return
	}

	// Return the updated Job details
	jobResponse := MapJobModelToJobResponse(updatedJob)
	c.JSON(http.StatusOK, jobResponse)
}

// RejectApplication godoc
//	@Summary		Reject a job application
//	@Description	Allows the employer to reject a 'Waiting' application.
//	@Tags			job_applications
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string						true	"Application ID"	Format(uuid)
//	@Success		200	{object}	dto.JobApplicationResponse	"Application rejected successfully"
//	@Failure		400	{object}	map[string]string			"Bad Request - Invalid ID format"
//	@Failure		401	{object}	map[string]string			"Unauthorized"
//	@Failure		403	{object}	map[string]string			"Forbidden - User is not the employer or application state is invalid"
//	@Failure		404	{object}	map[string]string			"Not Found - Application or Job not found"
//	@Failure		409	{object}	map[string]string			"Conflict - Application state prevents rejection"
//	@Failure		500	{object}	map[string]string			"Internal Server Error"
//	@Router			/applications/{id}/reject [patch]
//	@Security		BearerAuth
func (h *JobApplicationHandler) RejectApplication(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("RejectApplication: Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID format"})
		return
	}

	req := dto.RejectApplicationRequest{
		ApplicationID: appID,
		UserID:        userID,
	}

	updatedApp, err := h.service.RejectApplication(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()}) // Could be app or job not found
		} else if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: You are not the employer for this job"})
		} else if errors.Is(err, services.ErrInvalidState) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()}) // Use 409 Conflict for state issues
		} else {
			log.Printf("RejectApplication: Error rejecting application %s: %v", appID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject application"})
		}
		return
	}

	appResponse := MapJobApplicationModelToResponse(updatedApp)
	c.JSON(http.StatusOK, appResponse)
}

// WithdrawApplication godoc
//	@Summary		Withdraw a job application
//	@Description	Allows the applicant (contractor) to withdraw their 'Waiting' application.
//	@Tags			job_applications
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string						true	"Application ID"	Format(uuid)
//	@Success		200	{object}	dto.JobApplicationResponse	"Application withdrawn successfully"
//	@Failure		400	{object}	map[string]string			"Bad Request - Invalid ID format"
//	@Failure		401	{object}	map[string]string			"Unauthorized"
//	@Failure		403	{object}	map[string]string			"Forbidden - User is not the applicant or application state is invalid"
//	@Failure		404	{object}	map[string]string			"Not Found - Application not found"
//	@Failure		409	{object}	map[string]string			"Conflict - Application state prevents withdrawal"
//	@Failure		500	{object}	map[string]string			"Internal Server Error"
//	@Router			/applications/{id}/withdraw [patch]
//	@Security		BearerAuth
func (h *JobApplicationHandler) WithdrawApplication(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		log.Printf("WithdrawApplication: Error getting user ID from context: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	appIDStr := c.Param("id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid application ID format"})
		return
	}

	req := dto.WithdrawApplicationRequest{
		ApplicationID: appID,
		UserID:        userID,
	}

	updatedApp, err := h.service.WithdrawApplication(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		} else if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: You are not the applicant for this application"})
		} else if errors.Is(err, services.ErrInvalidState) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()}) // Use 409 Conflict for state issues
		} else {
			log.Printf("WithdrawApplication: Error withdrawing application %s: %v", appID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to withdraw application"})
		}
		return
	}

	appResponse := MapJobApplicationModelToResponse(updatedApp)
	c.JSON(http.StatusOK, appResponse)
}
