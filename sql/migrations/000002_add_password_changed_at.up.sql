-- ========================================================
-- 000002_add_password_changed_at.up.sql
-- Add password_changed_at column for session revocation
-- ========================================================

ALTER TABLE users ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMPTZ;

