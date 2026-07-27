CREATE INDEX idx_transactions_sender_user_id_created_at ON transactions(sender_user_id, created_at);
CREATE INDEX idx_transactions_sender_wallet_created_at ON transactions(sender_wallet_id, created_at);
CREATE INDEX idx_transactions_receiver_wallet_created_at ON transactions(receiver_wallet_id, created_at);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_status_created_at ON transactions(status, created_at);

CREATE INDEX idx_outbox_events_status_created_at ON outbox_events(status, created_at);
CREATE INDEX idx_outbox_events_aggregate_id_created_at ON outbox_events(aggregate_id, created_at);

