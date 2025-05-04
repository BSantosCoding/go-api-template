package routes

import (
	"go-api-template/internal/api/handlers"

	"github.com/gin-gonic/gin"
)

// RegisterJobApplicationRoutes registers all routes related to job applications.
func RegisterJobApplicationRoutes(
	rg *gin.RouterGroup,
	jobAppHandler handlers.JobApplicationHandlerInterface, // Use interface
	authMiddleware gin.HandlerFunc,
) {
	// Group for actions related to a specific job
	jobsGroup := rg.Group("/jobs")
	jobsGroup.Use(authMiddleware)
	{
		// Apply for a specific job
		jobsGroup.POST("/:job_id/apply", jobAppHandler.ApplyToJob)
		// List applications for a specific job (Employer view)
		jobsGroup.GET("/:job_id/applications", jobAppHandler.ListApplicationsByJob)
	}

	// Group for actions related to applications themselves
	appsGroup := rg.Group("/applications")
	appsGroup.Use(authMiddleware)
	{
		appsGroup.GET("/my", jobAppHandler.ListApplicationsByContractor) // List applications submitted by the current user
		appsGroup.GET("/:id", jobAppHandler.GetApplicationByID)
		appsGroup.PATCH("/:id/accept", jobAppHandler.AcceptApplication)
		appsGroup.PATCH("/:id/reject", jobAppHandler.RejectApplication)
		appsGroup.PATCH("/:id/withdraw", jobAppHandler.WithdrawApplication)
		// Note: Delete route is omitted for now, favoring Withdraw/Reject logic.
	}
}