CREATE INDEX idx_users_deleted_at ON users(deleted_at);
CREATE INDEX idx_users_deleted_at_created_at ON users(deleted_at, created_at);

CREATE INDEX idx_otp_codes_code_expires_at ON otp_codes(code, expires_at);
CREATE INDEX idx_otp_codes_expires_at_used ON otp_codes(expires_at, used);

CREATE INDEX idx_notification_outbox_events_status_created_at ON notification_outbox_events(status, created_at);
