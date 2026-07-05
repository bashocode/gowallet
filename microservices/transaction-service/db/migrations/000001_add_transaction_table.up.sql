CREATE TABLE transactions (
    id VARCHAR(36) PRIMARY KEY,
    sender_wallet_id VARCHAR(36) NULL,
    receiver_wallet_id VARCHAR(36) NOT NULL,
    amount DECIMAL(15, 2) NOT NULL,
    description TEXT NULL,
    idempotency_key VARCHAR(100) UNIQUE NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- default 'pending' untuk koordinasi Saga
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_sender_wallet_id (sender_wallet_id),
    INDEX idx_receiver_wallet_id (receiver_wallet_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
