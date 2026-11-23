-- MariaDB dump 10.19  Distrib 10.4.32-MariaDB, for Win64 (AMD64)
--
-- Host: localhost    Database: db_jalan_rusak
-- ------------------------------------------------------
-- Server version	10.4.32-MariaDB

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `laporan_kerusakan`
--

DROP TABLE IF EXISTS `laporan_kerusakan`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `laporan_kerusakan` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `user_id` bigint(20) unsigned NOT NULL,
  `wilayah_id` bigint(20) unsigned NOT NULL,
  `jenis_jalan` varchar(50) NOT NULL,
  `judul` varchar(150) NOT NULL,
  `deskripsi` text NOT NULL,
  `latitude` double NOT NULL,
  `longitude` double NOT NULL,
  `image_url` text NOT NULL,
  `tipe_kerusakan` varchar(150) NOT NULL,
  `status` varchar(50) NOT NULL DEFAULT 'menunggu',
  `severity` varchar(50) DEFAULT 'belum_dinilai',
  `ai_model` varchar(100) DEFAULT NULL,
  `ai_confidence` decimal(5,4) DEFAULT NULL,
  `ditugaskan_ke` varchar(150) DEFAULT NULL,
  `foto_bukti` text DEFAULT NULL,
  `catatan_admin` text DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_laporan_kerusakan_deleted_at` (`deleted_at`),
  KEY `fk_laporan_kerusakan_wilayah` (`wilayah_id`),
  KEY `fk_user_reports` (`user_id`),
  CONSTRAINT `fk_laporan_kerusakan_wilayah` FOREIGN KEY (`wilayah_id`) REFERENCES `wilayah` (`id`),
  CONSTRAINT `fk_user_reports` FOREIGN KEY (`user_id`) REFERENCES `user` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=11 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `laporan_kerusakan`
--

LOCK TABLES `laporan_kerusakan` WRITE;
/*!40000 ALTER TABLE `laporan_kerusakan` DISABLE KEYS */;
INSERT INTO `laporan_kerusakan` VALUES (1,'2026-11-23 00:29:02.081','2026-11-23 00:29:02.081',NULL,4,1,'desa','[DEV] Jalan Desa Sukaurip - Lubang Parah','Terdapat beberapa lubang berdiameter 40-60 cm dengan kedalaman sekitar 10 cm yang membahayakan pengendara motor di Desa Sukaurip.',-6.34,108.33,'https://images.unsplash.com/photo-1515162816999-a0c47dc192f7?w=800','Lubang','menunggu','belum_dinilai',NULL,NULL,'','',''),(2,'2026-11-23 00:29:02.095','2026-11-23 00:29:02.095',NULL,4,1,'desa','[DEV] Jalan Desa Margadana - Retak Buaya','Struktur aspal retak buaya cukup luas di sepanjang 50 meter jalan penghubung antar dusun Margadana.',-6.355,108.315,'https://images.unsplash.com/photo-1515162816999-a0c47dc192f7?w=800','Retak Buaya','proses','belum_dinilai',NULL,NULL,'Tim Pemeliharaan Desa Margadana','','Material pengurukan dan perataan telah tiba di lokasi desa.'),(3,'2026-11-23 00:29:02.105','2026-11-23 00:29:02.105',NULL,4,2,'desa','[DEV] Jalan Desa Lobener Lor - Lubang Jalan','Lubang jalan di dekat balai desa Lobener Lor yang sebelumnya tergenang saat hujan.',-6.415,108.283,'https://images.unsplash.com/photo-1515162816999-a0c47dc192f7?w=800','Lubang','selesai','belum_dinilai',NULL,NULL,'Kaur Pembangunan Desa Lobener Lor','https://images.unsplash.com/photo-1544620347-c4fd4a3d5957?w=800','Penambalan aspal cold-mix dan pemadatan telah selesai 100%.'),(4,'2026-11-23 00:29:02.109','2026-11-23 00:29:02.109',NULL,5,2,'desa','[DEV] Jalan Desa Lobener Lor - Bahu Jalan Amblas','Bahu jalan desa di dekat saluran irigasi Lobener Lor amblas sedalam 25 cm, memerlukan pengurukan batu dan pemadatan ulang.',-6.418,108.285,'https://images.unsplash.com/photo-1515162816999-a0c47dc192f7?w=800','Jalan Amblas','proses','belum_dinilai',NULL,NULL,'Tim Pemeliharaan Desa Lobener Lor','','Material pengurukan dan alat pemadat telah dijadwalkan ke lokasi dusun timur.'),(5,'2026-11-23 00:29:02.114','2026-11-23 00:29:02.114',NULL,6,2,'desa','[DEV] Jalan Desa Lobener Lor - Genangan dan Lubang','Permukaan jalan desa berlubang dengan diameter 40 cm dan kerap tergenang air saat hujan, membahayakan pengguna sepeda motor.',-6.412,108.281,'https://images.unsplash.com/photo-1515162816999-a0c47dc192f7?w=800','Lubang','menunggu','belum_dinilai',NULL,NULL,'','',''),(6,'2026-11-23 00:29:02.117','2026-11-23 00:29:02.117',NULL,4,1,'kabupaten','[DEV] Jalan Kabupaten Terisi-Cikedung - Amblas','Bahu jalan kabupaten amblas sedalam 30 cm akibat gerusan air irigasi, separuh jalan dipasangi rambu darurat.',-6.37,108.26,'https://images.unsplash.com/photo-1515162816999-a0c47dc192f7?w=800','Jalan Amblas','menunggu','belum_dinilai',NULL,NULL,'','',''),(7,'2026-11-23 00:29:02.120','2026-11-23 00:29:02.120',NULL,4,2,'kabupaten','[DEV] Jalan Kabupaten Jatibarang-Sleman - Retak Melintang','Retak melintang jalan kabupaten yang menyebabkan guncangan keras bagi kendaraan angkutan barang.',-6.47,108.305,'https://images.unsplash.com/photo-1515162816999-a0c47dc192f7?w=800','Retak Melintang','proses','belum_dinilai',NULL,NULL,'UPTD Pengawasan Jalan Dinas PU Indramayu','','Alat berat perata aspal dan tim pelaksana sudah mulai pengerjaan perbaikan.'),(8,'2026-11-23 00:29:02.123','2026-11-23 00:29:02.123',NULL,4,2,'kabupaten','[DEV] Jalan Kabupaten Lobener-Jatibarang - Bergelombang','Permukaan aspal jalan kabupaten bergelombang parah akibat muatan tonase berlebih.',-6.435,108.295,'https://images.unsplash.com/photo-1515162816999-a0c47dc192f7?w=800','Permukaan Bergelombang','selesai','belum_dinilai',NULL,NULL,'Dinas PUPR Kabupaten Indramayu','https://images.unsplash.com/photo-1544620347-c4fd4a3d5957?w=800','Overlay aspal hotmix sepanjang 200 meter telah tuntas dikerjakan.'),(9,'2026-11-23 00:29:02.126','2026-11-23 00:29:02.126',NULL,4,1,'provinsi','[DEV] Jalan Provinsi Indramayu-Karangampel - Lubang Besar','Ruas jalan provinsi mengalami kerusakan lubang berurutan dengan diameter lebih dari 50 cm.',-6.36,108.38,'https://images.unsplash.com/photo-1515162816999-a0c47dc192f7?w=800','Lubang','menunggu','belum_dinilai',NULL,NULL,'','',''),(10,'2026-11-23 00:29:02.130','2026-11-23 00:29:02.130',NULL,4,2,'nasional','[DEV] Jalan Nasional Pantura Lobener - Retak Memanjang','Jalur utama Pantura segmen Lobener mengalami retak memanjang pada lajur lambat.',-6.42,108.28,'https://images.unsplash.com/photo-1515162816999-a0c47dc192f7?w=800','Retak Memanjang','proses','belum_dinilai',NULL,NULL,'Balai Besar Pelaksanaan Jalan Nasional (BBPJN)','','Koordinasi penanganan dan penjadwalan patching lajur pantura bersama tim BBPJN.');
/*!40000 ALTER TABLE `laporan_kerusakan` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `notifikasi`
--

DROP TABLE IF EXISTS `notifikasi`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `notifikasi` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `user_id` bigint(20) unsigned NOT NULL,
  `laporan_id` bigint(20) unsigned DEFAULT NULL,
  `judul` varchar(200) NOT NULL,
  `pesan` text NOT NULL,
  `is_read` tinyint(1) DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_notifikasi_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `notifikasi`
--

LOCK TABLES `notifikasi` WRITE;
/*!40000 ALTER TABLE `notifikasi` DISABLE KEYS */;
/*!40000 ALTER TABLE `notifikasi` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `riwayat_chat`
--

DROP TABLE IF EXISTS `riwayat_chat`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `riwayat_chat` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `laporan_kerusakan_id` bigint(20) unsigned NOT NULL,
  `user_id` bigint(20) unsigned NOT NULL,
  `pesan` text NOT NULL,
  `admin_id` bigint(20) unsigned DEFAULT NULL,
  `balasan` text DEFAULT NULL,
  `dibalas_at` datetime(3) DEFAULT NULL,
  `lampiran_balasan_url` varchar(255) DEFAULT NULL,
  `lampiran_balasan_nama` varchar(255) DEFAULT NULL,
  `lampiran_balasan_mime_type` varchar(100) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_riwayat_chat_deleted_at` (`deleted_at`),
  KEY `fk_riwayat_chat_laporan_kerusakan` (`laporan_kerusakan_id`),
  KEY `fk_riwayat_chat_admin` (`admin_id`),
  KEY `fk_user_chat_histories` (`user_id`),
  CONSTRAINT `fk_riwayat_chat_admin` FOREIGN KEY (`admin_id`) REFERENCES `user` (`id`),
  CONSTRAINT `fk_riwayat_chat_laporan_kerusakan` FOREIGN KEY (`laporan_kerusakan_id`) REFERENCES `laporan_kerusakan` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_user_chat_histories` FOREIGN KEY (`user_id`) REFERENCES `user` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=7 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `riwayat_chat`
--

LOCK TABLES `riwayat_chat` WRITE;
/*!40000 ALTER TABLE `riwayat_chat` DISABLE KEYS */;
INSERT INTO `riwayat_chat` VALUES (1,'2026-09-25 09:15:00.000','2026-09-25 09:15:00.000',NULL,3,4,'Selamat pagi Bapak/Ibu Admin Desa, izin menanyakan perihal laporan lubang jalan di dekat balai desa Lobener Lor apakah sudah masuk jadwal penanganan?',3,'Selamat pagi Pak Faldy. Laporan sudah kami terima dan ditugaskan ke Kaur Pembangunan Desa Lobener Lor. Penambalan aspal cold-mix dan pemadatan telah dijadwalkan.','2026-09-25 09:30:00.000',NULL,NULL,NULL),(2,'2026-09-25 10:05:00.000','2026-09-25 10:05:00.000',NULL,3,4,'Baik Pak Admin, terima kasih banyak atas responnya. Apakah saat pengerjaan nanti ada pengalihan arus sementara di sekitar balai desa?',3,'Pengalihan arus kendaraan roda empat dikoordinasikan dengan Linmas desa setempat selama proses pengerjaan penambalan berlangsung.','2026-09-25 10:20:00.000',NULL,NULL,NULL),(3,'2026-09-26 10:15:00.000','2026-09-26 10:15:00.000',NULL,4,5,'Pak Admin, apakah laporan kerusakan bahu jalan amblas di dekat irigasi desa Lobener Lor sudah mulai diperbaiki?',3,'Selamat siang Ibu Siti. Tim pemeliharaan desa sudah mulai melakukan pengurukan batu di lokasi amblas tersebut. Terima kasih atas laporannya.','2026-09-26 10:45:00.000',NULL,NULL,NULL),(4,'2026-09-26 14:00:00.000','2026-09-26 14:00:00.000',NULL,5,6,'Pak Admin, kondisi jalan berlubang dan tergenang air di dusun barat semakin parah saat hujan. Apakah sudah ada jadwal pengecekan ke lokasi?',NULL,NULL,NULL,NULL,NULL,NULL),(5,'2026-09-25 11:20:00.000','2026-09-25 11:20:00.000',NULL,1,4,'Selamat siang Admin, mohon info apakah lubang parah di Desa Sukaurip sudah dipasangi rambu peringatan sebelum pengerjaan?',NULL,NULL,NULL,NULL,NULL,NULL),(6,'2026-09-25 13:00:00.000','2026-09-25 13:00:00.000',NULL,2,4,'Pak Admin Pemdes, material pengurukan sudah tampak tiba di Margadana. Apakah pengerjaan dimulai besok?',7,'Betul Pak, tim pemeliharaan desa dijadwalkan mulai pengerjaan perataan jalan besok pagi.','2026-09-25 13:45:00.000',NULL,NULL,NULL);
/*!40000 ALTER TABLE `riwayat_chat` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `user`
--

DROP TABLE IF EXISTS `user`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `user` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `name` varchar(150) NOT NULL,
  `email` varchar(150) NOT NULL,
  `password` varchar(150) NOT NULL,
  `role` varchar(50) NOT NULL DEFAULT 'warga',
  `wilayah_id` bigint(20) unsigned DEFAULT NULL,
  `profile_photo` text DEFAULT NULL,
  `phone` varchar(30) DEFAULT NULL,
  `avatar_url` text DEFAULT NULL,
  `last_login_at` datetime(3) DEFAULT NULL,
  `password_changed_at` datetime(3) DEFAULT NULL,
  `token_version` bigint(20) unsigned NOT NULL DEFAULT 1,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_user_email` (`email`),
  KEY `idx_user_deleted_at` (`deleted_at`),
  KEY `fk_user_wilayah` (`wilayah_id`),
  CONSTRAINT `fk_user_wilayah` FOREIGN KEY (`wilayah_id`) REFERENCES `wilayah` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=8 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `user`
--

LOCK TABLES `user` WRITE;
/*!40000 ALTER TABLE `user` DISABLE KEYS */;
INSERT INTO `user` VALUES (1,'2026-11-23 00:29:01.989','2026-11-23 00:29:01.989',NULL,'Super Admin','superadmin@gmail.com','$2a$10$Idh.tWTcajN1931fOiCA2.kBMXvgkPYpVwwJOk9qp50kShv1.06jy','super_admin',NULL,'',NULL,NULL,NULL,NULL,1),(2,'2026-11-23 00:29:02.007','2026-11-23 00:30:43.146',NULL,'Admin Dinas PU','adminpu@gmail.com','$2a$10$Idh.tWTcajN1931fOiCA2.kBMXvgkPYpVwwJOk9qp50kShv1.06jy','admin_pu',NULL,'',NULL,NULL,'2026-11-23 00:30:43.145',NULL,1),(3,'2026-11-23 00:29:02.014','2026-11-23 00:30:13.191',NULL,'Admin Dinas Pemdes','adminpemdes@gmail.com','$2a$10$Idh.tWTcajN1931fOiCA2.kBMXvgkPYpVwwJOk9qp50kShv1.06jy','admin_pemdes',2,'',NULL,NULL,'2026-11-23 00:30:13.186',NULL,1),(4,'2026-11-23 00:29:02.017','2026-11-23 00:29:02.017',NULL,'Faldy Ardiansyah','faldy@gmail.com','$2a$10$Idh.tWTcajN1931fOiCA2.kBMXvgkPYpVwwJOk9qp50kShv1.06jy','warga',2,'',NULL,NULL,NULL,NULL,1),(5,'2026-11-23 00:29:02.020','2026-11-23 00:29:02.020',NULL,'Siti Aminah','siti.aminah@roadis.local','$2a$10$Idh.tWTcajN1931fOiCA2.kBMXvgkPYpVwwJOk9qp50kShv1.06jy','warga',2,'',NULL,NULL,NULL,NULL,1),(6,'2026-11-23 00:29:02.022','2026-11-23 00:29:02.022',NULL,'Bambang Prasetyo','bambang.prasetyo@roadis.local','$2a$10$Idh.tWTcajN1931fOiCA2.kBMXvgkPYpVwwJOk9qp50kShv1.06jy','warga',2,'',NULL,NULL,NULL,NULL,1),(7,'2026-11-23 00:29:02.075','2026-11-23 00:29:02.075',NULL,'Admin Dinas Pemdes Indramayu','adminpemdes_indramayu@gmail.com','$2a$10$SZDGj3fXBhlqPJSigieOoersCgcq1yjjQI4ny0g8EHDd9j8/6wqfm','admin_pemdes',1,'',NULL,NULL,NULL,NULL,1);
/*!40000 ALTER TABLE `user` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `user_preference`
--

DROP TABLE IF EXISTS `user_preference`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `user_preference` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `user_id` bigint(20) unsigned NOT NULL,
  `notification_sound_enabled` tinyint(1) DEFAULT 1,
  `notification_report_enabled` tinyint(1) DEFAULT 1,
  `notification_status_enabled` tinyint(1) DEFAULT 1,
  `notification_chat_enabled` tinyint(1) DEFAULT 1,
  `theme` varchar(20) NOT NULL DEFAULT 'light',
  `display_density` varchar(20) NOT NULL DEFAULT 'comfortable',
  `map_default_view` varchar(20) NOT NULL DEFAULT 'standard',
  `map_show_labels` tinyint(1) DEFAULT 1,
  `report_display_preference` varchar(20) NOT NULL DEFAULT 'comfortable',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_user_preference_user_id` (`user_id`),
  KEY `idx_user_preference_deleted_at` (`deleted_at`),
  CONSTRAINT `fk_user_preference` FOREIGN KEY (`user_id`) REFERENCES `user` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `user_preference`
--

LOCK TABLES `user_preference` WRITE;
/*!40000 ALTER TABLE `user_preference` DISABLE KEYS */;
INSERT INTO `user_preference` VALUES (1,'2026-11-23 00:30:13.362','2026-11-23 00:30:13.362',NULL,3,1,1,1,1,'light','comfortable','standard',1,'comfortable'),(2,'2026-11-23 00:38:17.759','2026-11-23 00:38:17.759',NULL,2,1,1,1,1,'light','comfortable','standard',1,'comfortable');
/*!40000 ALTER TABLE `user_preference` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `wilayah`
--

DROP TABLE IF EXISTS `wilayah`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `wilayah` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `nama` varchar(250) NOT NULL,
  `tipe` varchar(50) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_wilayah_deleted_at` (`deleted_at`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `wilayah`
--

LOCK TABLES `wilayah` WRITE;
/*!40000 ALTER TABLE `wilayah` DISABLE KEYS */;
INSERT INTO `wilayah` VALUES (1,'2026-11-23 00:29:01.911','2026-11-23 00:29:01.911',NULL,'Indramayu','desa'),(2,'2026-11-23 00:29:01.931','2026-11-23 00:29:01.931',NULL,'Lobener Lor','desa');
/*!40000 ALTER TABLE `wilayah` ENABLE KEYS */;
UNLOCK TABLES;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2026-11-23 16:10:55
