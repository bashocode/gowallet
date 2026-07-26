-- Drop indexes for ledger_entries table
DROP INDEX IF EXISTS idx_ledger_entries_wallet_id_created_at ON ledger_entries;
DROP INDEX IF EXISTS idx_ledger_entries_wallet_id_entry_type ON ledger_entries;
DROP INDEX IF EXISTS idx_ledger_entries_created_at ON ledger_entries;
