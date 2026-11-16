-- ========================================================
-- 000002_add_password_changed_at.down.sql
-- Rollback password_changed_at column
-- ========================================================

ALTER TABLE users DROP COLUMN IF EXISTS password_changed_at;

