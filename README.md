# rigr

`rigr` is a small homelab service that inspects **running Docker containers**, reads their labels, fetches **RSS/Atom release feeds**, and reports **available updates** via HTTP API in **JSON** and **RSS** formats.

## How it discovers apps

Only containers with the required label are tracked.

- Required: `rigr.release_feed` — absolute URL to an RSS or Atom feed (e.g. GitHub `releases.atom`).
- Optional: `rigr.name` — override the displayed app name (otherwise the container name is used).

The label prefix is configurable via `LABEL_PREFIX` (default: `rigr.`).

## Version matching behavior

`rigr` uses the **image tag** (e.g. `ghcr.io/org/app:v1.2.3` → `v1.2.3`) as the current version and tries to find a matching entry in the feed.

- If a matching entry is found, all entries **newer** (above it in the feed) are returned as `updates_available`.
- If no matching entry is found, the app is reported with `match_status: "no_match"`, and `latest_known_release` is returned, but `updates_available` stays empty (to avoid false positives).

## API

- `GET /health`
- `GET /api/v1/apps`
- `GET /api/v1/apps/{container_id}`
- `GET /feed.xml` (RSS)
- `GET /api/v1/updates.rss` (RSS alias)

## Configuration (env)

- `LABEL_PREFIX` (default `rigr.`)
- `POLL_INTERVAL` (default `15m`)
- `HTTP_BIND` (default `0.0.0.0:8080`)
- `HTTP_TIMEOUT` (default `10s`)
- `USER_AGENT` (default `rigr/0.1 (+https://github.com/)`)
- `MAX_FEED_ENTRIES` (default `50`)

## Security note

To read container metadata, `rigr` typically needs access to the Docker socket. In a homelab this is often acceptable, but the socket effectively grants root-equivalent control over the host.

