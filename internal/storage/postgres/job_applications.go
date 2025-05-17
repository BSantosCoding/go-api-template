package postgres

import (
	"context"
	"fmt"
	"log"

	"go-api-template/ent"
	"go-api-template/ent/jobapplication"
	"go-api-template/internal/storage"
	"go-api-template/internal/transport/dto"

	"github.com/google/uuid"
)

// JobApplicationRepo implements the storage.JobApplicationRepository interface using Ent.
type JobApplicationRepo struct {
	client *ent.Client
}

// NewJobApplicationRepo creates a new JobApplicationRepo.
func NewJobApplicationRepo(client *ent.Client) *JobApplicationRepo {
	return &JobApplicationRepo{client: client}
}

func (r *JobApplicationRepo) WithTx(tx *ent.Tx) storage.JobApplicationRepository {
	return &JobApplicationRepo{client: tx.Client()}
}

// Compile-time check to ensure JobApplicationRepo implements JobApplicationRepository
var _ storage.JobApplicationRepository = (*JobApplicationRepo)(nil)

func (r *JobApplicationRepo) Create(ctx context.Context, req *dto.CreateJobApplicationRequest) (*ent.JobApplication, error) {
	createdApp, err := r.client.JobApplication.Create().
		SetContractorID(req.ContractorID).
		SetJobID(req.JobID).
		SetState(jobapplication.StateWaiting).
		Save(ctx)

	if err != nil {
		if ent.IsConstraintError(err) {
			log.Printf("Error creating job application (constraint violation): %v\n", err)
			return nil, fmt.Errorf("failed to create job application: unique constraint or foreign key violation: %w", storage.ErrConflict)
		}
		log.Printf("Error creating job application: %v\n", err)
		return nil, fmt.Errorf("failed to create job application: %w", err)
	}

	log.Printf("Job application created successfully with ID: %s", createdApp.ID)
	return createdApp, nil
}

func (r *JobApplicationRepo) GetByID(ctx context.Context, req *dto.GetJobApplicationByIDRequest) (*ent.JobApplication, error) {
	app, err := r.client.JobApplication.Get(ctx, req.ID)

	if err != nil {
		if ent.IsNotFound(err) {
			log.Printf("Job application not found with ID: %s\n", req.ID)
			return nil, storage.ErrNotFound
		}
		log.Printf("Error retrieving job application by ID %s: %v\n", req.ID, err)
		return nil, fmt.Errorf("failed to get job application by ID %s: %w", req.ID, err)
	}

	return app, nil
}

func (r *JobApplicationRepo) ListByContractor(ctx context.Context, req *dto.ListJobApplicationsByContractorRequest) ([]*ent.JobApplication, error) {
	apps, err := r.client.JobApplication.Query().
		Where(jobapplication.ContractorID(req.ContractorID)).
		Order(ent.Desc(jobapplication.FieldCreatedAt)).
		Limit(req.Limit).
		Offset(req.Offset).
		All(ctx)

	if err != nil {
		log.Printf("Error querying job applications by contractor ID %s: %v\n", req.ContractorID, err)
		return nil, fmt.Errorf("failed to list job applications by contractor: %w", err)
	}

	return apps, nil
}

func (r *JobApplicationRepo) ListByJob(ctx context.Context, req *dto.ListJobApplicationsByJobRequest) ([]*ent.JobApplication, error) {
	apps, err := r.client.JobApplication.Query().
		Where(jobapplication.JobID(req.JobID)).
		Order(ent.Desc(jobapplication.FieldCreatedAt)).
		Limit(req.Limit).
		Offset(req.Offset).
		All(ctx)

	if err != nil {
		log.Printf("Error querying job applications by job ID %s: %v\n", req.JobID, err)
		return nil, fmt.Errorf("failed to list job applications by job: %w", err)
	}

	return apps, nil
}

func (r *JobApplicationRepo) GetByJobAndContractor(ctx context.Context, req *dto.GetByJobAndContractorRequest) ([]*ent.JobApplication, error) {
	apps, err := r.client.JobApplication.Query().
		Where(jobapplication.JobID(req.JobID), jobapplication.ContractorID(req.UserID)).
		Order(ent.Desc(jobapplication.FieldCreatedAt)).
		All(ctx)

	if err != nil {
		log.Printf("Error querying job applications by job ID %s: %v\n", req.JobID, err)
		return nil, fmt.Errorf("failed to list job applications by job: %w", err)
	}

	return apps, nil
}

func (r *JobApplicationRepo) UpdateState(ctx context.Context, req *dto.UpdateJobApplicationStateRequest) (*ent.JobApplication, error) {
	updatedApp, err := r.client.JobApplication.UpdateOneID(req.ID).
		SetState(jobapplication.State(req.State)).
		Save(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			log.Printf("Job application not found for state update with ID: %s\n", req.ID)
			return nil, storage.ErrNotFound
		}
		log.Printf("Error updating job application state for ID %s: %v\n", req.ID, err)
		return nil, fmt.Errorf("failed to update job application state: %w", err)
	}

	return updatedApp, nil
}

func (r *JobApplicationRepo) UpdateStateByJobID(ctx context.Context, jobID uuid.UUID, newState jobapplication.State, excludeApplicationID *uuid.UUID) error {
	updateQuery := r.client.JobApplication.Update().
		Where(
			jobapplication.JobID(jobID),
			jobapplication.StateEQ(jobapplication.StateWaiting),
		).
		SetState(jobapplication.State(newState))

	if excludeApplicationID != nil {
		updateQuery = updateQuery.Where(jobapplication.IDNEQ(*excludeApplicationID))
	}

	err := updateQuery.Exec(ctx)
	if err != nil {
		log.Printf("Error updating states for job applications of job %s: %v\n", jobID, err)
		return fmt.Errorf("failed to update job application states for job %s: %w", jobID, err)
	}

	log.Printf("Updated job applications for job %s to state %s", jobID, newState)
	return nil
}

func (r *JobApplicationRepo) Delete(ctx context.Context, req *dto.DeleteJobApplicationRequest) error {
	err := r.client.JobApplication.DeleteOneID(req.ID).Exec(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			log.Printf("Job application not found for deletion with ID: %s\n", req.ID)
			return storage.ErrNotFound
		}
		log.Printf("Error deleting job application with ID %s: %v\n", req.ID, err)
		return fmt.Errorf("failed to delete job application: %w", err)
	}

	log.Printf("Job application deleted successfully with ID: %s", req.ID)
	return nil
}
