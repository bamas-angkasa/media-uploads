# MediaShare — Backend

REST API server for the MediaShare platform. Built with **Go 1.22**, **Gin**, **PostgreSQL 16**, **Redis 7**, and **AWS S3**.

---

## Table of Contents

- [Tech Stack](#tech-stack)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Environment Variables](#environment-variables)
  - [Run with Docker](#run-with-docker)
  - [Run Locally](#run-locally)
- [Database Migrations](#database-migrations)
- [API Reference](#api-reference)
  - [Auth](#auth)
  - [Upload](#upload)
  - [Media (User)](#media-user)
  - [Public](#public)
  - [Admin](#admin)
- [Architecture](#architecture)
  - [Upload Flow](#upload-flow)
  - [Auth & JWT Flow](#auth--jwt-flow)
  - [Media Processing](#media-processing)
  - [View Count Buffering](#view-count-buffering)
- [Module Overview](#module-overview)

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.22 |
| HTTP Framework | Gin v1.10 |
| Database | PostgreSQL 16 via `pgx/v5` (pgxpool) |
| Cache / Queue | Redis 7 via `go-redis/v9` |
| Object Storage | AWS S3 via `aws-sdk-go-v2` |
| Auth | JWT (HS256) via `golang-jwt/jwt/v5` |
| Migrations | Goose v3 (embedded SQL) |
| Image Processing | `disintegration/imaging` (libvips-style) |
| Video Processing | FFmpeg (exec) + ffprobe |
| Short Codes | `crypto/rand` (URL-safe, 8 chars) |
| Config | Viper (env vars) |
| Logging | Uber Zap |

---

## Project Structure

```
backend/
├── cmd/
│   ├── api/
│   │   └── main.go          # Entry point — wires all modules, starts HTTP server
│   └── migrate/
│       └── main.go          # Standalone goose CLI for manual migration runs
├── config/
│   └── config.go            # Viper-based env config struct
├── internal/
│   ├── auth/
│   │   ├── service.go       # Register, login, logout, refresh, JWT issuance
│   │   ├── handler.go       # HTTP handlers for /api/auth/*
│   │   └── middleware.go    # JWTMiddleware, RequireRole, GetUserID helpers
│   ├── upload/
│   │   ├── service.go       # Presigned URL generation, confirm, quota check
│   │   └── handler.go       # HTTP handlers + SSE progress stream
│   ├── media/
│   │   ├── service.go       # List, get, update, delete (user-scoped)
│   │   └── handler.go       # HTTP handlers for /api/media/*
│   ├── public/
│   │   ├── service.go       # Explore feed, search, short-code lookup, view/download counts
│   │   └── handler.go       # HTTP handlers for /api/public/*
│   ├── admin/
│   │   ├── service.go       # Admin CRUD, stats, report management
│   │   └── handler.go       # HTTP handlers for /api/admin/*
│   ├── processor/
│   │   └── processor.go     # Goroutine worker pool, image resize, video thumbnail
│   ├── storage/
│   │   └── s3.go            # S3 client wrapper (presign put/get, CDN URL, delete)
│   ├── shortcode/
│   │   └── shortcode.go     # Cryptographically random URL-safe code generator
│   └── database/
│       └── db.go            # pgxpool factory, Redis client factory
└── migrations/
    ├── embed.go             # go:embed *.sql → var FS embed.FS
    ├── 001_create_users.sql
    ├── 002_create_media.sql
    ├── 003_create_reports.sql
    └── 004_create_refresh_tokens.sql
```

---

## Getting Started

### Prerequisites

- Go 1.22+
- PostgreSQL 16
- Redis 7
- FFmpeg & ffprobe (for video processing)
- AWS account with an S3 bucket (or use LocalStack for local dev)

### Environment Variables

Copy the example file and fill in your values:

```bash
cp ../.env.example .env
```

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP server port |
| `APP_ENV` | `development` | `development` or `production` |
| `POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_USER` | `mediashare` | PostgreSQL user |
| `POSTGRES_PASSWORD` | — | PostgreSQL password |
| `POSTGRES_DB` | `mediashare` | Database name |
| `POSTGRES_SSL_MODE` | `disable` | SSL mode (`disable`, `require`, `verify-full`) |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` | — | Redis password (leave empty if none) |
| `REDIS_DB` | `0` | Redis DB index |
| `AWS_ACCESS_KEY_ID` | — | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | — | AWS secret key |
| `AWS_REGION` | `ap-southeast-1` | S3 bucket region |
| `S3_BUCKET_NAME` | — | S3 bucket name |
| `CDN_BASE_URL` | — | CloudFront base URL (e.g. `https://cdn.example.com`) |
| `JWT_SECRET` | — | Secret for signing JWT tokens (min 32 chars) |
| `JWT_ACCESS_TTL` | `15m` | Access token lifetime |
| `JWT_REFRESH_TTL` | `168h` | Refresh token lifetime (7 days) |
| `WATERMARK_ENABLED` | `false` | Enable watermark on processed images |
| `MAX_IMAGE_SIZE_BYTES` | `10485760` | Max image upload size (10 MB) |
| `MAX_VIDEO_SIZE_BYTES` | `524288000` | Max video upload size (500 MB) |
| `STORAGE_QUOTA_BYTES` | `10737418240` | Default storage quota per user (10 GB) |
| `WORKER_CONCURRENCY` | `3` | Max concurrent media processing goroutines |

### Run with Docker

```bash
# From the repo root
cp .env.example .env
# Edit .env with your credentials

docker compose up --build
```

The API will be available at `http://localhost:8080`.

### Run Locally

```bash
# 1. Start dependencies
docker compose up postgres redis -d

# 2. Install Go dependencies
cd backend
go mod tidy

# 3. Run (migrations run automatically on startup)
go run ./cmd/api
```

> **Note:** Make sure `ffmpeg` and `ffprobe` are installed and on your `$PATH`.
> On macOS: `brew install ffmpeg` | On Ubuntu: `apt install ffmpeg`

---

## Database Migrations

Migrations use [Goose v3](https://github.com/pressly/goose) and are embedded directly into the binary via `go:embed`, so no external SQL files are required at runtime.

### Auto-run on startup

Migrations run automatically when the API server starts via `goose.Up`. The server will exit if any migration fails.

### Manual migration CLI

A standalone migrate tool is available at `cmd/migrate`:

```bash
# Check current migration status
go run ./cmd/migrate status

# Apply all pending migrations
go run ./cmd/migrate up

# Apply one migration
go run ./cmd/migrate up-by-one

# Roll back the latest migration
go run ./cmd/migrate down

# Create a new migration file
go run ./cmd/migrate create add_likes_table sql
```

### Migration files

| File | Description |
|---|---|
| `001_create_users.sql` | Users table with role, plan, storage quota fields |
| `002_create_media.sql` | Media + media_files tables, GIN indexes for full-text search and tags |
| `003_create_reports.sql` | Reports table |
| `004_create_refresh_tokens.sql` | Refresh token table (hashed, revocable) |

### Writing a new migration

```bash
go run ./cmd/migrate create your_migration_name sql
```

This creates `migrations/YYYYMMDDHHMMSS_your_migration_name.sql`. Edit the file:

```sql
-- +goose Up
ALTER TABLE media ADD COLUMN is_featured BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE media DROP COLUMN is_featured;
```

---

## API Reference

All endpoints are prefixed with `/api`. JSON request/response bodies unless noted.

### Auth

#### `POST /api/auth/register`

Create a new user account.

**Request:**
```json
{
  "email": "user@example.com",
  "username": "johndoe",
  "password": "password123"
}
```

**Response `201`:**
```json
{
  "access_token": "<jwt>",
  "expires_in": 900,
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "username": "johndoe",
    "role": "user",
    "plan": "free",
    "storage_quota": 10737418240,
    "storage_used": 0,
    "created_at": "2026-03-14T10:00:00Z"
  }
}
```

Sets `refresh_token` cookie (`HttpOnly; Secure; SameSite=Strict`).

---

#### `POST /api/auth/login`

**Request:**
```json
{ "email": "user@example.com", "password": "password123" }
```

**Response `200`:** Same shape as register. Sets `refresh_token` cookie.

---

#### `POST /api/auth/refresh`

Silently refreshes the access token using the `refresh_token` cookie.
Implements token rotation — old refresh token is revoked and a new one is issued.

**Response `200`:**
```json
{ "access_token": "<new-jwt>", "expires_in": 900 }
```

---

#### `POST /api/auth/logout` 🔒

Revokes the current refresh token and clears the cookie.

**Response `200`:** `{ "message": "logged out" }`

---

#### `GET /api/auth/me` 🔒

Returns the current user's profile.

**Response `200`:** `{ "user": { ... } }`

---

### Upload

#### `POST /api/upload/sign` 🔒

Step 1 of the upload flow. Returns a presigned S3 PUT URL — the client uploads directly to S3 without proxying through the backend.

**Request:**
```json
{
  "filename": "photo.jpg",
  "content_type": "image/jpeg",
  "size_bytes": 2048000
}
```

**Allowed content types:** `image/jpeg`, `image/png`, `image/webp`, `image/gif`, `video/mp4`, `video/webm`, `video/quicktime`

**Response `200`:**
```json
{
  "upload_url": "https://bucket.s3.amazonaws.com/uploads/...?X-Amz-Signature=...",
  "media_id": "uuid",
  "short_code": "aBc12345",
  "expires_at": 1741950000
}
```

**Errors:**
- `400` — unsupported file type or file too large
- `403` — storage quota exceeded

---

#### `POST /api/upload/confirm` 🔒

Step 3 of the upload flow. Call this after the S3 PUT succeeds. Triggers background processing.

**Request:**
```json
{ "media_id": "uuid" }
```

**Response `200`:** `{ "message": "processing started", "media_id": "uuid" }`

---

#### `GET /api/upload/progress/:id` 🔒

Server-Sent Events stream for real-time processing progress. Connect after `/confirm`.

**Response:** `text/event-stream`

```
data: {"media_id":"uuid","status":"processing","progress":40}

data: {"media_id":"uuid","status":"processing","progress":80}

data: {"media_id":"uuid","status":"ready","progress":100}
```

Close the connection when `status` is `"ready"` or `"failed"`.

---

### Media (User)

All endpoints below require authentication and are scoped to the logged-in user's own files.

#### `GET /api/media` 🔒

List the current user's uploaded files.

**Query params:** `page` (default `1`), `page_size` (default `20`, max `50`)

**Response `200`:**
```json
{
  "data": [ { ...media }, ... ],
  "page": 1,
  "page_size": 20
}
```

---

#### `GET /api/media/:id` 🔒

Get a single media item owned by the current user.

---

#### `PATCH /api/media/:id` 🔒

Update title, description, or tags.

**Request:**
```json
{
  "title": "My new title",
  "description": "Updated description",
  "tags": ["nature", "travel"]
}
```

All fields are optional. Omitted fields are not changed.

---

#### `DELETE /api/media/:id` 🔒

Permanently deletes the media item and all its S3 variants. Updates the user's `storage_used`.

---

### Public

No authentication required (except `/report`).

#### `GET /api/public/explore`

Paginated feed of all public `ready` media, newest first. Uses keyset (cursor) pagination for performance.

**Query params:**
- `cursor` — opaque pagination cursor from previous response
- `page_size` — default `20`, max `50`
- `type` — filter by `image`, `video`, or `gif`

**Response `200`:**
```json
{
  "data": [ { ...media }, ... ],
  "next_cursor": "2026-03-14T10:00:00Z_uuid"
}
```

`next_cursor` is `null` when there are no more results.

---

#### `GET /api/public/search?q=`

Full-text search using PostgreSQL `to_tsvector` on `title` and `description`.

**Query params:** `q` (required), `page`, `page_size`

**Response `200`:**
```json
{
  "data": [ { ...media }, ... ],
  "total": 42,
  "page": 1,
  "page_size": 20
}
```

---

#### `GET /api/public/:short_code`

Fetch a media item by its 8-character short code. Used by the share page.

---

#### `POST /api/public/:short_code/view`

Increments view count. Buffered in Redis and flushed to PostgreSQL every 60 seconds.

---

#### `POST /api/public/:short_code/download`

Increments download count (written directly to PostgreSQL). Returns a time-limited presigned S3 GET URL.

**Response `200`:** `{ "download_url": "https://..." }`

---

#### `POST /api/public/:short_code/report` 🔒

Submit a content report.

**Request:**
```json
{ "reason": "Inappropriate content" }
```

---

### Admin

All endpoints require authentication **and** `role = admin`.

#### `GET /api/admin/media`

List all media across all users.

**Query params:** `page`, `page_size`, `status`, `type`, `q` (search), `user_id`, `sort_by` (`created_at` or `view_count`), `order` (`asc`/`desc`)

---

#### `DELETE /api/admin/media/:id`

Permanently delete any media item (S3 + database).

---

#### `GET /api/admin/users`

List all users with storage usage.

**Query params:** `page`, `page_size`, `q` (search username/email)

---

#### `PATCH /api/admin/users/:id`

Update a user's status, role, or storage quota.

**Request (all fields optional):**
```json
{
  "is_active": false,
  "role": "admin",
  "storage_quota": 21474836480
}
```

---

#### `GET /api/admin/reports`

List content reports.

**Query params:** `page`, `page_size`, `status` (`pending`, `resolved`, `dismissed`)

---

#### `PATCH /api/admin/reports/:id`

Resolve or dismiss a report.

**Request:**
```json
{ "action": "resolve" }
```

`action` must be `"resolve"` or `"dismiss"`.

---

#### `GET /api/admin/stats`

Platform-wide statistics.

**Response `200`:**
```json
{
  "data": {
    "total_media": 1523,
    "total_users": 248,
    "total_storage_gb": 18.42,
    "media_last_24h": 37,
    "users_last_7d": 14,
    "pending_reports": 3
  }
}
```

---

## Architecture

### Upload Flow

The backend **never proxies file data**. This keeps the API server lightweight and stateless.

```
Client                  API Server              S3
  │                         │                   │
  ├─ POST /upload/sign ────►│                   │
  │                         ├─ quota check       │
  │                         ├─ generate short_code
  │                         ├─ INSERT media (status=processing)
  │                         ├─ store intent in Redis (20min TTL)
  │◄─ { upload_url, ... } ──┤                   │
  │                         │                   │
  ├─ PUT file ─────────────────────────────────►│
  │                         │                   │
  ├─ POST /upload/confirm ─►│                   │
  │                         ├─ validate Redis intent
  │                         ├─ HeadObject (verify S3)
  │                         ├─ enqueue processor job
  │◄─ { media_id } ─────────┤                   │
  │                         │                   │
  ├─ GET /upload/progress/:id (SSE) ───────────►│
  │◄═ data: {progress: 40} ═╪═══════════════════│
  │◄═ data: {status: ready} ╪═══════════════════│
```

### Auth & JWT Flow

Two-token strategy:

| Token | Location | TTL | Storage |
|---|---|---|---|
| Access token | `Authorization: Bearer` header | 15 min | In-memory (JS) |
| Refresh token | `HttpOnly` cookie | 7 days | Hashed in PostgreSQL |

- Access token is **stateless** — verified by signature only.
- Refresh token is **stateful** — every refresh issues a new token and revokes the old one (rotation).
- On 401, the frontend auto-retries using the refresh token silently.
- Logout immediately revokes the refresh token in the database.

### Media Processing

Processing runs in a **goroutine worker pool** (configurable via `WORKER_CONCURRENCY`):

**Images:**
1. Download from S3 to temp file
2. Detect MIME type from magic bytes (`h2non/filetype`) — never trust client claim
3. Decode with `disintegration/imaging`
4. Generate 300px thumbnail → upload to S3 as `_thumb.jpg`
5. Generate 1080px variant if original is wider → upload as `_1080p.jpg`
6. Insert `media_files` rows, update `media.status = 'ready'`

**Videos:**
1. Download from S3 to temp file
2. Run `ffprobe` to extract duration, width, height
3. Extract poster frame with `ffmpeg -vframes 1`
4. Upload poster to S3 as `_poster.jpg`
5. Update `media.status = 'ready'`

Progress is written to Redis (`process:{mediaID}`) and polled by the SSE endpoint.

### View Count Buffering

Direct `UPDATE` on every page view causes write contention on popular items. Instead:

1. **On view:** `INCR media:views:{id}` in Redis (atomic, ~1μs)
2. **Every 60s:** Background goroutine reads all `media:views:*` keys, issues batch `UPDATE` statements to PostgreSQL, deletes Redis keys

This trades strong consistency for ~60-second eventual consistency on view counts — acceptable for analytics.

---

## Module Overview

| Package | Responsibility |
|---|---|
| `config` | Load all config from environment variables via Viper |
| `internal/database` | pgxpool factory (25 max connections), Redis client factory |
| `internal/auth` | JWT issuance, bcrypt hashing, refresh token rotation |
| `internal/upload` | Presigned URL generation, S3 upload confirmation, SSE |
| `internal/media` | User-scoped CRUD on media items |
| `internal/public` | Explore feed (keyset pagination), full-text search, share page |
| `internal/admin` | Admin panel operations, platform statistics |
| `internal/processor` | Worker pool for image/video post-processing |
| `internal/storage` | Thin S3 client wrapper: presign, get, put, delete, CDN URL |
| `internal/shortcode` | 8-character cryptographically random URL-safe code generator |
| `migrations` | Embedded SQL migration files (Goose) |
