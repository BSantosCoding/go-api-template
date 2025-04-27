-- Up migration for creating the items table
CREATE TABLE IF NOT EXISTS items (
    id TEXT PRIMARY KEY, -- Using TEXT for potential UUIDs
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price NUMERIC(10, 2) NOT NULL CHECK (price >= 0), -- Use NUMERIC for currency
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Re-use or re-create the trigger function if needed (if dropped in previous down migration)
CREATE OR REPLACE FUNCTION trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to update updated_at timestamp automatically
CREATE TRIGGER set_items_timestamp
BEFORE UPDATE ON items
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();
