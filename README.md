# rigr

`rigr` is a small homelab service that inspects **running Docker containers**, reads their labels, fetches **RSS/Atom release feeds**, and reports **available updates** via HTTP API in **JSON** and **RSS** formats.

## How it discovers apps

Only containers with the required label are tracked.

- Required: `rigr.release_feed` — absolute URL to an RSS or Atom feed (e.g. GitHub `releases.atom`).
- Optional: `rigr.name` — override the displayed app name (otherwise the container name is used).

The label prefix is configurable via `LABEL_PREFIX` (default: `rigr.`).

## Version matching behavior

`rigr` uses the **image tag** as the current version source, but compares by an extracted **match version** (defaults to semver core `X.Y.Z`).

Examples:
- `ghcr.io/org/app:v1.2.3` → match version `1.2.3`
- `ghcr.io/actualbudget/actual:26.5.0-alpine` → match version `26.5.0` (variant suffix is ignored by default)

- If a matching entry is found, all entries **newer** (above it in the feed) are returned as `updates_available`.
- If no matching entry is found, the app is reported with `match_status: "no_match"`, and `latest_known_release` is returned, but `updates_available` stays empty (to avoid false positives).

### Optional labels for version extraction

You can override how versions are extracted using regex labels (useful when tags/titles are not plain semver):

- `rigr.image_version_regex`: regex applied to the **image tag**.
- `rigr.feed_version_regex`: regex applied to **feed item title/link**.

Rules:
- If the regex matches and has a capture group 1, that group is used as the version.
- Otherwise the full match is used.
- For feed matching, title is tried first; if it doesn't match, link is tried.

Defaults:
- `rigr.image_version_regex`: `(?i)\\bv?(\\d+\\.\\d+\\.\\d+)\\b`
- `rigr.feed_version_regex`: `(?i)\\bv?(\\d+\\.\\d+\\.\\d+)\\b`

## API

- `GET /health`
- `GET /api/v1/apps`
- `GET /api/v1/apps/{container_id}`
- `GET /api/v1/homepage/updates` (Homepage-friendly JSON)
- `GET /feed.xml` (RSS)
- `GET /api/v1/updates.rss` (RSS alias)

### Homepage (gethomepage/homepage) example

This endpoint is designed for the `customapi` **dynamic-list** widget.

- It returns a **root JSON array**.
- It includes **only apps with confirmed updates** (`updates_available` is non-empty).
- Items are sorted by newest `published_at` first, then by `container_name`.

Example widget config snippet:

```yaml
- Updates:
    icon: si-docker
    widget:
      type: customapi
      url: http://rigr:8080/api/v1/homepage/updates
      method: GET
      display: dynamic-list
      mappings:
        name: container_name
        label: version_line
        target: "{changelog_url}"
```

## Configuration (env)

- `LABEL_PREFIX` (default `rigr.`)
- `POLL_INTERVAL` (default `15m`)
- `HTTP_BIND` (default `0.0.0.0:8080`)
- `HTTP_TIMEOUT` (default `10s`)
- `USER_AGENT` (default `rigr/0.1 (+https://github.com/)`)
- `MAX_FEED_ENTRIES` (default `50`)
- `UPDATE_SEVERITY_ENABLED` (default `false`) — when enabled, classifies update severity from feed item text and:
  - adds `severity` to `updates_available` items in JSON (`default|breaking_changes|security_fixes`)
  - prefixes RSS item titles and homepage `version_line` with emoji for non-default severity (`💥` breaking, `🔒` security)
- `LOG_LEVEL` (default `info`) — `debug|info|warn|error`

## Configuration (CLI)

- `--log-level` — `debug|info|warn|error` (overrides `LOG_LEVEL`)

## Security note

To read container metadata, `rigr` typically needs access to the Docker socket. In a homelab this is often acceptable, but the socket effectively grants root-equivalent control over the host.

