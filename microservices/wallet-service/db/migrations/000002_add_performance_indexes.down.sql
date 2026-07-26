-- Drop indexes for wallets table
DROP INDEX IF EXISTS idx_wallets_deleted_at ON wallets;
DROP INDEX IF EXISTS idx_wallets_status_deleted_at ON wallets;
DROP INDEX IF EXISTS idx_wallets_deleted_at_created_at ON wallets;
