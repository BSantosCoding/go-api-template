CREATE TYPE job_application_state AS ENUM ('Waiting', 'Accepted', 'Rejected', 'Withdrawn');

CREATE TABLE job_application (
    id UUID PRIMARY KEY,
    contractor_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    state job_application_state NOT NULL DEFAULT 'Waiting',
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT unique_application UNIQUE (job_id, contractor_id)
);

-- Index foreign key
CREATE INDEX idx_job_application_job_id ON job_application(job_id);
CREATE INDEX idx_job_application_user_id ON job_application(contractor_id);

-- Trigger for updated_at timestamp
CREATE TRIGGER set_job_application_updated_at
BEFORE UPDATE ON job_application
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();
