# Code && Chill

Code && Chill is a Go web application for running puzzle events with:

- Time-based puzzle unlocks
- Per-user progress tracking
- Discord OAuth login
- Admin user management
- Optional Discord notifications

It stores data in BoltDB and serves a website from files in `data/`.

## Features

- Multi-event puzzle support via YAML config
- Puzzle pages with part-by-part answer submission
- Unlock scheduling and latest-puzzle redirect
- Session management with expiration and cleanup
- Admin pages for users and puzzles
- JSON admin API for user listing and updates
- Optional HTTPS with live TLS certificate reload
- Automatic database backups on cron

## Requirements

- Go 1.26+
- A Discord application (for OAuth)
- A Discord bot token (optional, for notifier integration)

## Project Layout

- `cmd/code-and-chill`: main web server
- `cmd/api`: API client CLI for user admin operations
- `cmd/listdb`: inspect BoltDB contents
- `cmd/fakeusers`: seed fake users/progress for testing
- `config.yaml`: runtime configuration
- `data/`: static files, templates, and HTML pages
- `db/`: BoltDB storage location (default: `db/cc.db`)
- `bak/`: database backup output directory
- `secret/`: secret files referenced by `config.yaml`

## Quick Start

1. Create secret files referenced by `config.yaml`:

```text
secret/discord.clientsecret.txt
secret/discord.token.txt
```

2. Update `config.yaml` values for your environment:

- `server.port`, `server.host`, `server.dataDir`
- `auth.discord.clientID`, `auth.discord.redirectURI`, `auth.discord.guildID`
- `notifier.baseURI` and `notifier.discord.channels`
- `puzzles.events[*].config` paths

3. Run the app:

```bash
go run ./cmd/code-and-chill
```

To use a custom config file:

```bash
go run ./cmd/code-and-chill path/to/config.yaml
```

## Build and Test

Build all commands into `bin/`:

```bash
./build.sh
```

Run tests:

```bash
./test.sh
```

Equivalent direct commands:

```bash
GOBIN=$PWD/bin go install ./...
go test -timeout 30s ./... -v
```

## Configuration

The app uses strict YAML decoding, so unknown fields in `config.yaml` will fail startup.

Top-level sections:

- `server`
- `db`
- `session`
- `auth`
- `notifier`
- `puzzles`

Key settings:

- `server.port`: main web server port
- `server.apiPort`: localhost-only admin API port (`0` disables separate API server)
- `server.tlsCertFile` + `server.tlsKeyFile`: enable HTTPS
- `server.tlsReloadSchedule`: cron schedule to reload TLS cert/key
- `server.httpsRedirect`: optional HTTP :80 -> HTTPS redirect
- `db.backupSchedule`: cron schedule for DB backups
- `session.expire`, `session.truncate`, `session.cleanupSchedule`: session lifecycle
- `auth.discord.clientSecret`: path to file containing Discord client secret
- `notifier.discord.token`: path to file containing Discord bot token
- `puzzles.default`: default event path
- `puzzles.events[*].config`: event configuration file path

## Puzzle/Event Config

Puzzle definitions are loaded from the files referenced in `puzzles.events[*].config`.

Event config includes:

- event `id`, `name`
- list of puzzles with unlock times and puzzle config file paths

Puzzle config includes:

- puzzle `id`, `path`, `name`
- `parts`: markdown files for each part
- `inputs`: input files plus expected answers per part

## Admin API

Two access patterns exist:

- Main server routing under `/api/*` (admin web context, requires admin account)
- Separate localhost API server on `server.apiPort` (direct endpoints)

Direct endpoints (no `/api` prefix):

- `GET /users`
- `GET /users?name=<query>`
- `GET /user/{id}`
- `POST /user/{id}`

Example update payload:

```json
{
	"admin": true
}
```

## API CLI

Run the helper CLI against the API port (default shown in `config.yaml`: `1274`):

```bash
go run ./cmd/api --port 1274 user list
go run ./cmd/api --port 1274 user find "alice"
go run ./cmd/api --port 1274 user get <user-id>
go run ./cmd/api --port 1274 user update <user-id> --admin
go run ./cmd/api --port 1274 user update <user-id> --unset-admin
```

## Database Utilities

Inspect DB content:

```bash
go run ./cmd/listdb
go run ./cmd/listdb path/to/db-file
```

Seed fake users/progress (development/testing):

```bash
go run ./cmd/fakeusers
go run ./cmd/fakeusers path/to/db-file
```

## Notes

- The server expects website content and templates under `server.dataDir` (default: `data`).
- If `server.host` is set, host validation middleware is enabled.
- DB backups are triggered by cron and also once on process shutdown.
