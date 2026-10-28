# Ballo Todo

Aplikasi web todo single-user menggunakan [Ballo](https://github.com/lukman-ss/ballo) framework dan [Ballo Starter](https://github.com/lukman-ss/ballo-starter) sebagai fondasi.

## Versi

| Komponen | Versi |
|---|---|
| Go | 1.23.0 |
| Ballo | `v0.0.0-20261027061017-e84a7679c8a4` |
| modernc.org/sqlite | lihat `go.mod` (pure Go, tanpa CGO) |

## Fitur

- Buat, edit, hapus todo dengan konfirmasi
- Tandai selesai / kembalikan ke aktif
- Prioritas: Rendah · Sedang · Tinggi
- Tanggal tenggat (opsional), indikator terlambat
- Filter: Semua · Aktif · Selesai
- Pencarian judul dan catatan
- Urutkan: Terbaru · Tenggat · Prioritas
- Ringkasan aktif, selesai, terlambat
- Pagination 20 item per halaman
- Antarmuka berbahasa Indonesia, responsif (mobile & desktop)
- Penyimpanan persisten via SQLite
- Perlindungan CSRF pada semua form

## Mulai Cepat

```bash
# Clone dan jalankan
git clone git@github.com:lukman-ss/ballo-todo.git
cd ballo-todo
go run ./cmd/server
```

Server berjalan di `http://localhost:8080` secara default.

Cek health:
```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

## Instalasi

Prasyarat: Go 1.23+. Tidak ada CGO, tidak perlu gcc atau SQLite library sistem.

```bash
go mod download
make run
```

## Konfigurasi

Semua konfigurasi via environment variable. Salin `.env.example` ke `.env` dan sesuaikan:

```bash
cp .env.example .env
```

| Variabel | Default | Keterangan |
|---|---|---|
| `APP_ENV` | `development` | `production` aktifkan default lebih ketat |
| `HTTP_ADDR` | `:8080` | Alamat listen server |
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | |
| `HTTP_READ_TIMEOUT` | `10s` | |
| `HTTP_WRITE_TIMEOUT` | `10s` | |
| `HTTP_IDLE_TIMEOUT` | `120s` | |
| `SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown |
| `MAX_BODY_BYTES` | `1048576` | Maks ukuran request body (1 MB) |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `DB_PATH` | `./data/ballo-todo.db` | Lokasi file SQLite |
| `TZ` | `Asia/Jakarta` | Timezone IANA untuk kalkulasi terlambat |
| `TEMPLATE_DIR` | `internal/web/templates` | Direktori template HTML |

## Persistence Database

Data disimpan di file SQLite. Default lokasi: `./data/ballo-todo.db`.

File-file SQLite:
- `ballo-todo.db` — database utama
- `ballo-todo.db-wal` — Write-Ahead Log (WAL mode)
- `ballo-todo.db-shm` — shared memory file

File-file ini **tidak masuk Git** (tercantum di `.gitignore`).

Migrasi schema dijalankan otomatis saat startup. Aman dijalankan ulang (idempotent). Migrasi gagal akan menghentikan startup dengan pesan error yang jelas.

## Build & Test

```bash
make check       # fmt + vet + test + race
make build       # binary → bin/server
make test        # unit tests saja
make race        # dengan race detector
make vet         # go vet
make fmt         # cek format dengan gofmt
make clean       # hapus bin/ dan artefak
```

Jalankan langsung:
```bash
go vet ./...
go test ./...
go test -race ./...
go build ./...
```

## Backup & Restore SQLite

**Jangan salin file `.db` saat aplikasi berjalan** — WAL mode memerlukan ketiga file (`*.db`, `*.db-wal`, `*.db-shm`) dalam kondisi konsisten.

Cara backup yang benar menggunakan SQLite online backup:

```bash
# Backup (aman saat aplikasi berjalan)
sqlite3 ./data/ballo-todo.db ".backup '/backup/ballo-todo-$(date +%Y%m%d).db'"

# Atau gunakan rsync setelah checkpoint
sqlite3 ./data/ballo-todo.db "PRAGMA wal_checkpoint(TRUNCATE);"
cp ./data/ballo-todo.db /backup/ballo-todo-$(date +%Y%m%d).db

# Restore (pastikan aplikasi mati dulu)
cp /backup/ballo-todo-20261027.db ./data/ballo-todo.db
rm -f ./data/ballo-todo.db-wal ./data/ballo-todo.db-shm
```

## Docker

Build dan jalankan dengan volume persisten:

```bash
# Build image
docker build -t ballo-todo .

# Jalankan dengan volume untuk data SQLite
docker run -p 8080:8080 \
  -v ballo-todo-data:/app/data \
  ballo-todo

# Dengan environment kustom
docker run -p 8080:8080 \
  -v ballo-todo-data:/app/data \
  -e TZ=Asia/Jakarta \
  -e LOG_LEVEL=info \
  ballo-todo
```

**Penting:** Selalu mount volume di `/app/data`. Jangan menyimpan database di filesystem container yang sementara.

Backup dari container:
```bash
docker run --rm \
  -v ballo-todo-data:/app/data \
  alpine sh -c "sqlite3 /app/data/ballo-todo.db \".backup '/app/data/backup.db'\""
```

## Endpoint HTTP

### Health probes

| Method | Path | Keterangan |
|---|---|---|
| `GET` | `/healthz` | `200 {"status":"pass"}` |
| `GET` | `/readyz` | `200` atau `503` |

### Antarmuka Web (Browser)

| Method | Path | Keterangan |
|---|---|---|
| `GET` | `/` | Daftar todo (filter, search, sort, pagination) |
| `GET` | `/todos/new` | Form tambah todo |
| `POST` | `/todos` | Buat todo (quick-add atau full form) |
| `GET` | `/todos/:id/edit` | Form edit todo |
| `POST` | `/todos/:id/edit` | Simpan perubahan todo |
| `POST` | `/todos/:id/toggle` | Toggle status selesai |
| `POST` | `/todos/:id/delete` | Hapus todo |

### REST API (dari starter)

| Method | Path | Keterangan |
|---|---|---|
| `POST` | `/api/v1/tasks` | Buat task (JSON) |
| `GET` | `/api/v1/tasks` | List tasks (JSON) |
| `GET` | `/api/v1/tasks/:id` | Get task by ID |
| `PATCH` | `/api/v1/tasks/:id` | Update partial task |
| `DELETE` | `/api/v1/tasks/:id` | Hapus task |

## Struktur Proyek

```
cmd/server/main.go              # Entrypoint, signal handling, graceful shutdown
internal/app/app.go             # Wiring: engine, middleware, routes, DB, store
internal/config/config.go       # Konfigurasi dari env vars
internal/todo/                  # Domain todo: store SQLite, validasi, migrasi
  todo.go
  migrate.go
  todo_test.go
internal/task/                  # Domain task (dari starter): in-memory store
  task.go
  task_test.go
internal/http/handler/
  health.go                     # Health endpoints
  task.go                       # REST API task handler
  web.go                        # Web UI handler (todo)
  web_test.go                   # Integration tests web handler
internal/web/
  renderer.go                   # HTML template renderer
  csrf.go                       # CSRF double-submit cookie
  templates/                    # HTML templates
    base.html
    index.html
    new.html
    edit.html
  static/                       # Asset statis
    style.css
    app.js
```

## Keamanan & Batasan

- **Single-user tanpa autentikasi.** Server mendengarkan localhost secara default (`:8080`).
- Deployment publik **harus** menggunakan lapisan autentikasi (reverse proxy dengan basic auth, VPN, dll) atau pembatasan akses jaringan.
- CSRF dilindungi dengan double-submit cookie pattern (bukan pengganti autentikasi).
- Body request dibatasi oleh `MAX_BODY_BYTES`.
- Log mencatat method, path, status, duration, remote_addr — tidak mencatat isi catatan, cookie, atau body.
- Tidak ada credential dalam konfigurasi, kode, atau test.
- Escaping HTML menggunakan `html/template` bawaan Go.

## Keterbatasan

- Data todo in-memory hilang saat restart — **sudah diatasi dengan SQLite**.
- Data task (`/api/v1/tasks`) masih in-memory (dari starter), hilang saat restart.
- Tidak ada autentikasi, autorisasi, atau rate limiting.
- Tidak ada pagination untuk API REST `/api/v1/tasks`.
- Tidak ada ekspor/impor data.
- Tidak ada recurring task, attachment, notifikasi, atau kolaborasi.
- Single instance: belum mendukung multi-replica horizontal (SQLite single-writer).
