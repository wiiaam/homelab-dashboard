# Homelab Dashboard — Scope & Plan

A learning/creativity project to replace Homepage with a purpose-built, live-updating
service dashboard for the `honeypuffs-k8s-cluster`. Not solving an urgent pain point —
optimizing for depth of learning (controllers, leader election, pub/sub, SSE, HTMX)
over minimal effort.

## Goals

- Auto-discover services via annotated `Ingress` / `HTTPRoute` objects (same UX as
  Homepage's annotation-driven discovery).
- No client-side rendering framework — server renders HTML, browser just displays it.
- Live updates pushed to the browser with no polling: metrics refresh in place,
  services appear/disappear from the grid as they're added/removed in the cluster.
- Horizontally scalable read path, singleton write path.
- Deliberately over-engineered relative to actual need, as a vehicle for practicing
  patterns (leader election, event streaming, SSE) that generalize beyond this project.

## Non-goals

- Replacing Homepage's broader feature set (bookmarks, weather widgets, etc.) —
  scope is service tiles + live metrics only.
- Supporting non-Kubernetes sources (Docker labels, static config) — Ingress/HTTPRoute
  only, at least for v1.
- Multi-cluster support.

## Architecture

Three logical components, one Redis instance:

```
┌─────────────┐   watch Ingress/HTTPRoute   ┌──────────────┐
│  Controller │ ──────────────────────────► │ k8s API      │
│  (singleton,│                              └──────────────┘
│  leader-    │   write route data
│  elected)   │ ──────────────────────────►┌─────────────┐
└─────────────┘                             │             │
┌─────────────┐   write metrics             │    Redis    │
│   Metrics   │ ──────────────────────────► │             │
│   Poller    │                             │  hashes +   │
└─────────────┘                             │  streams    │
                                             └─────────────┘
                                                    │
                                      subscribe (Streams / pub-sub)
                                                    ▼
                                       ┌─────────────────────┐
                                       │   HTTP Server(s)     │
                                       │   (stateless,         │
                                       │   horizontally        │
                                       │   scaled)             │
                                       │  - full page render   │
                                       │  - SSE endpoint       │
                                       └─────────────────────┘
                                                    │
                                              HTMX + SSE
                                                    ▼
                                               Browser(s)
```

### 1. Controller (route watcher)

- Go binary using `client-go` informers on `Ingress` and `HTTPRoute`.
- Filters by Ingress class (`spec.ingressClassName` / the legacy
  `kubernetes.io/ingress.class` annotation) and, for `HTTPRoute`, by which `Gateway`
  it's attached to (`spec.parentRefs`) — so only routes belonging to the
  gateway/class the dashboard cares about are discovered, rather than every
  Ingress/HTTPRoute in the cluster regardless of which controller/gateway serves it.
  Configurable via the controller's own startup flags/config (e.g. a list of allowed
  ingress classes and/or gateway names), not hardcoded.
- Parses annotations: name, icon, url, `order`, `group`.
- Runs with `client-go`'s `leaderelection` package — one active leader, standbys
  on hot idle. Same pattern as `kube-controller-manager` / most operators.
- On Add/Update/Delete, writes to Redis:
  - `HSET service:<ns>:<name> ...fields...`
  - `SADD services:index service:<ns>:<name>` (removed on delete)
  - Publishes a change event (route-added / route-updated / route-removed) to a
    Redis Stream (`XADD updates:routes`) — Streams over plain pub/sub so a
    reconnecting HTTP server replica can replay missed events via consumer groups,
    rather than losing anything published while it was down.

### 2. Metrics poller

- Separate process/loop from the controller — different cadence (seconds, not
  "on annotation change").
- Source: `metrics-server` initially; consider switching to reading from the
  Prometheus/Grafana stack once that split is finished, since it's more realistic
  to how a real platform team would wire this and avoids re-scraping kubelet/cAdvisor
  directly.
- Writes `HSET metrics:<ns>:<name> cpu_millicores mem_bytes updated_at`, each key with
  `EXPIRE` (~30s) so a stalled poller results in "unknown" rather than silently stale
  numbers.
- Publishes metrics updates to a separate Stream (`updates:metrics`) — kept distinct
  from route updates so consumers can treat them at different priority/frequency.

### 3. HTTP server (frontend)

- Stateless, horizontally scaled behind a `Deployment` + HPA (if we want to actually
  exercise scaling, even though load will never justify it for real).
- On each request: reads current state from Redis (`SMEMBERS` + pipelined `HGETALL`s),
  renders the full grid server-side.
- Local in-memory cache refreshed periodically as a fallback/perf layer, in addition
  to the live stream — periodic reconcile as a safety net under the push path.
- `/events` SSE endpoint:
  - Subscribes to both Redis Streams (`updates:routes`, `updates:metrics`) via
    consumer groups, using `Last-Event-ID` from the browser to resume from the
    right offset on reconnect.
  - Forwards each event to the browser as a named SSE event containing a
    pre-rendered HTML fragment with the correct `hx-swap-oob` target:
    - Metrics: `hx-swap-oob="true"` on the existing `#metrics-<ns>-<name>` node.
    - Route added: `hx-swap-oob="afterend:#service-<neighbor>"` (position computed
      server-side from the `order` field) or `afterbegin:#service-list` if first.
    - Route removed: `hx-swap-oob="delete"` on `#service-<ns>-<name>`.

### 4. Frontend rendering

- Plain HTML + HTMX (SSE extension), CSS Grid with auto-placement (no explicit
  `grid-column`/`grid-row` per tile) — reflow on insert/remove is free via normal
  grid/flow behavior as long as we rely on DOM order rather than explicit placement.
- View Transitions API (`document.startViewTransition`, wired via an
  `htmx:beforeSwap` listener) for animated reflow when tiles are inserted/removed,
  rather than hand-rolled FLIP animation.
- Sort order: `order` annotation ascending, default fallback value for unannotated
  services, alphabetical tie-break for determinism. Grouping (`group` annotation) is
  a stretch goal — only add if the flat list actually becomes unreadable.

## Data model (Redis)

```
service:<ns>:<name>      Hash   { name, icon, url, order, group }
services:index           Set    { "service:<ns>:<name>", ... }
metrics:<ns>:<name>      Hash   { cpu_millicores, mem_bytes, updated_at }  (TTL ~30s)
updates:routes           Stream { type: added|updated|removed, key, fragment_html }
updates:metrics          Stream { key, fragment_html }
```

`maxmemory` set explicitly (e.g. 32-64mb) with `noeviction`, given total data volume
here is trivially small — the cap is about failing loudly rather than needing the
ceiling. No persistence (no RDB/AOF) — watcher/poller repopulate from source of
truth (k8s API / metrics-server) on restart, so Redis stays a pure cache.

## Storage abstraction

To avoid v0's in-memory approach forcing a rewrite when Redis is introduced, storage
is defined as a `Store` interface from the start, shared by both the controller and
HTTP server code:

```go
type Store interface {
    GetService(key string) (Service, bool)
    ListServices() []Service
    SetService(s Service)
    DeleteService(key string)
    SetMetrics(key string, m Metrics)
    GetMetrics(key string) (Metrics, bool)
    Subscribe(ctx context.Context) <-chan Event
}
```

Two implementations share this interface:
- `MemoryStore` — mutex-protected map + in-process channel fan-out for `Subscribe`.
  Used in v0.
- `RedisStore` — backed by the hashes/streams described below. Used from v1 onward.

Controller/reconcile logic, HTTP rendering, and SSE/OOB-swap fragment generation are
all written once against the interface and never change between v0 and v1 — only the
`Store` implementation passed in at startup changes. Event/fragment types live in a
shared package imported by both implementations, not duplicated per-backend.

This also makes topology a deployment-time choice rather than a code choice: the same
binary can run combined (controller + server in one process, `MemoryStore`, single
replica, no leader election) or split (`dashboard controller` / `dashboard server`
subcommands, `RedisStore`, controller leader-elected, server horizontally scaled) —
selected via a `REDIS_ADDR`-style config flag or a Helm `mode: singleton | distributed`
value. Deciding later that Redis isn't worth the operational overhead for a homelab is
then a config/values change, not a code rewrite.

## Build phases

1. **v0 — single binary, `MemoryStore`.** Controller and HTTP server compiled
   together, sharing one `MemoryStore` instance directly (no network hop between
   them). Validates discovery/annotation parsing, grid layout, and the SSE/OOB swap
   mechanics end-to-end before any distributed-systems complexity is introduced.
2. **v1 — split controller/poller/server + `RedisStore`.** Split into separate
   binaries/deployments and swap in `RedisStore`. Reconcile and render logic
   unchanged from v0 — only the `Store` implementation and process boundary change.
3. **v2 — live metrics via SSE.** Add the `/events` endpoint and OOB swaps for
   metrics only. Validates the SSE + HTMX plumbing on the simpler, higher-frequency
   data path first.
4. **v3 — live route add/remove via SSE.** Add insertion/removal events with
   position-aware OOB targeting, plus View Transitions for animated reflow.
5. **v4 — resilience pass.** Leader election on the controller, Redis Stream
   consumer groups + `Last-Event-ID` resume, periodic full-reconcile fallback on
   the frontend, `EXPIRE`-based metrics staleness handling.
6. **Stretch goals.** Grouping/sections, HPA on the frontend tied to request rate,
   OpenTelemetry tracing across the three services, feeding metrics from the
   Prometheus/Grafana stack instead of metrics-server directly.

## Open questions / decisions deferred

- Grouping layout (sections vs flat list) — deferred until the flat list is
  actually unreadable.
- Whether newly-inserted mid-session tiles need exact sort-position insertion or
  whether "append + correct on next full reload" is an acceptable simplification.
- Redis Streams vs NATS JetStream for the event bus, if delivery guarantees become
  a bigger focus than initially planned.

## Development setup (v0)

Toolchain is managed by [mise](https://mise.jdx.dev): Go, Node, templ (via the
`go:` backend), air and tailwindcss (via aqua). Versions are pinned in
`.mise.toml`.

```sh
mise trust                 # first time: trust the project config
mise install               # fetch the pinned toolchain
mise run dev               # air: rebuilds templ + tailwind + Go and hot-reloads
```

`air` rebuilds everything on change — `.templ` files are re-generated, `web/input.css`
is compiled to the (git-ignored) `web/static/main.css`, and the binary restarts. For a
one-off stylesheet build run `mise run css`; `mise run build` produces the whole
artifact. Other tasks: `mise run test`, `mise run vet`.

v0 is a fixture-seeded demo — no cluster, no Redis. Point a browser at
`http://localhost:8080` and you should see:

- Eight seeded service tiles in order-annotated grid layout.
- CPU/mem metrics ticking in place every 2s via the `/events` SSE endpoint
  (`hx-swap-oob` updates on each `#metrics-*` span).
- A flap simulation: after a 10s quiet period one random service disappears for
  5 seconds and then returns, repeating every 15s — animated via the View
  Transitions API.