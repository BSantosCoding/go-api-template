// /home/bsant/testing/go-api-template/internal/api/routes/invoice_routes.go
package routes

import (
	"go-api-template/internal/api/handlers"

	"github.com/gin-gonic/gin"
)

// RegisterInvoiceRoutes registers all routes related to invoices.
func RegisterInvoiceRoutes(
	rg *gin.RouterGroup, 
	invoiceHandler handlers.InvoiceHandlerInterface, 
	authMiddleware gin.HandlerFunc,
) {
	// Create a group for general invoice actions (e.g., /api/v1/invoices)
	invoices := rg.Group("/invoices")
	invoices.Use(authMiddleware) // Apply auth middleware
	{
		invoices.POST("/", invoiceHandler.CreateInvoice)       // Create a new invoice (handler calculates value/interval)
		invoices.GET("/:id", invoiceHandler.GetInvoiceByID)    // Get a specific invoice by ID
		invoices.PATCH("/:id/state", invoiceHandler.UpdateInvoiceState) // Update the state of an invoice
		invoices.DELETE("/:id", invoiceHandler.DeleteInvoice)  // Delete an invoice
	}

	jobsGroupForInvoices := rg.Group("/jobs")
	jobsGroupForInvoices.Use(authMiddleware)
	{
		jobsGroupForInvoices.GET("/:id/invoices", invoiceHandler.ListInvoicesByJob)
	}
}

