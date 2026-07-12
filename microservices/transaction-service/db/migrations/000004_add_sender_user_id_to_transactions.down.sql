ALTER TABLE transactions
    DROP INDEX idx_transactions_sender_user_id,
    DROP COLUMN sender_user_id;
