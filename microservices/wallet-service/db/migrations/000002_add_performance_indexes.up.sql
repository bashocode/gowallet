CREATE INDEX idx_wallets_deleted_at ON wallets(deleted_at);
CREATE INDEX idx_wallets_status_deleted_at ON wallets(status, deleted_at);
CREATE INDEX idx_wallets_deleted_at_created_at ON wallets(deleted_at, created_at);
