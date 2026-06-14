# PushKit Backend

The PushKit backend is a Go API server that brokers file uploads and downloads through pre-signed S3 URLs. The S3 bucket stays private; clients (the CLI and the Android app) authenticate to this server with an API key and receive short-lived presigned URLs to transfer bytes directly to and from S3.

Stack: Go (chi router), SQLite for metadata, and any S3-compatible object store (AWS S3, MinIO, Cloudflare R2) via `aws-sdk-go-v2`.

## Run locally with Docker

The fastest path is `docker compose`, which brings up the server alongside a local MinIO:

```bash
cd backend
docker compose up --build
```

This starts three services:

| Service | Port | Purpose |
|---------|------|---------|
| `api` | `8000` | The PushKit server |
| `minio` | `9000` | S3-compatible object store |
| `minio` console | `9001` | MinIO web UI (`minioadmin` / `minioadmin`) |
| `minio-init` | — | One-shot job that creates the `pushkit-dev` bucket |

Dev defaults baked into `docker-compose.yml`: MinIO credentials `minioadmin` / `minioadmin`, and API keys `dev-key-1:user1,dev-key-2:user2`. Point the CLI at it with `--api-url=http://localhost:8000 --api-key=dev-key-1`.

## Build and run directly

```bash
cd backend
CGO_ENABLED=0 go build -o pushkit-server ./cmd/server

# Provide configuration via the environment, then run:
S3_BUCKET=my-bucket \
AWS_ACCESS_KEY_ID=... \
AWS_SECRET_ACCESS_KEY=... \
API_KEYS=key1:alice,key2:bob \
./pushkit-server
```

Check the version (this is baked in at build time with `-ldflags "-X main.Version=<v>"`; a plain `go build` reports `dev`):

```bash
./pushkit-server --version    # → pushkit-server <version>
```

## Configuration

All configuration is read from environment variables at startup. The server fails fast if a required variable is missing.

| Env Var | Required | Default | Description |
|---------|----------|---------|-------------|
| `S3_BUCKET` | Yes | — | Bucket name |
| `AWS_ACCESS_KEY_ID` | Yes | — | S3 access key |
| `AWS_SECRET_ACCESS_KEY` | Yes | — | S3 secret key |
| `API_KEYS` | Yes | — | Comma-separated `key:user` pairs, e.g. `k1:alice,k2:bob` |
| `AWS_REGION` | No | `us-east-1` | S3 region |
| `S3_ENDPOINT_URL` | No | — | S3-compatible endpoint. Set for MinIO/R2; leave unset for AWS S3 |
| `DATABASE_URL` | No | `pushkit.db` | SQLite file path |
| `LISTEN_ADDR` | No | `:8000` | `host:port` to listen on |
| `TLS_CERT_FILE` | No | — | TLS certificate path |
| `TLS_KEY_FILE` | No | — | TLS private-key path |

### S3 vs. MinIO / R2

Set `S3_ENDPOINT_URL` to your S3-compatible endpoint (e.g. `http://minio:9000` for the bundled MinIO, or your Cloudflare R2 endpoint). Leave it unset to talk to real AWS S3 using `AWS_REGION`.

### TLS

When **both** `TLS_CERT_FILE` and `TLS_KEY_FILE` are set, the server starts in HTTPS mode; otherwise it serves plain HTTP. TLS is off by default.

## API

All `/v1` routes require an `X-API-Key` header whose value matches one of the keys in `API_KEYS`. The health check is unauthenticated.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/uploads/init` | Reserve a file ID and get a presigned PUT URL |
| `POST` | `/v1/uploads/complete` | Finalize an upload and record metadata |
| `GET` | `/v1/files` | List files (`cursor`, `limit`, `q`, `sort`, `order` query params) |
| `GET` | `/v1/files/{fileId}/download` | Get a presigned GET URL |
| `DELETE` | `/v1/files/{fileId}` | Delete a file (DB record + best-effort S3 object) |
| `GET` | `/health` | Health check, returns `{"status":"ok"}` (no auth) |

See the [project README](../README.md#api-endpoints) for an end-to-end curl example.

## Test

```bash
cd backend
go vet ./...
go test ./...
```

## Windows installer (end users)

Each release publishes a Windows installer, `pushkit-server-setup.exe`, that installs the server binary and optionally registers it as a Windows service. (Maintainers building the installer: see [installer/README.md](installer/README.md).)

### Download

- Latest stable: <https://github.com/jayteealao/PushKit/releases/latest>
- Including prereleases: <https://github.com/jayteealao/PushKit/releases>

### Install

Interactive (UAC prompt):

```powershell
.\pushkit-server-setup.exe
```

Default components install `pushkit-server.exe` to `%ProgramFiles%\PushKit\`, add a Start Menu shortcut, and register an Apps & Features entry. The Windows service component (default-checked) registers `PushKitServer` as a manual-start service via the bundled NSIS SimpleSC plugin; manage it afterward with `sc.exe` or Services.msc.

Silent (applies all default-checked components, including the service):

```powershell
.\pushkit-server-setup.exe /S
```

The installer requires elevation (`RequestExecutionLevel admin`); UAC prompts if the shell is not already elevated.

### Uninstall

Settings → Apps & Features → **PushKit Server** → Uninstall, or silently:

```powershell
& "$env:ProgramFiles\PushKit\uninstall.exe" /S
```

### SmartScreen warning

The installer is not code-signed (out of scope for the v0.x line). On first launch Windows SmartScreen shows "Unknown publisher". Click **More info** → **Run anyway** to proceed. Code-signing is on the roadmap.
