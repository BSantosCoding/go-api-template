-- Remove the password hash column
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
