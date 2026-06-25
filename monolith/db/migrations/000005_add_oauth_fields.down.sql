ALTER TABLE users DROP INDEX unique_oauth;
ALTER TABLE users DROP COLUMN oauth_id;
ALTER TABLE users DROP COLUMN oauth_provider;
ALTER TABLE users MODIFY password_hash VARCHAR(255) NOT NULL;
