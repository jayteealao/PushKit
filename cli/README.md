# PushKit CLI

`pushkit` is a command-line client for uploading, listing, and downloading files via an S3-backed API. Files move directly to and from S3 through short-lived presigned URLs — the CLI never holds AWS credentials.

## Install

```bash
pip install --pre pushkit
```

PushKit is currently prerelease, so `--pre` is required. The package ships as a self-contained binary wheel; no Go toolchain is needed to run it.

To build from source instead:

```bash
cd cli
go build -o pushkit .
```

## Configure

The CLI needs an API base URL and an API key. Set them once:

```bash
pushkit config set --api-url=https://your-server.example.com --api-key=sk-...
```

Check the current configuration (the key is masked):

```bash
pushkit config show
```

### Where credentials come from

Values are resolved with this precedence — **CLI flags > environment variables > config file**:

| Source | API URL | API key |
|--------|---------|---------|
| Flags | `--api-url` | `--api-key` |
| Environment | `PUSHKIT_API_URL` | `PUSHKIT_API_KEY` |
| Config file | `api_url` | `api_key` |

> Passing `--api-key` on the command line exposes the key in the OS process list, so the CLI prints a warning when you do. Prefer `PUSHKIT_API_KEY` or the config file.

### Config file location

A JSON file (`{"api_url": "...", "api_key": "..."}`) written with `0600` permissions:

| OS | Path |
|----|------|
| macOS | `~/Library/Application Support/pushkit/config.json` |
| Linux | `$XDG_CONFIG_HOME/pushkit/config.json` (or `~/.config/pushkit/config.json`) |
| Windows | `%APPDATA%\pushkit\config.json` |

## Commands

### `upload <file>`

Uploads a file in three automatic steps (init → presigned S3 PUT → complete).

```bash
pushkit upload ./report.pdf
pushkit upload ./data.csv --name quarterly.csv --tag team=analytics --tag quarter=Q1
pushkit upload ./backup.tar.gz --sha256
```

| Flag | Description |
|------|-------------|
| `--name` | Filename to store in the API (default: the file's basename) |
| `--tag` | `key=value` metadata tag; repeatable |
| `--sha256` | Compute and send a SHA-256 hash for server-side integrity verification |

### `download <fileId>`

Requests a presigned GET URL and streams the file to disk.

```bash
pushkit download abc123
pushkit download abc123 --out ./local-copy.pdf --force
```

| Flag | Description |
|------|-------------|
| `--out`, `-o` | Output path (default: the original filename in the current directory) |
| `--force`, `-f` | Overwrite an existing file (otherwise the download is refused) |

The default output name comes from the server's `Content-Disposition`, sanitized to strip any directory components.

### `ls` (alias: `list`)

Lists files. Results are paginated — the 20 most recent by default.

```bash
pushkit ls
pushkit ls -q .pdf --sort size_bytes --order desc
pushkit ls --all
```

| Flag | Description |
|------|-------------|
| `-q` | Filter by filename substring |
| `--sort` | `created_at` (default), `original_filename`, or `size_bytes` |
| `--order` | `desc` (default) or `asc` |
| `--limit` | Results per page (default: 20) |
| `--all` | Fetch every page |

### `config set` / `config show`

Manage stored credentials (see [Configure](#configure)).

## Global flags

These work on every command:

| Flag | Description |
|------|-------------|
| `--api-url` | Override the configured API base URL |
| `--api-key` | Override the configured API key (visible in the process list — prefer the env var) |
| `--json` | Emit machine-readable JSON on stdout; suppress progress bars and status messages |

## JSON output (for scripts and agents)

Pass `--json` to any command for structured output. On success the result goes to stdout; on failure `{"error":"message"}` goes to stderr with a non-zero exit code.

```jsonc
// upload
{"id":"...","originalFilename":"...","contentType":"...","sizeBytes":1234,"status":"uploaded"}

// download
{"fileId":"...","filename":"...","path":"/abs/path","sizeBytes":1234}

// ls (each item: id, originalFilename, contentType, sizeBytes, createdAt, status)
{"items":[...],"nextCursor":"..."}

// config set
{"saved":true,"configPath":"..."}

// config show (the full key is never included)
{"apiUrl":"...","apiKeySet":true,"configPath":"..."}
```

## Exit codes

Commands exit `0` on success and non-zero on failure. In `--json` mode, errors are emitted as `{"error":"..."}` on stderr; otherwise as `Error: <message>`.
