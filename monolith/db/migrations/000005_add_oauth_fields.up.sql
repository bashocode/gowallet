ALTER TABLE users MODIFY password_hash VARCHAR(255) NULL;
ALTER TABLE users ADD COLUMN oauth_provider VARCHAR(50) NULL AFTER email;
ALTER TABLE users ADD COLUMN oauth_id VARCHAR(255) NULL AFTER oauth_provider;
ALTER TABLE users ADD UNIQUE KEY unique_oauth (oauth_provider, oauth_id);
