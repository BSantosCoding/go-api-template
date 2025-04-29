CREATE TYPE invoice_state AS ENUM ('Waiting', 'Complete');

CREATE TABLE invoices (
    id UUID PRIMARY KEY,
    value NUMERIC(12, 2) NOT NULL CHECK (value >= 0),
    state invoice_state NOT NULL DEFAULT 'Waiting',
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    interval_number INTEGER NOT NULL CHECK (interval_number > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT unique_job_interval UNIQUE (job_id, interval_number)
);

-- Index foreign key
CREATE INDEX idx_invoices_job_id ON invoices(job_id);

-- Trigger for updated_at timestamp
CREATE TRIGGER set_invoices_updated_at
BEFORE UPDATE ON invoices
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();
