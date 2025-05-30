package handlers

import "github.com/gin-gonic/gin"

// UserHandlerInterface defines the methods needed by the user routes.
type UserHandlerInterface interface {
	GetUserByID(c *gin.Context)
	Login(c *gin.Context)
	GetUsers(c *gin.Context)
	Register(c *gin.Context)
	UpdateUser(c *gin.Context)
	DeleteUser(c *gin.Context)
	Refresh(c *gin.Context)
	Logout(c *gin.Context)
}

// JobHandlerInterface defines the methods needed by the job routes.
type JobHandlerInterface interface {
	CreateJob(c *gin.Context)
	GetJobByID(c *gin.Context)
	ListAvailableJobs(c *gin.Context)
	ListEmployerJobs(c *gin.Context)  // Handler for employer's own jobs
	ListContractorJobs(c *gin.Context) // Handler for contractor's own jobs
	UpdateJobDetails(c *gin.Context)   // For Rate/Duration by Employer (before assignment)
	UpdateJobState(c *gin.Context)
	DeleteJob(c *gin.Context)
}

// JobApplicationHandlerInterface defines methods for job application routes.
type JobApplicationHandlerInterface interface {
	ApplyToJob(c *gin.Context)
	GetApplicationByID(c *gin.Context)
	ListApplicationsByContractor(c *gin.Context)
	ListApplicationsByJob(c *gin.Context)
	AcceptApplication(c *gin.Context)
	RejectApplication(c *gin.Context)
	WithdrawApplication(c *gin.Context)
}

// InvoiceHandlerInterface defines the methods needed by the invoice routes.
type InvoiceHandlerInterface interface {
	CreateInvoice(c *gin.Context) // Will handle calculation logic
	GetInvoiceByID(c *gin.Context)
	ListInvoicesByJob(c *gin.Context)
	UpdateInvoiceState(c *gin.Context)
	DeleteInvoice(c *gin.Context)
}

// Ensure handlers implements the interface (compile-time check)
var _ UserHandlerInterface = (*UserHandler)(nil)
var _ JobHandlerInterface = (*JobHandler)(nil)
var _ JobApplicationHandlerInterface = (*JobApplicationHandler)(nil) // Add this when handler is created
var _ InvoiceHandlerInterface = (*InvoiceHandler)(nil)