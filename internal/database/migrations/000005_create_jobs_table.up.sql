CREATE TYPE job_state AS ENUM ('Waiting', 'Ongoing', 'Complete', 'Archived');

CREATE TABLE jobs (
    id UUID PRIMARY KEY,
    rate NUMERIC(10, 2) NOT NULL CHECK (rate >= 0), -- Use NUMERIC for currency
    duration INTEGER NOT NULL CHECK (duration > 0), -- Assuming duration must be positive hours
    contractor_id UUID NULL REFERENCES users(id) ON DELETE SET NULL, -- Allow contractor deletion
    employer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- If employer is deleted, remove their job postings
    state job_state NOT NULL DEFAULT 'Waiting',
    invoice_interval INTEGER NOT NULL CHECK (invoice_interval > 0), -- Interval in hours
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index foreign keys for performance
CREATE INDEX idx_jobs_contractor_id ON jobs(contractor_id);
CREATE INDEX idx_jobs_employer_id ON jobs(employer_id);

-- Trigger for updated_at timestamp 
CREATE TRIGGER set_jobs_updated_at
BEFORE UPDATE ON jobs
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();
