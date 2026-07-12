ALTER TABLE transactions
    ADD COLUMN sender_user_id VARCHAR(36) NULL AFTER type,
    ADD INDEX idx_transactions_sender_user_id (sender_user_id);
