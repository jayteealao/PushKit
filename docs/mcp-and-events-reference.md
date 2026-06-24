# PushKit MCP & events reference

Reference for the `pushkit mcp` server (the Claude Code integration) and the backend
`GET /v1/events` stream that drives its auto-refresh. For setup instructions, see
[Connect Claude Code to PushKit](connect-claude-code.md).

## MCP server

| Property | Value |
|----------|-------|
| Command | `pushkit mcp` (hidden subcommand of the `pushkit` CLI) |
| Transport | MCP over stdio |
| Configuration | API URL + key resolved as flags > environment (`PUSHKIT_API_URL`, `PUSHKIT_API_KEY`) > config file |
| Startup | Always starts, even with a missing/invalid key or unreachable backend; tool calls then return an auth/connection error |
| Output | Tool results return file paths and compact metadata, never inline file bytes, to stay under the MCP output cap |
| Capabilities | Four tools + read-only file resources (`resources/list_changed` supported) |

### Tool: `pushkit_push`

Upload one or more local files to PushKit.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `paths` | string[] | yes | File paths, directories (uploaded recursively), or globs (`**`, `{a,b}`). Relative paths resolve against the project directory. |

**Returns:** `{ matched, succeeded, failed, results[] }`. Each result is
`{ ok, path, filename?, file_id?, error? }`.

**Notes:** Best-effort — each file uploads independently via init → presigned PUT →
complete. A per-file failure leaves no `INITIATED` orphan (best-effort cleanup runs).
Uploaded IDs are recorded for self-echo de-duplication.

### Tool: `pushkit_pull`

Download files to disk and return their local paths. Provide **exactly one** selector.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | one-of | The file with this exact ID. |
| `filename` | string | one-of | The newest file with this exact filename. |
| `newest` | bool | one-of | The single most recently uploaded file. |
| `all_unpulled` | bool | one-of | Every file not pulled before from this backend. |

**Returns:** `{ target_dir, pulled, failed, results[] }`. Each result is
`{ ok, file_id, filename?, path?, error? }`.

**Notes:** Writes into `CLAUDE_PROJECT_DIR`, falling back to `~/.pushkit/downloads`.
Writes are confined to that directory via `os.Root`; path traversal (`..`, absolute
paths, symlink escapes, null bytes) is rejected. Filename collisions get a numeric
suffix (`report-2.pdf`) — existing files are never overwritten. `all_unpulled` tracks
already-pulled IDs per backend in the CLI state file.

### Tool: `pushkit_list`

List your uploaded files, compact and paginated, newest first.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `cursor` | string | no | Opaque pagination cursor from a previous call's `next_cursor`. |
| `limit` | int | no | Max entries to return, 1–100 (default 50). |
| `query` | string | no | Case-insensitive filename substring filter. |

**Returns:** `{ files[], count, next_cursor? }`. Each file is
`{ id, filename, size, created_at }`; `size` may be `null`.

### Tool: `pushkit_delete`

Delete a file (mirrors the backend hard-delete). Select by `id` or `filename`.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | one-of | The file with this exact ID. |
| `filename` | string | one-of | The newest file with this exact filename. |
| `confirm` | bool | no | Must be `true` to delete. `false`/omitted returns a dry-run preview and deletes nothing. |

**Returns:** `{ deleted, file_id, filename?, message }`.

### Resources

| Property | Value |
|----------|-------|
| URI scheme | `pushkit://files/{id}` (one resource per file; a URI template is also registered) |
| Read ≤ 10 MiB | Returns the file's bytes inline with the file's MIME type |
| Read > 10 MiB | Returns JSON metadata (`uri`, `id`, `contentType`, size) plus a note to use `pushkit_pull` — not the bytes |
| Refresh | The list is rebuilt and `resources/list_changed` fired on `file.uploaded` and on (re)connect |

Events for file IDs this server pushed within a short window are suppressed
(self-echo de-duplication), so the agent's own pushes do not trigger a redundant
refresh.

## Backend event stream

### `GET /v1/events`

A server-sent event (SSE) stream of file events for the authenticated user.

| Property | Value |
|----------|-------|
| Method / path | `GET /v1/events` |
| Authentication | `X-API-Key` header (same as other `/v1` endpoints) |
| Scope | Strictly per `user_id` — a user receives only their own events |

**Response headers:**

| Header | Value |
|--------|-------|
| `Content-Type` | `text/event-stream` |
| `Cache-Control` | `no-cache` |
| `Connection` | `keep-alive` |
| `X-Accel-Buffering` | `no` (disables reverse-proxy buffering) |

**Frames:** named SSE events in the form

```
event: file.uploaded
data: {"type":"file.uploaded","id":"...","filename":"...","size":1234,"contentType":"application/pdf","createdAt":"2026-06-24T12:00:00Z"}
```

A heartbeat comment line `: ping` is sent every 15 seconds to keep the connection and
intermediary proxies alive. Comment lines are ignored by `EventSource`.

**`file.uploaded` data:**

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Event type, e.g. `file.uploaded`. |
| `id` | string | File ID. |
| `filename` | string | Original filename. |
| `size` | int \| null | Size in bytes; `null` if unknown. |
| `contentType` | string | MIME type. |
| `createdAt` | string | RFC 3339 timestamp. |

**Semantics:**

- An in-process, single-instance fan-out hub.
- Publishing is non-blocking: a slow consumer whose buffer is full has events
  **dropped** rather than blocking the publisher or other subscribers.
- On reconnect, clients reconcile by re-listing. There is no replay buffer and no
  `Last-Event-ID` support.
- Subscriptions are released on client disconnect (request-context cancellation).
- Emitted by `POST /v1/uploads/complete`, after the file status is updated to
  `UPLOADED`.

**Limits:**

- Single-instance only — the hub is in-process; horizontal scaling would require an
  external bus (out of scope).
- No new backend dependencies — implemented with the standard library and chi.
