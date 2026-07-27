DROP INDEX idx_users_deleted_at ON users;
DROP INDEX idx_users_deleted_at_created_at ON users;
DROP INDEX idx_otp_codes_code_expires_at ON otp_codes;
DROP INDEX idx_otp_codes_expires_at_used ON otp_codes;
DROP INDEX idx_notification_outbox_events_status_created_at ON notification_outbox_events;
