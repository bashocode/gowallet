CREATE TABLE IF NOT EXISTS processed_notifications (
    event_id VARCHAR(100) PRIMARY KEY,
    processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_processed_at ON processed_notifications(processed_at);
