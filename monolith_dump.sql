-- MySQL dump 10.13  Distrib 8.0.46, for Linux (aarch64)
--
-- Host: localhost    Database: gowallet
-- ------------------------------------------------------
-- Server version	8.0.46

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!50503 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `ledger_entries`
--

DROP TABLE IF EXISTS `ledger_entries`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `ledger_entries` (
  `id` varchar(36) NOT NULL,
  `wallet_id` varchar(36) NOT NULL,
  `transaction_id` varchar(36) NOT NULL,
  `entry_type` varchar(10) NOT NULL,
  `amount` decimal(15,2) NOT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `wallet_id` (`wallet_id`),
  KEY `transaction_id` (`transaction_id`),
  CONSTRAINT `ledger_entries_ibfk_1` FOREIGN KEY (`wallet_id`) REFERENCES `wallets` (`id`),
  CONSTRAINT `ledger_entries_ibfk_2` FOREIGN KEY (`transaction_id`) REFERENCES `transactions` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ledger_entries`
--

LOCK TABLES `ledger_entries` WRITE;
/*!40000 ALTER TABLE `ledger_entries` DISABLE KEYS */;
INSERT INTO `ledger_entries` VALUES ('0e3d4807-70d1-4a05-a739-8f53a4579fc5','31e84205-93ab-43a7-aa5d-79d3adb8b8a2','20d2daff-a13b-42c3-8548-d575aac28c13','debit',20000.00,'2026-06-27 17:25:48'),('2e7cf3bf-cc6e-4bd5-9ad9-e7b3893a4d3d','31e84205-93ab-43a7-aa5d-79d3adb8b8a2','de13235b-5a96-4fef-b046-66e046551236','debit',50000.00,'2026-06-20 08:53:06'),('33ffc422-9656-4c9c-8b15-9cdce88ae817','31e84205-93ab-43a7-aa5d-79d3adb8b8a2','05be2233-d7cc-4204-97c1-ca73de08ccc3','credit',100000.00,'2026-06-27 17:24:56'),('3787f0b1-2237-41f9-97d9-ecced80d953a','1f97b30b-23a6-4dd8-9cca-06c1d7d75006','5c43a1d7-7bfa-4127-94f3-5e483b33ef3d','credit',10.00,'2026-06-27 09:47:43'),('64093c10-a463-4978-9680-34fdb4c3de1f','117e1866-999d-49db-ac96-b903502dcc1d','dd7bc5e8-7e68-4366-9601-3c9bd4b5d728','credit',10.00,'2026-06-27 09:54:46'),('7ada843c-6b51-409e-9a58-945fe4b9ec92','7e99ad82-bbf0-4e5e-9576-d0317877ba91','dd7bc5e8-7e68-4366-9601-3c9bd4b5d728','debit',10.00,'2026-06-27 09:54:46'),('7fdebcbf-2b69-430c-bb1d-1823f82c8daa','45065b58-3f04-4631-b430-d5f35c2c771f','de13235b-5a96-4fef-b046-66e046551236','credit',50000.00,'2026-06-20 08:53:06'),('d94d3ec5-fc45-4940-a09f-f0414c9a1970','45065b58-3f04-4631-b430-d5f35c2c771f','f613f498-295c-4d18-8b7a-674e59951157','debit',50000.00,'2026-06-27 09:20:24'),('e93e433d-c517-4536-8729-225c80fdfd3a','7e99ad82-bbf0-4e5e-9576-d0317877ba91','199f145c-f227-4772-8a87-8cc09de4a9bf','credit',100.00,'2026-06-27 09:54:23'),('f2a60a77-9f0c-4dae-a21b-b116eaa9d310','4f3b0cac-985d-4563-9b55-2a4c58830621','5c43a1d7-7bfa-4127-94f3-5e483b33ef3d','debit',10.00,'2026-06-27 09:47:43'),('f3f09c78-8b7f-4eb7-9b70-ee25dd1964d7','45065b58-3f04-4631-b430-d5f35c2c771f','20d2daff-a13b-42c3-8548-d575aac28c13','credit',20000.00,'2026-06-27 17:25:48'),('f7648ced-5555-4c10-93a0-96d3ece3e39d','31e84205-93ab-43a7-aa5d-79d3adb8b8a2','f613f498-295c-4d18-8b7a-674e59951157','credit',50000.00,'2026-06-27 09:20:24');
/*!40000 ALTER TABLE `ledger_entries` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `otp_codes`
--

DROP TABLE IF EXISTS `otp_codes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `otp_codes` (
  `id` varchar(36) NOT NULL,
  `user_id` varchar(36) NOT NULL,
  `code` varchar(6) NOT NULL,
  `type` varchar(36) NOT NULL,
  `expires_at` timestamp NOT NULL,
  `used` tinyint(1) NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `user_id` (`user_id`),
  CONSTRAINT `otp_codes_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `otp_codes`
--

LOCK TABLES `otp_codes` WRITE;
/*!40000 ALTER TABLE `otp_codes` DISABLE KEYS */;
/*!40000 ALTER TABLE `otp_codes` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `refresh_tokens`
--

DROP TABLE IF EXISTS `refresh_tokens`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `refresh_tokens` (
  `id` varchar(36) NOT NULL,
  `user_id` varchar(36) NOT NULL,
  `token` varchar(500) NOT NULL,
  `expires_at` timestamp NOT NULL,
  `revoked` tinyint(1) NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `token` (`token`),
  KEY `user_id` (`user_id`),
  CONSTRAINT `refresh_tokens_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `refresh_tokens`
--

LOCK TABLES `refresh_tokens` WRITE;
/*!40000 ALTER TABLE `refresh_tokens` DISABLE KEYS */;
INSERT INTO `refresh_tokens` VALUES ('1307ce9f-9739-47a7-b033-1915bdc498d4','00bf8263-aa6e-4c8c-bed4-a1be16642cda','eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMDBiZjgyNjMtYWE2ZS00YzhjLWJlZDQtYTFiZTE2NjQyY2RhIiwiZW1haWwiOiJmcmllcmVuQGV4YW1wbGUuY29tIiwicm9sZSI6IiIsImV4cCI6MTc4MzE1MTgwMiwiaWF0IjoxNzgyNTQ3MDAyLCJqdGkiOiIwMGJmODI2My1hYTZlLTRjOGMtYmVkNC1hMWJlMTY2NDJjZGEtMjAyNjA2MjcxNDU2NDIifQ.dr_xo_yVQojK08iTi5zuMjB4WiWzVM4YSaWeMRTODeI','2026-07-04 07:56:43',0,'2026-06-27 07:56:42','2026-06-27 07:56:42'),('4f2b971b-2b88-4710-9aba-f699b2077eb2','00bf8263-aa6e-4c8c-bed4-a1be16642cda','eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMDBiZjgyNjMtYWE2ZS00YzhjLWJlZDQtYTFiZTE2NjQyY2RhIiwiZW1haWwiOiJmcmllcmVuQGV4YW1wbGUuY29tIiwicm9sZSI6ImFkbWluIiwiZXhwIjoxNzgzMTUxOTM0LCJpYXQiOjE3ODI1NDcxMzQsImp0aSI6IjAwYmY4MjYzLWFhNmUtNGM4Yy1iZWQ0LWExYmUxNjY0MmNkYS0yMDI2MDYyNzE0NTg1NCJ9.tW5UGzPe0T-4HzylNYDfy_HsGfarqYGVDe2uLpCFwNI','2026-07-04 07:58:54',0,'2026-06-27 07:58:54','2026-06-27 07:58:54'),('550fc406-cc4c-4129-ace3-124042a56fd9','00bf8263-aa6e-4c8c-bed4-a1be16642cda','eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMDBiZjgyNjMtYWE2ZS00YzhjLWJlZDQtYTFiZTE2NjQyY2RhIiwiZW1haWwiOiJmcmllcmVuQGV4YW1wbGUuY29tIiwicm9sZSI6ImFkbWluIiwiZXhwIjoxNzgzMTUyMjM4LCJpYXQiOjE3ODI1NDc0MzgsImp0aSI6IjAwYmY4MjYzLWFhNmUtNGM4Yy1iZWQ0LWExYmUxNjY0MmNkYS0yMDI2MDYyNzE1MDM1OCJ9.z8IIYq5NdizHfU87R65d5l6aSMNXfgngjj6g18ZF2-A','2026-07-04 08:03:58',0,'2026-06-27 08:03:58','2026-06-27 08:03:58'),('6e5b44aa-137c-468b-b9a2-8d307a307ab5','f287cc66-24b9-4f17-b29f-2da20e98d0db','eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiZjI4N2NjNjYtMjRiOS00ZjE3LWIyOWYtMmRhMjBlOThkMGRiIiwiZW1haWwiOiJGZXJuQGV4YW1wbGUuY29tIiwicm9sZSI6InVzZXIiLCJleHAiOjE3ODMxNTgyNDUsImlhdCI6MTc4MjU1MzQ0NSwianRpIjoiZjI4N2NjNjYtMjRiOS00ZjE3LWIyOWYtMmRhMjBlOThkMGRiLTIwMjYwNjI3MTY0NDA1In0.RMrLHikFJxt-ituTx9YA5ZMPVL1CqOoqnEHRPowBAVI','2026-07-04 09:44:05',0,'2026-06-27 09:44:05','2026-06-27 09:44:05'),('6f84423d-bd3f-4ddf-a3a3-e5982506a8e9','00bf8263-aa6e-4c8c-bed4-a1be16642cda','eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMDBiZjgyNjMtYWE2ZS00YzhjLWJlZDQtYTFiZTE2NjQyY2RhIiwiZW1haWwiOiJmcmllcmVuQGV4YW1wbGUuY29tIiwiZXhwIjoxNzgzMDkwMzUyLCJpYXQiOjE3ODI0ODU1NTIsImp0aSI6IjAwYmY4MjYzLWFhNmUtNGM4Yy1iZWQ0LWExYmUxNjY0MmNkYS0yMDI2MDYyNjIxNTIzMiJ9.jvy95xW8u9pO7dyFyaB8lkdmxNP1J2Qq92TLI-KKGb4','2026-07-03 14:52:32',1,'2026-06-26 14:52:32','2026-06-26 14:54:28'),('70414490-f4d8-46a6-9b6c-8d3def52e269','00bf8263-aa6e-4c8c-bed4-a1be16642cda','eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMDBiZjgyNjMtYWE2ZS00YzhjLWJlZDQtYTFiZTE2NjQyY2RhIiwiZW1haWwiOiJmcmllcmVuQGV4YW1wbGUuY29tIiwicm9sZSI6ImFkbWluIiwiZXhwIjoxNzgzMTg1Nzg5LCJpYXQiOjE3ODI1ODA5ODksImp0aSI6IjAwYmY4MjYzLWFhNmUtNGM4Yy1iZWQ0LWExYmUxNjY0MmNkYS0yMDI2MDYyODAwMjMwOSJ9.wNY1Wq7vYD1ZHJyNNbM1mrI3HwEB5OApcwpgyV9x7Lw','2026-07-04 17:23:09',0,'2026-06-27 17:23:09','2026-06-27 17:23:09'),('7b55bbe7-8101-4fd9-ba09-f7e1ee4459fa','00bf8263-aa6e-4c8c-bed4-a1be16642cda','eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMDBiZjgyNjMtYWE2ZS00YzhjLWJlZDQtYTFiZTE2NjQyY2RhIiwiZW1haWwiOiJmcmllcmVuQGV4YW1wbGUuY29tIiwicm9sZSI6IiIsImV4cCI6MTc4MzE1MTUyMSwiaWF0IjoxNzgyNTQ2NzIxLCJqdGkiOiIwMGJmODI2My1hYTZlLTRjOGMtYmVkNC1hMWJlMTY2NDJjZGEtMjAyNjA2MjcxNDUyMDEifQ.ian0EN7YyLKPSUmkunxluB_FCMZ5fiIV7GBUtuT_-xU','2026-07-04 07:52:01',0,'2026-06-27 07:52:01','2026-06-27 07:52:01'),('8ad45fdf-3a65-4cc7-b5f6-3e459f5134b7','00bf8263-aa6e-4c8c-bed4-a1be16642cda','eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMDBiZjgyNjMtYWE2ZS00YzhjLWJlZDQtYTFiZTE2NjQyY2RhIiwiZW1haWwiOiJmcmllcmVuQGV4YW1wbGUuY29tIiwicm9sZSI6ImFkbWluIiwiZXhwIjoxNzgzMTU3MzUzLCJpYXQiOjE3ODI1NTI1NTMsImp0aSI6IjAwYmY4MjYzLWFhNmUtNGM4Yy1iZWQ0LWExYmUxNjY0MmNkYS0yMDI2MDYyNzE2MjkxMyJ9.9xPO0ykjSOsn3KypMIVavEp7Jz7hcK_k0mw-pBJU75Q','2026-07-04 09:29:13',0,'2026-06-27 09:29:13','2026-06-27 09:29:13'),('921e36c3-58e7-4dbf-95cd-17b9db4d8518','00bf8263-aa6e-4c8c-bed4-a1be16642cda','eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMDBiZjgyNjMtYWE2ZS00YzhjLWJlZDQtYTFiZTE2NjQyY2RhIiwiZW1haWwiOiJmcmllcmVuQGV4YW1wbGUuY29tIiwiZXhwIjoxNzgzMDkwMTc5LCJpYXQiOjE3ODI0ODUzNzksImp0aSI6IjAwYmY4MjYzLWFhNmUtNGM4Yy1iZWQ0LWExYmUxNjY0MmNkYS0yMDI2MDYyNjIxNDkzOSJ9.vDi2OKEKqt_F0NZPtVvoSuI-V5jYW3H7Ja5QZA2yO08','2026-07-03 14:49:40',1,'2026-06-26 14:49:39','2026-06-26 14:52:32'),('a45e6209-d16c-484e-be6d-6147136471c9','f287cc66-24b9-4f17-b29f-2da20e98d0db','eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiZjI4N2NjNjYtMjRiOS00ZjE3LWIyOWYtMmRhMjBlOThkMGRiIiwiZW1haWwiOiJGZXJuQGV4YW1wbGUuY29tIiwicm9sZSI6InVzZXIiLCJleHAiOjE3ODMxNTY3ODIsImlhdCI6MTc4MjU1MTk4MiwianRpIjoiZjI4N2NjNjYtMjRiOS00ZjE3LWIyOWYtMmRhMjBlOThkMGRiLTIwMjYwNjI3MTYxOTQyIn0.GiuF1ERD8mdWUVX-rgufn9QWpIV8NRa1J7Xq2EnW5fE','2026-07-04 09:19:42',0,'2026-06-27 09:19:42','2026-06-27 09:19:42'),('d1318d87-ea3f-4493-b911-a42aab143c00','1bc9f191-5f2f-4df0-ac8e-3e6be8ce3533','eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMWJjOWYxOTEtNWYyZi00ZGYwLWFjOGUtM2U2YmU4Y2UzNTMzIiwiZW1haWwiOiJyaW11cnVAZXhhbXBsZS5jb20iLCJyb2xlIjoidXNlciIsImV4cCI6MTc4MzE1ODgxMSwiaWF0IjoxNzgyNTU0MDExLCJqdGkiOiIxYmM5ZjE5MS01ZjJmLTRkZjAtYWM4ZS0zZTZiZThjZTM1MzMtMjAyNjA2MjcxNjUzMzEifQ.G6Z8OFbl954kEwVsJeqOHI1L5DCnS_SV90kk0KUBLzk','2026-07-04 09:53:32',0,'2026-06-27 09:53:31','2026-06-27 09:53:31'),('eab96fe7-212b-4181-8794-29163d87da9a','46487f0c-d2a7-4482-82fb-8dfdf405e109','eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiNDY0ODdmMGMtZDJhNy00NDgyLTgyZmItOGRmZGY0MDVlMTA5IiwiZW1haWwiOiJoZWl0ZXIyQGV4YW1wbGUuY29tIiwicm9sZSI6InVzZXIiLCJleHAiOjE3ODMxNTgyODYsImlhdCI6MTc4MjU1MzQ4NiwianRpIjoiNDY0ODdmMGMtZDJhNy00NDgyLTgyZmItOGRmZGY0MDVlMTA5LTIwMjYwNjI3MTY0NDQ2In0.lAAu-EosEdVkhfI3fB4PiDvqrYlPa9DtfEUhTKBwA68','2026-07-04 09:44:47',0,'2026-06-27 09:44:46','2026-06-27 09:44:46');
/*!40000 ALTER TABLE `refresh_tokens` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `schema_migrations`
--

DROP TABLE IF EXISTS `schema_migrations`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `schema_migrations` (
  `version` bigint NOT NULL,
  `dirty` tinyint(1) NOT NULL,
  PRIMARY KEY (`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `schema_migrations`
--

LOCK TABLES `schema_migrations` WRITE;
/*!40000 ALTER TABLE `schema_migrations` DISABLE KEYS */;
INSERT INTO `schema_migrations` VALUES (7,0);
/*!40000 ALTER TABLE `schema_migrations` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `transactions`
--

DROP TABLE IF EXISTS `transactions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `transactions` (
  `id` varchar(36) NOT NULL,
  `sender_wallet_id` varchar(36) DEFAULT NULL,
  `receiver_wallet_id` varchar(36) NOT NULL,
  `amount` decimal(15,2) NOT NULL,
  `description` text,
  `idempotency_key` varchar(100) NOT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'success',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idempotency_key` (`idempotency_key`),
  KEY `sender_wallet_id` (`sender_wallet_id`),
  KEY `receiver_wallet_id` (`receiver_wallet_id`),
  CONSTRAINT `transactions_ibfk_1` FOREIGN KEY (`sender_wallet_id`) REFERENCES `wallets` (`id`),
  CONSTRAINT `transactions_ibfk_2` FOREIGN KEY (`receiver_wallet_id`) REFERENCES `wallets` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `transactions`
--

LOCK TABLES `transactions` WRITE;
/*!40000 ALTER TABLE `transactions` DISABLE KEYS */;
INSERT INTO `transactions` VALUES ('05be2233-d7cc-4204-97c1-ca73de08ccc3',NULL,'31e84205-93ab-43a7-aa5d-79d3adb8b8a2',100000.00,'Top Up','unique-uuid-key-abc','success','2026-06-27 17:24:56'),('199f145c-f227-4772-8a87-8cc09de4a9bf',NULL,'7e99ad82-bbf0-4e5e-9576-d0317877ba91',100.00,'Top Up','test-topup-001','success','2026-06-27 09:54:23'),('20d2daff-a13b-42c3-8548-d575aac28c13','31e84205-93ab-43a7-aa5d-79d3adb8b8a2','45065b58-3f04-4631-b430-d5f35c2c771f',20000.00,'Dinner split','unique-uuid-key-123','success','2026-06-27 17:25:48'),('5c43a1d7-7bfa-4127-94f3-5e483b33ef3d','4f3b0cac-985d-4563-9b55-2a4c58830621','1f97b30b-23a6-4dd8-9cca-06c1d7d75006',10.00,'Test transfer back','test-transfer-003','success','2026-06-27 09:47:43'),('dd7bc5e8-7e68-4366-9601-3c9bd4b5d728','7e99ad82-bbf0-4e5e-9576-d0317877ba91','117e1866-999d-49db-ac96-b903502dcc1d',10.00,'Test transfer','test-transfer-004','success','2026-06-27 09:54:46'),('de13235b-5a96-4fef-b046-66e046551236','31e84205-93ab-43a7-aa5d-79d3adb8b8a2','45065b58-3f04-4631-b430-d5f35c2c771f',50000.00,'Test transfer','test-transfer-001','success','2026-06-20 08:53:06'),('f613f498-295c-4d18-8b7a-674e59951157','45065b58-3f04-4631-b430-d5f35c2c771f','31e84205-93ab-43a7-aa5d-79d3adb8b8a2',50000.00,'Test transfer back','test-transfer-002','success','2026-06-27 09:20:24');
/*!40000 ALTER TABLE `transactions` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `users`
--

DROP TABLE IF EXISTS `users`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `users` (
  `id` varchar(36) NOT NULL,
  `full_name` varchar(100) NOT NULL,
  `email` varchar(150) NOT NULL,
  `role` varchar(20) NOT NULL DEFAULT 'user',
  `oauth_provider` varchar(50) DEFAULT NULL,
  `oauth_id` varchar(255) DEFAULT NULL,
  `password_hash` varchar(255) DEFAULT NULL,
  `avatar_url` varchar(255) DEFAULT NULL,
  `is_verified` tinyint(1) NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `email` (`email`),
  UNIQUE KEY `unique_oauth` (`oauth_provider`,`oauth_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `users`
--

LOCK TABLES `users` WRITE;
/*!40000 ALTER TABLE `users` DISABLE KEYS */;
INSERT INTO `users` VALUES ('00bf8263-aa6e-4c8c-bed4-a1be16642cda','Frieren','frieren@example.com','admin',NULL,NULL,'$2a$10$ijsuqLqrKTF7a2fz1Pqu0eWTJeicV1AjhRNZMK/K6YFMeh3gOCqJS','/uploads/00bf8263-aa6e-4c8c-bed4-a1be16642cda.jpeg',0,'2026-06-18 11:35:31','2026-06-27 07:51:34',NULL),('110b310f-f0b3-437f-abf5-74661f6b84dd','veldora','veldora@example.com','user',NULL,NULL,'$2a$10$6LAJJjknmTA/dLa71W8XL.taI3Jt.qYDFp69BSXBOFG3yffXgp1GO',NULL,0,'2026-06-27 09:53:25','2026-06-27 09:53:25',NULL),('1bc9f191-5f2f-4df0-ac8e-3e6be8ce3533','rimuru','rimuru@example.com','user',NULL,NULL,'$2a$10$zy.4fzpr.RjhjUtjI1.Q0.QJADJwyMrgRz0w1002v60X4M.oWKB76',NULL,0,'2026-06-27 09:53:16','2026-06-27 09:53:16',NULL),('3a70c1e5-175f-4835-b760-e2d7b9d694f5','Stark','stark@example.com','user',NULL,NULL,'$2a$10$Iil526Z2sM4Q6Qmw8ZlpZ.0fWiTSaCZ3KOMCmtZiVWK5MO4nJgyHO',NULL,0,'2026-06-23 14:04:00','2026-06-23 14:04:00',NULL),('46487f0c-d2a7-4482-82fb-8dfdf405e109','Heiter3','heiter2@example.com','user',NULL,NULL,'$2a$10$1Nih3aUj6r12EwY6dXdKrug8BVjh0ugKsD6VEUeJG3Ez2WB7BWQB.',NULL,1,'2026-06-23 14:10:45','2026-06-23 14:18:08',NULL),('6a799adb-9dba-42bd-8b3c-a796a02f7517','Heiter','heiter@example.com','user',NULL,NULL,'$2a$10$ITWjTDnT8jNbgMkW2aaJZOSYF5EGU8huEapzJrA7mn6BhycFpez52',NULL,0,'2026-06-23 14:07:35','2026-06-25 14:31:27',NULL),('726f4351-6599-4614-a24f-1faa3761958f','himmel','himmel@example.com','user',NULL,NULL,'$2a$10$K5WBGWtNyfSX6eZ8DCnOze3vYGgD2IK9.zlDUf1mmfRpEwZXQhyhW',NULL,0,'2026-06-20 14:34:21','2026-06-20 14:35:25','2026-06-20 14:35:25'),('aa47ca85-214a-45eb-a3e0-26c12d6e2173','Nida Afha','nidaafha0@gmail.com','user','google','106532491699376254225','',NULL,1,'2026-06-25 15:01:41','2026-06-25 15:01:41',NULL),('ac5517b6-91a5-461f-886a-2e8101c3b298','Eisen','eisen@example.com','user',NULL,NULL,'$2a$10$Xivm4lOZVvEy9IFEu9X7COZ/MQexEIlSh0L2dpW/XLdO.Cv8AKsUq',NULL,0,'2026-06-21 13:19:48','2026-06-21 13:19:48',NULL),('bb2fd545-14fa-492c-b099-116ff6142468','Heiter2','heiter3@example.com','user',NULL,NULL,'$2a$10$QOdrS5/1w9ETg3X7vccAi.5UJRSelE4GhQYeFc0oHuOA.c.7CDtQ2',NULL,0,'2026-06-23 14:10:26','2026-06-23 14:10:26',NULL),('f287cc66-24b9-4f17-b29f-2da20e98d0db','Fern','Fern@example.com','user',NULL,NULL,'$2a$10$wgEBjDB6wLYyTelMLEoEFelAC6qEo4TcFktgrn2o1W7ieAev/UKca',NULL,0,'2026-06-18 11:36:34','2026-06-18 11:36:34',NULL),('f6370b7f-156a-4d70-b051-b5201c9bf0c0','Heiter1','heiter1@example.com','user',NULL,NULL,'$2a$10$wI0/nsjAhM/cmamjTe88KOvTJNF5G5ANtqcjs9WzjC423HOdmdB5S',NULL,0,'2026-06-23 14:08:55','2026-06-23 14:08:55',NULL);
/*!40000 ALTER TABLE `users` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `wallets`
--

DROP TABLE IF EXISTS `wallets`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `wallets` (
  `id` varchar(36) NOT NULL,
  `user_id` varchar(36) NOT NULL,
  `balance` decimal(15,2) NOT NULL DEFAULT '0.00',
  `currency` varchar(3) NOT NULL DEFAULT 'IDR',
  `status` varchar(20) NOT NULL DEFAULT 'active',
  `version` int NOT NULL DEFAULT '1',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `user_id` (`user_id`),
  CONSTRAINT `wallets_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `wallets`
--

LOCK TABLES `wallets` WRITE;
/*!40000 ALTER TABLE `wallets` DISABLE KEYS */;
INSERT INTO `wallets` VALUES ('117e1866-999d-49db-ac96-b903502dcc1d','110b310f-f0b3-437f-abf5-74661f6b84dd',10.00,'IDR','active',2,'2026-06-27 09:53:25','2026-06-27 09:54:46',NULL),('11897732-f721-48a9-ad7a-1a7e67add437','f6370b7f-156a-4d70-b051-b5201c9bf0c0',0.00,'IDR','active',1,'2026-06-23 14:08:55','2026-06-23 14:08:55',NULL),('1f97b30b-23a6-4dd8-9cca-06c1d7d75006','bb2fd545-14fa-492c-b099-116ff6142468',10.00,'IDR','active',2,'2026-06-23 14:10:26','2026-06-27 09:47:43',NULL),('204c4d5e-7933-463c-a5b2-2d1f9858c5fb','6a799adb-9dba-42bd-8b3c-a796a02f7517',0.00,'IDR','active',1,'2026-06-23 14:07:35','2026-06-23 14:07:35',NULL),('31e84205-93ab-43a7-aa5d-79d3adb8b8a2','00bf8263-aa6e-4c8c-bed4-a1be16642cda',80000.00,'IDR','active',5,'2026-06-18 11:35:31','2026-06-27 17:25:48',NULL),('45065b58-3f04-4631-b430-d5f35c2c771f','f287cc66-24b9-4f17-b29f-2da20e98d0db',20100.00,'IDR','active',4,'2026-06-18 11:36:34','2026-06-27 17:25:48',NULL),('4f3b0cac-985d-4563-9b55-2a4c58830621','46487f0c-d2a7-4482-82fb-8dfdf405e109',90.00,'IDR','active',2,'2026-06-23 14:10:45','2026-06-27 09:47:43',NULL),('63ffe1e9-cdae-47c8-9cf1-d606b6abe5d5','ac5517b6-91a5-461f-886a-2e8101c3b298',0.00,'IDR','active',1,'2026-06-21 13:19:48','2026-06-21 13:19:48',NULL),('7e99ad82-bbf0-4e5e-9576-d0317877ba91','1bc9f191-5f2f-4df0-ac8e-3e6be8ce3533',90.00,'IDR','active',3,'2026-06-27 09:53:16','2026-06-27 09:54:46',NULL),('ae3770f9-163d-47c5-a5e2-b110b38d39e4','726f4351-6599-4614-a24f-1faa3761958f',0.00,'IDR','active',1,'2026-06-20 14:34:21','2026-06-20 14:34:21',NULL),('d29e222e-505a-4120-90a9-1fb26d166e3a','aa47ca85-214a-45eb-a3e0-26c12d6e2173',0.00,'IDR','active',1,'2026-06-25 15:01:41','2026-06-25 15:01:41',NULL),('da4f3b78-8495-49df-986b-6f84335582a7','3a70c1e5-175f-4835-b760-e2d7b9d694f5',0.00,'IDR','active',1,'2026-06-23 14:04:00','2026-06-23 14:04:00',NULL);
/*!40000 ALTER TABLE `wallets` ENABLE KEYS */;
UNLOCK TABLES;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2026-06-30 15:29:05
