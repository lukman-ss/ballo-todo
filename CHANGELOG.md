# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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