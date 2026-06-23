ALTER TABLE users ADD COLUMN is_verified BOOLEAN NOT NULL DEFAULT false AFTER avatar_url;
