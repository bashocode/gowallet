-- Drop indexes for transactions table
DROP INDEX idx_transactions_sender_user_id_created_at ON transactions;
DROP INDEX idx_transactions_sender_wallet_created_at ON transactions;
DROP INDEX idx_transactions_receiver_wallet_created_at ON transactions;
DROP INDEX idx_transactions_status ON transactions;
DROP INDEX idx_transactions_status_created_at ON transactions;
-- Drop indexes for outbox_events table
DROP INDEX idx_outbox_events_status_created_at ON outbox_events;
DROP INDEX idx_outbox_events_aggregate_id_created_at ON outbox_events;

