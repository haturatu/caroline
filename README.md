# Caroline
<img width="1341" height="629" alt="top" src="https://github.com/user-attachments/assets/d89ad9db-64fa-45c4-93af-3e7329bdca6e" />

Caroline is a lightweight Hub and Agent system for searching and inspecting stdout/stderr from Docker containers across multiple hosts. It is inspired by the information architecture of Google Cloud Logs Explorer, but its query syntax and data model are intentionally a small subset designed for Docker logs.

See the [Japanese README](README.ja.md).

## Why Caroline?

Most of my servers are managed with containers, but I wanted a single place to inspect their logs. When checking logs remotely, I had been using `Beszel` to inspect one container at a time, which was not ideal for log management. Caroline started as a personal tool with a Logs Explorer-like UI/UX, based on the parts of GCP Cloud Logging that I find useful.

The name came from listening to The Velvet Underground's “Caroline” while working on the project.

## Features

- A Hub that stores normalized logs in SQLite
- A read-only Agent that discovers containers and follows Docker logs on each host
- At-least-once delivery with bounded batches, retry, disk spool, and duplicate suppression
- Node-aware search, container grouping, and Agent online/offline status
- Filters by container, stdout/stderr stream, severity, and time range
- Full-field search and Caroline Query Syntax
- Timeline, field aggregation, and a log detail drawer
- Streaming display of new logs over SSE
- Threshold-based log alerts with automatic Discord, Slack, ntfy, Microsoft Teams, and generic webhook delivery
- Share links that preserve the current search
- Dark and light themes with mobile navigation
- English, Japanese, Simplified Chinese, Traditional Chinese, and Russian UI
- Agents use read-only access to their local Docker Engine; the Hub does not need the Docker socket

## Internationalization

The UI supports English, Japanese, `zh-CN`, `zh-TW`, and Russian. Choose a language from the header menu; the selection is stored in `localStorage` as `caroline-locale`. When no language has been selected, Caroline uses `navigator.languages` and `navigator.language` for detection. Query syntax remains locale-independent, while dates, numbers, and plural forms use the browser's standard `Intl` APIs. The language preference is not included in share links.

## Getting started

### Docker Compose

~~~sh
docker compose up -d --build
~~~

Open <http://localhost:8080> in a browser.

The default Compose service is the Hub. It stores logs and identities in the `caroline-data` volume and does not mount the Docker socket. An Agent is a separate Compose profile because it must be placed on the Docker host whose logs it should collect:

~~~sh
docker compose up -d --build caroline
~~~

Generate an enrollment token from the Hub's node API, then start the Agent with that token and the Hub public key. The Agent is read-only against `/var/run/docker.sock` and keeps its identity and offline spool in `caroline-agent-data`.

To stop Caroline:

~~~sh
docker compose down
~~~

### Run locally

Requirements:

- Go 1.26 or later
- Node.js 22 or later and npm
- An accessible Docker Engine (only required by `caroline-agent`)

~~~sh
npm ci
npm run build
go run ./cmd/caroline

# On each Docker host, after configuring CAROLINE_HUB_URL and enrollment:
go run ./cmd/caroline-agent
~~~

The server listens on <http://localhost:8080> by default.

## Environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | Port used by the web server |
| `CAROLINE_DATA_DIR` | `.` | Hub data directory for the SQLite database, Hub key, and default alert file |
| `CAROLINE_DB` | `$CAROLINE_DATA_DIR/caroline.db` | SQLite database path |
| `CAROLINE_HUB_KEY` | `$CAROLINE_DATA_DIR/hub.key` | Hub Ed25519 private key |
| `CAROLINE_RETENTION` | disabled | Log retention duration, for example `7d` |
| `CAROLINE_MAX_STORAGE_SIZE` | disabled | Logical retained log payload budget, for example `10GiB` |
| `ALERTS_FILE` | `alerts.json` | JSON file used to persist alert rules and state |
| `CAROLINE_HUB_URL` | — | Agent: Hub base URL |
| `CAROLINE_ENROLLMENT_TOKEN` | — | Agent: single-use registration token |
| `CAROLINE_HUB_PUBLIC_KEY` | — | Agent: base64 raw Ed25519 Hub public key; required unless TOFU is explicitly enabled |
| `CAROLINE_AGENT_STATE_DIR` | `/var/lib/caroline-agent` | Agent identity, persistent Hub pin (`hub.json`), and disk spool directory |
| `CAROLINE_AGENT_SPOOL_MAX_SIZE` | `1GiB` | Agent spool size limit; oldest batches are dropped after the limit |
| `CAROLINE_AGENT_SPOOL_MAX_AGE` | `24h` | Agent spool age limit |
| `CAROLINE_AGENT_TRUST_ON_FIRST_USE` | `false` | Allow the first Hub key to be pinned automatically; prefer `CAROLINE_HUB_PUBLIC_KEY` |
| `CAROLINE_AGENT_COMPRESSION` | `gzip` | Agent batch transport: `identity`, `gzip`, or `zstd` |
| `DOCKER_HOST` | Docker default | Agent Docker Engine endpoint |

`DOCKER_HOST` supports `unix://`, `tcp://`, `http://`, and `https://` endpoints. When using a TCP or HTTP connection, configure authentication, TLS, and network controls on the Docker Engine side as appropriate.

The Hub can run without a Docker socket. In Hub mode `/api/status` reports `mode: "hub"`; logs arrive from authenticated Agents. For a single-host deployment, run an Agent beside the Hub rather than granting the Hub access to the Docker socket.

### Registering an Agent

Create a short-lived, single-use token from the Hub:

~~~sh
curl -X POST http://localhost:8080/api/nodes \
  -H 'Content-Type: application/json' \
  -d '{"ttlSeconds":900}'
~~~

Read `hubPublicKey` from `GET /api/health`, encode its raw bytes with base64, and configure the Agent with `CAROLINE_HUB_URL`, `CAROLINE_ENROLLMENT_TOKEN`, and `CAROLINE_HUB_PUBLIC_KEY`. The Agent registers once, verifies a signed Hub challenge, and subsequently signs every request with its persistent Ed25519 key. If TOFU is enabled, the verified Hub key is persisted in `hub.json` under `CAROLINE_AGENT_STATE_DIR` and reused after restart. Protect the node-management endpoints with the deployment's access-control layer before exposing them outside a trusted network.

## Using the UI

### Filters

- **Container**: Select running containers
- **Node**: Select one or more Docker hosts
- **Stream**: `stdout` or `stderr`
- **Severity**: Exact matches for `Errors`, `Warnings`, `Info`, and `Debug`
- **Time**: 5 minutes, 15 minutes, 1 hour, 6 hours, 24 hours, 7 days, or a custom range
- **Search Logs**: Search text across all supported fields

Preset changes to Container, Stream, Severity, and Time run immediately. For a custom time range, enter From and To and press **Apply**. Sort changes also apply immediately. Search Logs runs 300 ms after input stops.

Edit advanced conditions from **Show Query**. Run an advanced query with **Run Query**, or press `Ctrl + Enter` / `Cmd + Enter` in the query editor. Basic filters and the Advanced Query are combined with `AND`.

**Reset Filters** returns the search text, container, stream, severity, time range, and Advanced Query to their initial state.

### Streaming

When Streaming is enabled, Caroline opens an SSE connection to `/api/tail` after the initial search and appends new logs to the page. It does not poll every five seconds.

Stopping Streaming closes the SSE connection. Changing filters, the time range, sort order, or query closes the current stream and reconnects from a new search result. If the connection drops, the browser's `EventSource` automatically attempts to reconnect.

### Alerts

Create an alert from the current query with a threshold, time window, cooldown, severity, labels, an optional runbook URL, and a sample redaction mode. The UI keeps only `Runbook URL` and `Webhook URL`; Caroline detects the webhook provider from the HTTPS host and path. It supports Discord (`discord.com/api/webhooks/...`), Slack Incoming Webhooks (`hooks.slack.com/services/...`), ntfy topics (`ntfy.sh/<topic>`), Microsoft Teams Workflows (`*.logic.azure.com/.../paths/invoke` and `*.api.powerplatform.com/powerautomate/...`), and generic JSON for other URLs. Set `CAROLINE_URL` to the public Caroline URL to include a time-bounded Explorer deep link in notifications. The alert engine consumes the same shared Docker `follow` streams as SSE, so each running container has at most one Caroline-side follow stream regardless of how many alert rules use it.

Use **Manage Alerts** in Explorer to open the dedicated policy management view. It provides policy, firing, enabled, and notification summaries; name/query search; status filtering; edit; pause/resume; and delete actions. Webhook URLs remain hidden from list and API responses, and partial edits retain an existing webhook unless it is explicitly removed.

Rules and alert state are persisted to the JSON file configured by `ALERTS_FILE` and restored on startup. Caroline does not store log bodies or matching entries themselves; it keeps timestamps and small incident metadata such as firing start, peak count, and container name. A rule transitions between `OK` and `FIRING`, and sends a webhook notification for firing and resolution events when a webhook is configured. Docker Compose stores the file in the `caroline-data` volume.

### Timeline, Fields, and Logs

- **Timeline**: Splits the result range into 24–96 responsive buckets based on the timeline width and displays them by severity. Click a bar or drag across a range to change the time range. Zoom controls are also available.
- **Fields**: Aggregates System Metadata and Frequent Fields in the result set. Clicking a field value adds it to the Advanced Query.
- **Logs**: Supports newest/oldest sorting, Wrap Lines, and Load More.
- Selecting a row opens a detail drawer with the Summary, Payload, metadata, and copyable Entry JSON.

On desktop and mobile, use the menu button in the upper-left corner to open the navigation drawer. The Explore and Detect categories are collapsible and expose the implemented Logs, Timeline, Fields, and Alerts views. Fields appears as a bottom sheet on mobile.

### Keyboard shortcuts

- `/`: Focus Search Logs
- `Ctrl + Enter` / `Cmd + Enter`: Run the Advanced Query
- `j` / `ArrowDown`: Move to the next log row
- `k` / `ArrowUp`: Move to the previous log row
- `Home` / `End`: Move to the first or last log row
- `Escape`: Close the detail drawer, mobile navigation, or menu

## Caroline Query Syntax

Caroline's query language is inspired by Cloud Logging, but is not fully compatible with it.

~~~text
severity >= ERROR
resource.labels.container_name = "nginx"
stream = stderr
SEARCH("timeout")
jsonPayload.status >= 500
timestamp >= "2026-08-10T00:00:00Z"
~~~

### Operators

The supported operators are:

~~~text
=   !=   :   >   <   >=   <=
~~~

- `=` / `!=`: Case-insensitive comparison
- `:`: Case-insensitive substring match
- Comparisons on `severity`: Compare severity ranks
- Comparisons on `timestamp`: Compare RFC3339 timestamps
- A newline: Treated as `AND`
- `AND` / `OR`: Combine clauses

Supported fields include `severity`, `stream`, `logName`, `resource.type`, `resource.labels.node_id`, `resource.labels.node_name`, `resource.labels.container_name`, `resource.labels.container_id`, `resource.labels.image`, `timestamp`, `labels.*`, `jsonPayload.*`, and `textPayload` / `message`. Short names such as `node`, `container`, and `container.name` are also supported.

`SEARCH("text")` searches the Summary, Text Payload, Log Name, resource type and labels, and JSON Payload. `NOT`, parenthesized precedence, regular expressions, and Cloud Logging-specific functions are not supported.

## Data model and limits

Docker log lines are normalized into a format similar to a Cloud Logging LogEntry:

- `resource.type`: `docker_container`
- `resource.labels`: Node ID/name, container name/ID, and image
- `logName`: `containers/<container>/<stream>`
- `severity`: Estimated as `DEBUG`, `INFO`, `WARNING`, or `ERROR` from keywords in the log body
- `stream`: `stdout` or `stderr`
- Lines that can be parsed as JSON also retain a `jsonPayload`

Severity is not a standard Docker attribute. It is estimated from keywords in the log body, including `ERROR`, `FATAL`, `PANIC`, `WARN`, `DEPRECATED`, `DEBUG`, and `TRACE`.

The following limits keep searches bounded:

- Logs read per container: The latest 1,000 lines
- Maximum entries in one response: 50,000
- Maximum page size for `/api/explorer`: 1,000 entries (the UI normally requests 100)
- Concurrent Docker Engine search requests: 8
- Containers included in Streaming: 8
- Maximum size of one log payload or follow frame: 8 MiB

Because of these limits, the response's `approximate` field indicates that the search is approximate. The UI shows this as `Partial`, where the per-container line limit can be inspected. Expanding the time range does not retrieve unlimited historical logs from a container.

Pagination uses an opaque `nextPageToken` based on the timestamp and insert ID. Keep the same search conditions, time range, and sort order when requesting the next page.

## API

Read APIs support GET and also accept HEAD requests. Alert rules additionally use POST, PUT/PATCH, and DELETE.

### `GET /api/health`

Returns a health response for the Caroline server.

~~~json
{"ok":true,"service":"caroline"}
~~~

### `GET /api/status`

Returns the Hub mode or Docker Engine connection state, Docker version, API version, and check time. A Hub response has `mode: "hub"`; its `connected` field is not a Docker health signal.

### `GET /api/explorer`

Returns a snapshot of search results.

Main query parameters:

| Parameter | Description |
| --- | --- |
| `duration` | `5m`, `1h`, `7d`, `30d`, Go durations, and similar values; maximum 30 days |
| `from` / `to` | Range specified as RFC3339 timestamps |
| `q` | Caroline Query Syntax |
| `severity` | Severity filter |
| `stream` | `stdout` / `stderr` |
| `containers` | Container names, short IDs, or full IDs, comma-separated |
| `nodes` | Node IDs or names, comma-separated |
| `sort` | `asc` or `desc` |
| `limit` | 1–1,000; default 100 |
| `pageToken` | The previous response's `nextPageToken` |

Example:

~~~sh
curl 'http://localhost:8080/api/explorer?duration=15m&limit=100&q=severity%20%3E%3D%20ERROR&sort=desc'
~~~

In addition to `entries`, the response includes `containers`, a responsive 24–96-bucket `timeline`, `fields`, `total`, `generatedAt`, `from`, `to`, `approximate`, `logTail`, `entryLimit`, and `truncated`.

### `GET /api/tail`

Returns new logs as Server-Sent Events.

Main query parameters are `since`, `q`, `severity`, `stream`, `containers`, and `nodes`. `since` is an RFC3339 timestamp.

Events:

- `ready`: Number of target containers and the stream start time
- `log`: A new log entry, using the same entry shape as `/api/explorer`
- `warning`: A warning such as the maximum number of Streaming targets
- `error`: A Streaming error for a specific container
- An SSE keep-alive comment is sent every 15 seconds while the connection is open.

The SSE endpoint and alert engine share a `logstream.Manager`; a browser connection does not create another Docker follow stream for a container already watched by Caroline.

### `/api/alerts`

Alert rules are persisted in the JSON file configured by `ALERTS_FILE`. `GET /api/alerts` lists rules, `POST /api/alerts` creates one, `GET` or `PUT` `/api/alerts/{id}` reads or replaces one, `PATCH` `/api/alerts/{id}` updates selected fields, and `DELETE` `/api/alerts/{id}` removes one. Omitted fields in a PATCH are retained. Because the file contains webhook URLs, Caroline writes it with `0600` permissions.

Create a rule with JSON such as:

~~~json
{
  "name": "nginx errors",
  "query": "resource.labels.container_name = \"nginx\" AND severity >= ERROR",
  "severity": "critical",
  "labels": {
    "service": "nginx",
    "environment": "production"
  },
  "runbookUrl": "https://runbooks.example.test/nginx-errors",
  "sampleMode": "summary",
  "threshold": 5,
  "windowSeconds": 60,
  "cooldownSeconds": 600,
  "webhookUrl": "https://alerts.example.test/caroline"
}
~~~

Webhook payloads contain `alert.firing` or `alert.resolved`, the stable rule ID, severity and labels, rule query, current and peak match counts, threshold, window, container, firing start time, timestamp, Explorer URL, runbook URL, and a redacted sample entry when enabled. `sampleMode` can be `off`, `summary`, or `full`; full samples are sanitized before leaving Caroline. Webhook URLs are not returned by the API.

Discord Incoming Webhooks follow Discord's [Execute Webhook](https://docs.discord.com/developers/resources/webhook#execute-webhook) format, including an `embeds` message and `allowed_mentions: {"parse": []}`. Caroline also sends `wait=true` so Discord confirms message creation. Slack uses the official [Incoming Webhooks](https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/) JSON and Block Kit format, ntfy uses the official [publishing headers and message body](https://docs.ntfy.sh/publish/), and Teams uses the Microsoft [Teams webhook connector](https://learn.microsoft.com/en-us/connectors/teams/) message envelope with an Adaptive Card. Unknown webhook hosts retain the generic JSON payload.

## Development

~~~sh
npm ci
npm run typecheck
npm run build
go test ./...
~~~

### Directory layout

~~~text
.
├── cmd/caroline/        # Go entrypoint
├── internal/docker/     # Docker Engine client and log frame processing
├── internal/explorer/   # Normalization, search, Timeline, and filters
├── internal/logstream/  # Shared Docker follow streams and subscribers
├── internal/alert/      # JSON-persisted alert engine and webhook notifier
├── internal/httpserver/ # HTTP API, SSE, alerts, and static file serving
├── web/                 # Frontend application
│   ├── index.html       # Vite application entry
│   ├── public/          # Optional files copied as-is by Vite
│   └── src/
│       ├── app/         # Bootstrap and URL state
│       ├── features/    # Explorer, filters, timeline, logs, and streaming
│       ├── shared/      # API, DOM, formatting, i18n, and types
│       ├── ui/          # JSX application shell
│       └── styles/      # CSS layers
├── static/              # Generated Vite serving assets from npm run build
├── Dockerfile
└── docker-compose.yml
~~~

## Security and operational notes

Caroline has no authentication or authorization. It runs with access to the Docker socket, so expose port 8080 only to a trusted local network. When using a TCP Docker Engine, configure TLS, authentication, and firewall controls outside Caroline.

Caroline does not store log bodies or search results. Each request reads from the Docker Engine and displays the result as browser state. Alert configuration and matching timestamps are stored in `ALERTS_FILE`.

The UI uses IBM Plex Sans. IBM Plex Mono is reserved for log bodies, timestamps, the query editor, container IDs, field names, and other monospace values. If Google Fonts is unavailable, the UI falls back to system fonts. Caroline uses Container Queries, `subgrid`, CSS Nesting, `dvh`, `light-dark()`, and other modern CSS features, so a relatively recent browser is recommended.
