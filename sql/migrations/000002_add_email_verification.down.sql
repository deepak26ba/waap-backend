-- ========================================================
-- 000002_add_email_verification.down.sql
-- ========================================================

DROP TABLE IF EXISTS email_verification_tokens;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
