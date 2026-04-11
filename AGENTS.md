# AGENTS.md — repbot-go

A Discord bot written in Go that tracks reputation points for server members. Uses discordgo + SQLite.

## Repository layout

```
main.go            — entire bot logic (single file)
config/config.go   — config loading via viper (YAML file or env vars)
Containerfile      — multi-stage build (alpine); requires CGO for sqlite3
.github/workflows/ — CI: builds and pushes container image to GHCR on push to master
```

No test files currently exist.

## Building

Requires CGO (for `go-sqlite3`). On Alpine-based systems, install `gcc` and `musl-dev` first.

```bash
go build -o repbot-go .
```

Running:

```bash
./repbot-go -c /path/to/botconfig
```

## Configuration

Priority order (highest wins): CLI flag > `REPBOT_CONFIG` env var > `$HOME/.config/repbot-go/botconfig` > `./config/botconfig`.

| Variable       | Purpose                        | Default        |
|----------------|--------------------------------|----------------|
| `REPBOT_TOKEN` | Discord bot token (required)   | —              |
| `REPBOT_DB`    | Path to SQLite database file   | `/data/rep.db` |
| `REPBOT_CONFIG`| Path to YAML config file       | —              |

Config file format (YAML):
```yaml
token: your-discord-bot-token
db: /path/to/rep.db
```

## Database schema

**Critical naming quirk — do not migrate away from it:**

```sql
CREATE TABLE reputation (
    username TEXT PRIMARY KEY,  -- stores Discord USER ID (not a display name)
    rep      INTEGER DEFAULT 0,
    user     VARCHAR             -- stores display name (not a user ID)
);
```

The column names are inverted from what you'd expect:
- `WHERE username = ?` → filter by Discord **user ID**
- `SET user = ?` → set the **display name**

All SQL constants are defined at the top of `main.go`. Do not add a migration to rename these columns.

## Bot commands

| Command              | Behavior                                              |
|----------------------|-------------------------------------------------------|
| `!rep @user`         | Give +1 rep to mentioned user (self-rep blocked)      |
| `!rep rank`          | Show top-10 leaderboard (rank, rep, title, username)  |
| `!rep rank @user`    | Show a specific user's rep and leaderboard rank       |
| `!rep me`            | Show calling user's own rep and rank                  |
| `!rep ping`          | Health check — responds `pong`                        |

Emoji reaction: reacting to any message with the `LODLove01` custom emoji gives the message author +1 rep.

## Rank titles

| Rep threshold | Title     |
|---------------|-----------|
| 500+          | Icon      |
| 300–499       | Legend    |
| 200–299       | Champion  |
| 100–199       | Elite     |
| 50–99         | Veteran   |
| 25–49         | Member    |
| 10–24         | Regular   |
| 0–9           | Newcomer  |

Milestone announcements fire at: 10, 25, 50, 100, 200, 300, 400, 500 rep.

## Key types and entry points

- `repbotClient` (`main.go:42`) — top-level struct holding Discord session, DB handle, and config.
- `messageCreate` (`main.go:203`) — Discord message event handler; routes all `!rep` commands.
- `messageReactionAdd` (`main.go:422`) — handles emoji reactions for rep-by-reaction.
- `repInc` (`main.go:319`) — core rep increment logic; creates user on first encounter.
- `config.Get` (`config/config.go:18`) — reads config from file and/or env vars.

## CI / Container

- GitHub Actions builds and pushes to `ghcr.io/stevehnh/repbot-go` on every push to `master`.
- Tags: `latest` and a 7-char short SHA.
- The Containerfile uses a two-stage build; the runtime image is `alpine:latest`.
- Container volume mounts: `/data` (database), `/etc/repbot` (optional config file).
