package postgres

import (
	"context"
	"fmt"
	"log"

	"go-api-template/ent"
	"go-api-template/ent/invoice"
	"go-api-template/internal/storage"
	"go-api-template/internal/transport/dto"
)

type InvoiceRepo struct {
	client *ent.Client
}

func NewInvoiceRepo(client *ent.Client) *InvoiceRepo {
	return &InvoiceRepo{client: client}
}

func (r *InvoiceRepo) WithTx(tx *ent.Tx) storage.InvoiceRepository {
	return &InvoiceRepo{client: tx.Client()}
}

var _ storage.InvoiceRepository = (*InvoiceRepo)(nil)

func (r *InvoiceRepo) Create(ctx context.Context, invoiceEntity *ent.Invoice) (*ent.Invoice, error) {
	createdInvoice, err := r.client.Invoice.Create().
		SetValue(invoiceEntity.Value).
		SetState(invoice.State(invoiceEntity.State)).
		SetJobID(invoiceEntity.JobID).
		SetIntervalNumber(invoiceEntity.IntervalNumber).
		Save(ctx)

	if err != nil {
		if ent.IsConstraintError(err) {
			log.Printf("Error creating invoice (constraint violation): %v\n", err)
			return nil, fmt.Errorf("failed to create invoice: unique constraint violation: %w", storage.ErrConflict)
		}
		if ent.IsNotFound(err) {
			// This case might happen if JobID doesn't exist due to foreign key constraint.
			log.Printf("Error creating invoice (related entity not found): %v\n", err)
			return nil, fmt.Errorf("failed to create invoice: related entity not found: %w", storage.ErrConflict)
		}
		log.Printf("Error creating invoice: %v\n", err)
		return nil, fmt.Errorf("failed to save invoice: %w", err)
	}

	log.Printf("Invoice created successfully with ID: %s for Job ID: %s, Interval: %d",
		createdInvoice.ID, createdInvoice.JobID, createdInvoice.IntervalNumber)
	return createdInvoice, nil
}

func (r *InvoiceRepo) GetByID(ctx context.Context, req *dto.GetInvoiceByIDRequest) (*ent.Invoice, error) {
	invoice, err := r.client.Invoice.Get(ctx, req.ID)

	if err != nil {
		if ent.IsNotFound(err) {
			log.Printf("Invoice not found with ID: %s\n", req.ID)
			return nil, storage.ErrNotFound
		}
		log.Printf("Error retrieving invoice by ID %s: %v\n", req.ID, err)
		return nil, fmt.Errorf("failed to get invoice by ID %s: %w", req.ID, err)
	}

	return invoice, nil
}

func (r *InvoiceRepo) ListByJob(ctx context.Context, req *dto.ListInvoicesByJobRequest) ([]*ent.Invoice, error) {
	query := r.client.Invoice.Query().
		Where(invoice.JobID(req.JobID))

	if req.State != nil {
		query = query.Where(invoice.StateEQ(*req.State))
	}

	invoices, err := query.Order(ent.Asc(invoice.FieldIntervalNumber)).
		Limit(req.Limit).
		Offset(req.Offset).
		All(ctx)

	if err != nil {
		log.Printf("Error querying invoices by job %s: %v\n", req.JobID, err)
		return nil, fmt.Errorf("failed to query invoices by job: %w", err)
	}

	return invoices, nil
}

func (r *InvoiceRepo) UpdateState(ctx context.Context, req *dto.UpdateInvoiceStateRequest) (*ent.Invoice, error) {
	updatedInvoice, err := r.client.Invoice.UpdateOneID(req.ID).
		SetState(invoice.State(req.NewState)).
		Save(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			log.Printf("Invoice not found for state update with ID: %s\n", req.ID)
			return nil, storage.ErrNotFound
		}
		log.Printf("Error updating invoice state %s: %v\n", req.ID, err)
		return nil, fmt.Errorf("failed to update invoice state %s: %w", req.ID, err)
	}

	log.Printf("Invoice state updated successfully for ID: %s to %s", updatedInvoice.ID, updatedInvoice.State)
	return updatedInvoice, nil
}

func (r *InvoiceRepo) Delete(ctx context.Context, req *dto.DeleteInvoiceRequest) error {
	err := r.client.Invoice.DeleteOneID(req.ID).Exec(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			log.Printf("Invoice not found for deletion with ID: %s\n", req.ID)
			return storage.ErrNotFound
		}
		log.Printf("Error deleting invoice %s: %v\n", req.ID, err)
		return fmt.Errorf("failed to delete invoice %s: %w", req.ID, err)
	}

	log.Printf("Invoice deleted successfully: %s", req.ID)
	return nil
}

func (r *InvoiceRepo) GetMaxIntervalForJob(ctx context.Context, req *dto.GetMaxIntervalForJobRequest) (int, error) {
	maxInterval, err := r.client.Invoice.Query().
		Where(invoice.JobID(req.JobID)).
		Aggregate(ent.Max(invoice.FieldIntervalNumber)).
		Int(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return 0, nil
		}
		log.Printf("Error querying max interval for job %s: %v\n", req.JobID, err)
		return 0, fmt.Errorf("failed to query max interval number for job %s: %w", req.JobID, err)
	}

	return maxInterval, nil
}
