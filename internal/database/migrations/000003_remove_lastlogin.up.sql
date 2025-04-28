-- db/migrations/TIMESTAMP_remove_last_login_from_users.up.sql
ALTER TABLE users
DROP COLUMN last_login;
