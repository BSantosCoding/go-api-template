-- Add the password hash column required for authentication
-- Using TEXT for potentially longer hashes, VARCHAR(255) is also common
ALTER TABLE users
ADD COLUMN password_hash TEXT NOT NULL DEFAULT 'DISABLED';

ALTER TABLE users
ALTER COLUMN password_hash DROP DEFAULT;