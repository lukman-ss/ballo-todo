# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.0] - 2026-11-04

### Changed

- `List` dan `scanTodoFromRows`: error iterasi database (misalnya jaringan atau kegagalan query) kini dipropagasikan ke pemanggil alih-alih dianggap akhir normal atau menghasilkan dataset parsial.
- `TestList_queryError` diperbaiki agar driver simulasi benar-benar memicu error saat iterasi baris, sehingga tes ini secara akurat memverifikasi propagasi error.
- `scanTodoFromRows`: penutup `rows.Close()` ditambahkan sebelum mengembalikan error scan (misalnya ketidakcocokan tipe kolom), menghindari kebocoran resource pada kegagalan pemindaian.
- Tes baru `TestList_scanError` mensimulasikan error scan melalui driver kustom dan memverifikasi propagasi error.

## [0.3.0] - 2026-11-04

### Changed

- `List`: rows.Next() yang berhenti karena error database dikembalikan ke pemanggil, bukan diabaikan.
- `scanTodoFromRows`: rows.Err() diperiksa dan dikembalikan jika terjadi error saat iterasi.
- Tes `TestList_queryError`: mensimulasikan error saat rows.Next() dan memastikan error dikembalikan.

## [0.2.0] - 2026-11-04

### Added

- Migrasi otomatis mengaktifkan WAL mode pada SQLite untuk keamanan backup dan konkurensi.
- Tes regresi: validasi mode WAL, cookie Secure/non-Secure, dan penghindaran CSRF pada rute `/api/`.

### Changed

- Dokumentasi README selaras dengan endpoint, fitur, dependency, dan batasan aktual.
- Penjelasan backup/restore memperjelas bahwa WAL diaktifkan oleh migrasi.

## [0.1.0] - 2026-10-27

### Added

- Web UI todo dengan create, read, update, delete, toggle selesai, konfirmasi hapus.
- Filter status (Semua · Aktif · Selesai), pencarian judul & catatan, urutan (Terbaru · Tenggat · Prioritas).
- Pagination 20 item per halaman.
- Prioritas: Rendah · Sedang · Tinggi, dengan tenggat opsional dan indikator terlambat.
- Ringkasan aktif, selesai, terlambat di header.
- Penyimpanan persisten via SQLite (pure Go, tanpa CGO) dengan migrasi schema idempotent.
- Perlindungan CSRF double-submit cookie pada semua form (kecuali `/api/`).
- REST API `/api/v1/tasks` (dari starter): create, list, get, update, delete.
- Health probes `/healthz` dan `/readyz`.
- Antarmuka berbahasa Indonesia, responsif (mobile & desktop).
- Konfigurasi via environment variable (lihat `.env.example`).
- Dockerfile dengan volume persisten untuk data SQLite.
- Backup & restore SQLite melalui `sqlite3 .backup` atau checkpoint WAL.
- Makefile: `run`, `build`, `test`, `race`, `vet`, `fmt`, `check`, `clean`.

### Changed

- Dockerfile menggunakan image builder `golang:1.26-alpine` sesuai versi Go di `go.mod`.

### Removed

- Tidak ada perubahan yang menghapus fitur.