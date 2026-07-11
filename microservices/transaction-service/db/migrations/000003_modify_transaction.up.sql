ALTER TABLE transactions
    MODIFY COLUMN receiver_wallet_id VARCHAR(36) NULL,
    ADD COLUMN type VARCHAR(50) NOT NULL DEFAULT 'internal_transfer' AFTER id,
    ADD COLUMN receiver_email VARCHAR(150) NULL AFTER receiver_wallet_id,
    ADD COLUMN currency VARCHAR(10) NOT NULL DEFAULT 'IDR' AFTER amount,
    ADD COLUMN external_ewallet VARCHAR(50) NULL AFTER currency,
    ADD COLUMN updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER created_at,
    ADD INDEX idx_transaction_type (type),
    ADD INDEX idx_transaction_receiver_email (receiver_email);

ALTER TABLE outbox_events
    ADD COLUMN aggregate_id VARCHAR(100) NULL AFTER event_type,
    ADD COLUMN last_error TEXT NULL AFTER attempts,
    ADD INDEX idx_outbox_status_created (status, created_at),
    ADD INDEX idx_outbox_aggregate_id (aggregate_id);
