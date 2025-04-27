-- Down migration for reverting the items table creation
DROP TRIGGER IF EXISTS set_items_timestamp ON items;
-- Consider carefully if the function should be dropped if shared
-- DROP FUNCTION IF EXISTS trigger_set_timestamp();
DROP TABLE IF EXISTS items;
