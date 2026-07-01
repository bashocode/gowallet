-- New database for Auth Service
CREATE DATABASE IF NOT EXISTS `gowallet_auth`;

-- New database for User Service
CREATE DATABASE IF NOT EXISTS `gowallet_user`;

-- New database for Wallet Service
-- CREATE DATABASE IF NOT EXISTS `gowallet_wallet`;

-- New database for Transaction Service
-- CREATE DATABASE IF NOT EXISTS `gowallet_transaction`;

-- New database for Payment Service
-- CREATE DATABASE IF NOT EXISTS `gowallet_payment`;

-- Grant full privileges to user gowallet_user
GRANT ALL PRIVILEGES ON `gowallet_auth`.* TO 'gowallet_user'@'%';
GRANT ALL PRIVILEGES ON `gowallet_user`.* TO 'gowallet_user'@'%';
-- GRANT ALL PRIVILEGES ON `gowallet_wallet`.* TO 'gowallet_user'@'%';
-- GRANT ALL PRIVILEGES ON `gowallet_transaction`.* TO 'gowallet_user'@'%';
-- GRANT ALL PRIVILEGES ON `gowallet_payment`.* TO 'gowallet_user'@'%';
FLUSH PRIVILEGES;
