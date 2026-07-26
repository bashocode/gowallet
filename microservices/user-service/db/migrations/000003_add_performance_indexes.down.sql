-- Drop indexes for users table
DROP INDEX IF EXISTS idx_users_deleted_at ON users;
DROP INDEX IF EXISTS idx_users_deleted_at_created_at ON users;

-- Drop indexes for otp_codes table
DROP INDEX IF EXISTS idx_otp_codes_code_expires_at ON otp_codes;
DROP INDEX IF EXISTS idx_otp_codes_expires_at_used ON otp_codes;

-- Drop indexes for outbox_events table
DROP INDEX IF EXISTS idx_outbox_events_status_created_at ON outbox_events;
