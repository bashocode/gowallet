ALTER TABLE outbox_events
    DROP INDEX idx_outbox_aggregate_id,
    DROP INDEX idx_outbox_status_created,
    DROP COLUMN last_error,
    DROP COLUMN aggregate_id;

ALTER TABLE transactions
    DROP INDEX idx_transaction_receiver_email,
    DROP INDEX idx_transaction_type,
    DROP COLUMN updated_at,
    DROP COLUMN external_ewallet,
    DROP COLUMN currency,
    DROP COLUMN receiver_email,
    DROP COLUMN type,
    MODIFY COLUMN receiver_wallet_id VARCHAR(36) NOT NULL;
