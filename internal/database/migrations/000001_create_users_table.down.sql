-- Down migration for reverting the users table creation
DROP TRIGGER IF EXISTS set_users_timestamp ON users;
DROP FUNCTION IF EXISTS trigger_set_timestamp(); -- Drop function only if it's not used by other tables
DROP INDEX IF EXISTS idx_users_email;
DROP TABLE IF EXISTS users;
