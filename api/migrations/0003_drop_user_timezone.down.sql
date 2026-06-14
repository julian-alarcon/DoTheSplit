-- Restore the nullable per-user timezone override column. Data is not
-- recoverable on downgrade; the column comes back empty (NULL = device zone).
ALTER TABLE users ADD COLUMN timezone TEXT;
