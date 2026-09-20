# AGENTS.md

Homelab dashboard (v0): a single binary that renders service tiles and pushes
live updates over SSE. Module `github.com/wiiaam/homelab-dashboard`.

## Commands

Tooling is managed by mise (see `.mise.toml`). Run everything through mise tasks:

- `mise run dev` — air hot reload; rebuilds templ + tailwind + go on change
  (`templ generate && tailwindcss -i web/input.css -o web/static/main.css
  --minify && go build -o ./tmp/bin/dashboard ./cmd/dashboard`) and starts the
  server on `:8080`. Controller/metrics logs stream to the console.
- `mise run vet` — `templ generate`, `go mod tidy`, `go vet ./...`, `go test ./...`
- `mise run test` — `templ generate`, `go test ./...`
- `mise run build` — templ + css + go build to `tmp/bin/dashboard`
- `mise run generate` — `templ generate` only

Always run `mise run vet` (or at least `test`) after changing Go or templ code,
and regenerate `_templ.go` via `mise run generate` after editing any `.templ`.

## Layout

- `cmd/dashboard/` — entrypoint: wires MemoryStore + FixtureSource controller +
  SimulatedMetrics + HTTP server, graceful shutdown.
- `internal/model/` — `Service`, `Metrics`, `Event`, Redis-style keys
  (`service:<ns>:<name>`, `metrics:<ns>:<name>`), `SplitKey`, `SortServices`.
- `internal/store/` — `Store` interface + `MemoryStore` (mutex maps, non-blocking
  subscribe fan-out of events). v1 swaps in RedisStore with no render/SSE changes.
- `internal/controller/` — reconciles a `Source` into the store. `FixtureSource`
  seeds 8 services and flaps one fixed service (10s up / 5s gone / repeat).
  `SimulatedMetrics` writes random CPU/mem for active services every 2s.
- `internal/server/` — `GET /` full render, `GET /events` SSE stream,
  `/static/`. `fragmentFor` maps store events to OOB-swap fragments.
- `web/components/` — templ UI (package `components`): `page.templ`, `tile.templ`,
  `fragments.templ`. Tailwind v4 `@source "./components"` in `web/input.css`.
- `web/static/` — `app.js` (htmx View-Transitions handler) committed;
  `main.css` is generated and gitignored.

## Conventions

- Keys: service events carry the service key (`service:` prefix); metrics
  events carry the `metrics:` key. `SplitKey` handles both.
- The metrics poller only writes metrics for services present in the store, and
  the SSE layer drops metrics events for services no longer active, so tiles
  never update after removal.
- Subscribers are non-blocking; missed route events self-heal on page reload.
- No comments in code unless they explain non-obvious domain logic.

## Gotchas

- This project lives on the native WSL filesystem; air uses `poll` in
  `.air.toml` as an fsnotify fallback (needed when on `/mnt/c`).
- Don't squat on `:8080` when testing a throwaway server — use a different port.