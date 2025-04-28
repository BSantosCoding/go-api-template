-- db/migrations/TIMESTAMP_remove_last_login_from_users.down.sql
ALTER TABLE users
ADD COLUMN last_login TIMESTAMPTZ NULL; -- Use the original type and nullability
