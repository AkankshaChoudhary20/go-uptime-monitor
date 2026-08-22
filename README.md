# go-uptime-monitor

A small uptime/health monitor: watches a list of HTTP endpoints
concurrently, tracks up/down state, and fires a webhook alert **on state
transitions only** (so a target that's been down for an hour doesn't spam
your alert channel every check interval).

## The problem this solves

You have a handful of services/endpoints and want to know the moment one
goes down (and the moment it recovers) without paying for a SaaS uptime
monitor or standing up Prometheus + Alertmanager for a small project.

## How it works

- One goroutine per target, each on its own ticker/interval.
- A target's state only changes between `up`/`down` when its status
  actually flips; alerts fire exactly once per transition, not once per
  check.
- `GET /status` exposes the current state of every target as JSON.
- Alerts go to a webhook URL (Slack incoming webhook, or anything that
  accepts a JSON POST) if configured, otherwise they're just logged to
  stdout.
- Zero external dependencies -- entirely Go standard library.

## Run it

```bash
cp config.example.json config.json
# edit config.json with real targets and (optionally) a webhook_url
go run . -config config.json -addr :8080
```

Then:
```bash
curl localhost:8080/status
```

## Config

```json
{
  "webhook_url": "https://hooks.slack.com/services/...",
  "targets": [
    { "name": "homepage", "url": "https://example.com", "interval_seconds": 30, "timeout_seconds": 5 }
  ]
}
```

A `5xx` response or a network error is "down"; anything else (including
`4xx`) is "up" -- the server answered, it just doesn't have that route.

## Tests

```bash
go test ./... -v
```

10 tests, all exercising real behavior rather than mocking everything away:

- **State-transition logic** (`monitor_test.go`): alerts fire exactly once
  per up<->down transition, never on the first check, never while state is
  unchanged, and snapshots are safe to mutate without corrupting internal
  state.
- **HTTP checker** (`checker_test.go`): run against real `httptest.Server`
  instances -- 2xx is up, 5xx is down, 404 is still up, an unreachable
  server is down.
- **Webhook notifier** (`notifier_test.go`): posts real JSON over real HTTP
  to a test server and asserts the payload; a delivery failure logs
  instead of panicking.

## What's intentionally not here

No Prometheus/Grafana integration, no persistent history storage (state is
in-memory only), no multi-node coordination. If you need those, this is a
starting point, not a replacement for a real observability stack.
