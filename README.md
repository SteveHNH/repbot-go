# repbot-go

A Discord bot that tracks reputation points for server members.

## Commands

| Command | Description |
|---|---|
| `!rep @user` | Give a reputation point to a user |
| `!rep rank` | Show the top 10 users by reputation |
| `!rep ping` | Health check — bot responds with `pong` |

A user cannot give rep to themselves.

## Configuration

Configuration can be provided via a YAML file, environment variables, or a mix of both. Environment variables take priority over file values.

### Config file

```yaml
token: your-discord-bot-token
db: /path/to/rep.db
```

The bot searches for the config file in the following order:

1. Path passed via `-c` flag
2. Path in the `REPBOT_CONFIG` environment variable
3. `$HOME/.config/repbot-go/botconfig`
4. `./config/botconfig`

If no config file is found and `REPBOT_TOKEN` is set, the bot will start using environment variables only.

### Environment variables

| Variable | Description | Default |
|---|---|---|
| `REPBOT_TOKEN` | Discord bot token | *(required)* |
| `REPBOT_DB` | Path to the SQLite database file | `/data/rep.db` |
| `REPBOT_CONFIG` | Path to a config file | *(none)* |

### Flags

| Flag | Description |
|---|---|
| `-c <path>` | Explicit path to a config file |

## Running Locally

```bash
go build -o repbot-go .
./repbot-go -c /path/to/botconfig
```

The database schema is created automatically on first run if it does not exist.

## Container

### Build

```bash
podman build -t repbot-go -f Containerfile .
```

### Run with a mounted config file

```bash
podman run -d \
  --name repbot \
  -e REPBOT_CONFIG=/etc/repbot/botconfig \
  -v ./botconfig:/etc/repbot/botconfig:Z \
  -v ./rep.db:/data/rep.db:Z \
  repbot-go
```

### Run with environment variables only (no config file)

```bash
podman run -d \
  --name repbot \
  -e REPBOT_TOKEN=your-discord-bot-token \
  -v ./rep.db:/data/rep.db:Z \
  repbot-go
```

> `:Z` is required on SELinux hosts (Fedora/RHEL). It is a no-op elsewhere.

If `rep.db` does not exist yet, create it first — the bot will initialize the schema on startup:

```bash
touch ./rep.db
```

The container exposes two volume mount points:

| Path | Purpose |
|---|---|
| `/data` | SQLite database (`rep.db`) |
| `/etc/repbot` | Optional config file |
