# How to connect Claude Code to PushKit

## When to use this guide

Use this guide when you want a Claude Code agent to share files with your phone:
push files from a coding session into PushKit, pull files the phone uploaded, list
them, and `@`-mention them — with the agent's file view refreshing automatically
when a new file arrives.

It assumes you already run PushKit and use Claude Code. It does **not** explain how
the MCP server or event stream work internally — see the
[MCP & events reference](mcp-and-events-reference.md) for that.

## Prerequisites

- The `pushkit` CLI installed and on your `PATH` (`pip install --pre pushkit`, or
  build from `cli/`). Verify with `pushkit --help`.
- A reachable PushKit backend and an API key (see the [backend guide](../backend/README.md)).
- Claude Code.

The MCP server *is* the `pushkit` CLI's hidden `mcp` subcommand: it speaks MCP over
stdio and reuses your CLI configuration. You never start it yourself — Claude Code
launches it.

## Step 1 — Decide how the server gets your credentials

The server resolves the API URL and key exactly like the CLI does:
**flags > environment > config file**. For Claude Code, environment variables are the
most portable choice. Set:

- `PUSHKIT_API_KEY` — your API key.
- `PUSHKIT_API_URL` — your backend's base URL (optional if you have already run
  `pushkit config set`).

Never commit the key.

## Step 2 — Register the server (pick one path)

### Option A — project `.mcp.json` (checked in, shared with the repo)

PushKit ships a template at the repository root:

```json
{
  "mcpServers": {
    "pushkit": {
      "command": "pushkit",
      "args": ["mcp"],
      "env": {
        "PUSHKIT_API_KEY": "${PUSHKIT_API_KEY}",
        "PUSHKIT_API_URL": "${PUSHKIT_API_URL:-http://localhost:8080}"
      }
    }
  }
}
```

`${PUSHKIT_API_KEY}` is expanded from your environment when the server launches, so
no secret is stored in the file. Export `PUSHKIT_API_KEY` (and `PUSHKIT_API_URL` if
your backend is not at the default) before starting Claude Code in this project.

### Option B — `claude mcp add` (per-user, any directory)

```bash
claude mcp add pushkit \
  --env PUSHKIT_API_KEY=sk-your-key \
  --env PUSHKIT_API_URL=https://your-server.example.com \
  -- pushkit mcp
```

## Step 3 — Confirm the tools are available

Start (or restart) Claude Code. The `pushkit` server exposes four tools —
`pushkit_push`, `pushkit_pull`, `pushkit_list`, `pushkit_delete` — plus read-only
file resources. Ask the agent to list your files, or invoke `pushkit_list`.

The server starts even when the key is missing or invalid, or the backend is
unreachable. The problem surfaces as a clear error on the first tool call, not as a
server that fails to load.

## Step 4 — Push, pull, and `@`-mention

- **Push:** "push `./report.pdf` to pushkit" — uploads via the tool; directories and
  globs (`**`, `{a,b}`) are expanded.
- **Pull:** "pull the newest pushkit file" — writes into `CLAUDE_PROJECT_DIR` (or
  `~/.pushkit/downloads`) and returns the path. Name collisions get a numeric suffix;
  the agent never receives raw bytes inline.
- **List:** "list my pushkit files" — compact and paginated.
- **`@`-mention:** type `@` and pick a `pushkit://files/{id}` resource to read a file
  into context. Files over 10 MiB come back as metadata plus a download instruction.

## Step 5 — Verify auto-refresh

With the server connected, upload a file from your phone (or another CLI session).
The agent's resource list updates on its own — you should not have to re-ask. Behind
the scenes the server subscribes to the backend event stream and emits
`resources/list_changed`.

## Validation checklist

- `pushkit_list` returns your `UPLOADED` files.
- A pushed file appears in the list and on the phone.
- A phone upload appears in the agent's view without re-asking.
- An `@`-mentioned `pushkit://files/{id}` resolves and reads.

## Troubleshooting

- **Auth or connection errors on a tool call** — the key or URL is wrong, or the
  backend is down. Fix `PUSHKIT_API_KEY` / `PUSHKIT_API_URL`; the corrected values are
  picked up on the server's next launch.
- **The server does not appear** — confirm `pushkit` is on the `PATH` Claude Code
  sees, and that `pushkit mcp` runs (it blocks waiting on stdio; Ctrl-C to exit).
- **Default URL mismatch** — the `.mcp.json` template falls back to
  `http://localhost:8080`. Set `PUSHKIT_API_URL` to match your backend (the local
  docker-compose backend listens on `:8000`).
- **Large files are not read inline** — by design (10 MiB cap). Use `pushkit_pull`
  with the file id to download it.

## Related

- [MCP & events reference](mcp-and-events-reference.md) — exact tool inputs/outputs,
  resource behavior, and the `GET /v1/events` contract.
- [CLI guide](../cli/README.md) — the rest of the `pushkit` commands.
