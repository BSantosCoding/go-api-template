// /home/bsant/testing/go-api-template/internal/api/routes/job_routes.go
package routes

import (
	"go-api-template/internal/api/handlers"

	"github.com/gin-gonic/gin"
)

// RegisterJobRoutes registers all routes related to jobs.
// It applies the provided authentication middleware to all job routes.
func RegisterJobRoutes(
	rg *gin.RouterGroup, // Base group (e.g., /api/v1)
	jobHandler handlers.JobHandlerInterface, // Use interface
	authMiddleware gin.HandlerFunc,
) {
	jobs := rg.Group("/jobs")
	jobs.Use(authMiddleware) // Apply auth middleware to all job routes
	{
		jobs.POST("/", jobHandler.CreateJob)             // Create a new job posting
		jobs.GET("/available", jobHandler.ListAvailableJobs) // List jobs available for contractors
		jobs.GET("/my/employer", jobHandler.ListEmployerJobs) // List jobs posted by the authenticated employer
		jobs.GET("/my/contractor", jobHandler.ListContractorJobs) // List jobs taken by the authenticated contractor
		jobs.GET("/:id", jobHandler.GetJobByID)          // Get a specific job by ID
		jobs.PATCH("/:id/details", jobHandler.UpdateJobDetails)     // Update Rate/Duration
		jobs.PUT("/:id/contractor", jobHandler.AssignContractor)    // Assign Contractor (PUT is suitable for replacing/setting the contractor)
		jobs.DELETE("/:id/contractor", jobHandler.UnassignContractor) // Unassign Contractor (DELETE is suitable)
		jobs.PATCH("/:id/state", jobHandler.UpdateJobState) 
		jobs.DELETE("/:id", jobHandler.DeleteJob)        // Delete a job
	}
}

