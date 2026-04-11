# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added
- `!rep rank @username` command shows a specific user's rep and leaderboard rank
- `LODLove01` emoji reaction gives rep to the message author; self-rep and bot messages are ignored
- `!rep me` command shows the caller's current rep and numeric leaderboard rank
- Milestone announcements when a user hits 10, 25, 50, 100, 200, 300, 400, or 500 rep
- Rank titles in `!rep rank` leaderboard (Newcomer → Regular → Member → Veteran → Elite → Champion → Legend → Icon)

### Fixed
- All `db.Query()` errors were silently swallowed; now checked and logged throughout
- `rows.Close()` was never called after queries, causing resource leaks
- `rows.Err()` was never checked after iteration loops
- `checkUser()` returned false for any user with exactly 0 rep due to `repValue > 0` check; now correctly checks row existence
- Regex patterns were recompiled on every Discord message; moved to package-level compiled vars
- `defer db.Close()` and `defer ds.Close()` were registered before their respective error checks, risking a nil dereference on failure
- Prepared statements in `updateUsers()` were never closed and a failed `Prepare()` would cause a nil pointer panic
- `msg.Author` can be nil for system messages (pins, boosts, etc.); added nil guard in reaction handler
- `signal.Notify` included uncatchable `os.Kill` (SIGKILL) and duplicate `os.Interrupt`; removed both

### Changed
- `go.mod` updated from `go 1.15` to `go 1.21`
- `!rep rank` leaderboard table columns changed from `Rep | User` to `# | Rep | Title | User`
- Milestone rep confirmations replace the standard "Rep increased" message instead of sending both

---

## [2026-03-01]

### Added
- Containerfile for Podman/Docker multi-stage builds
- README with usage instructions, configuration reference, and container setup

### Fixed
- Nil pointer panic on startup
- Discord rate limit response parsed incorrectly when value was a float

---

## [2021-10-13]

### Added
- On startup, bot checks the database for users whose display names are out of date and syncs them from Discord

---

## [2020-12-08]

### Fixed
- `!rep rank` now correctly returns the top 10 users instead of an unbounded result

---

## [2020-12-07]

### Added
- Config file path can be set via `-c` CLI flag or `REPBOT_CONFIG` environment variable

---

## [2020-12-03]

### Added
- `!rep rank` command displays a formatted leaderboard of the top 10 users by rep
- Logging throughout message handling and database operations

### Fixed
- Unnecessary `else` statements removed; functions now return early on database errors

---

## [2020-11-25]

### Added
- Core reputation bot: `!rep @user` to give rep, `!rep ping` health check
- SQLite-backed reputation storage with automatic table creation on first run
- Configuration via YAML file with support for `REPBOT_TOKEN` and `REPBOT_DB` environment variable overrides
- Go modules support
- Config file resolved from `$HOME/.config/repbot-go/`, `./config/`, or home directory
