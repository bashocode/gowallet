CREATE INDEX IF NOT EXISTS idx_ledger_entries_wallet_id_created_at ON ledger_entries(wallet_id, created_at);

CREATE INDEX IF NOT EXISTS idx_ledger_entries_wallet_id_entry_type ON ledger_entries(wallet_id, entry_type);

CREATE INDEX IF NOT EXISTS idx_ledger_entries_created_at ON ledger_entries(created_at);
