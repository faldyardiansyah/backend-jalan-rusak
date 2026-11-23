-- ROADIS: Migrasi Skema Tabel Riwayat Status Laporan (Tahap 2B Final)
-- Target Database: db_jalan_rusak
-- Deskripsi: Menyimpan catatan audit transisi status operasional laporan secara atomik.
-- Kebijakan FK:
--   - fk_riwayat_status_laporan_laporan: ON DELETE CASCADE (laporan spam/dihapus menghapus riwayat terkait)
--   - fk_riwayat_status_laporan_user: ON DELETE SET NULL (akun user/petugas dihapus tidak merusak jejak audit riwayat)

CREATE TABLE IF NOT EXISTS `riwayat_status_laporan` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `laporan_id` bigint(20) unsigned NOT NULL,
  `status_lama` varchar(50) NOT NULL,
  `status_baru` varchar(50) NOT NULL,
  `user_id` bigint(20) unsigned DEFAULT NULL,
  `role` varchar(50) NOT NULL,
  `catatan` text DEFAULT NULL,
  `created_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_riwayat_status_laporan_laporan_id` (`laporan_id`),
  KEY `idx_riwayat_status_laporan_user_id` (`user_id`),
  KEY `idx_riwayat_status_laporan_created_at` (`created_at`),
  CONSTRAINT `fk_riwayat_status_laporan_laporan` FOREIGN KEY (`laporan_id`) REFERENCES `laporan_kerusakan` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_riwayat_status_laporan_user` FOREIGN KEY (`user_id`) REFERENCES `user` (`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
