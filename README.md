# KMA

Go/Gin REST API for KMA's internal operations — order entry, deliveries,
invoicing, client catalogues with year-by-year pricing, and production/
operation cost tracking ("Kas Bon"). SQLite via GORM, no ORM magic beyond
that: it's a straightforward CRUD backend for a small internal team, not
a public product.

It does **not** handle authentication itself — see [Auth](#auth) below.

## What's in here

```
cmd/server/main.go        entrypoint: routes, middleware wiring, :8000
internal/handler/         one file per entity, plain CRUD + a few
                           entity-specific endpoints (grouped views,
                           photo upload, by-client/by-order filters)
internal/middleware/      RequireAuth — validates the session cookie
                           against KMA-Auth on every request
internal/database/        AutoMigrate (schema) / DropAllTables (dev only)
dto/                      GORM models — one struct per table, FKs and
                           cascade behavior declared via gorm tags
api/kma.yaml              OpenAPI 3.0 spec, served at /docs/kma.yaml
                           and rendered at /swagger
backup/                   backup.sh + crontab for the sqlite-backup
                           sidecar container (see Backups below)
Dockerfile                two-stage build (CGO needed for mattn/go-sqlite3)
docker-compose.yaml       sqlite-db + backend + backup, on kma_network
```

## Data model

Thirteen entities, each with its own handler file in `internal/handler/`:

- **Orders → Items → Invoice** — an order carries line items and gets
  invoiced (dp/pelunasan split, tracked via `Invoice.Type`).
- **Delivery → DeliveryItem** — what actually shipped, separate from
  what was ordered.
- **Client → ClientContact → ClientItem → ClientItemPrice** — each
  client has its own independent item catalogue (no shared master
  price list) with full year-by-year price history per item.
- **FinanceHeader → ProductionItem / OperationItem** — one shared
  parent ("Kas Bon") for two kinds of cost lines, filtered by
  `?type=production` or `?type=operation`.
- **Supplier** — standalone.

Most `id` columns are client-chosen sequential strings (e.g.
`"003/KMA/26"`), not auto-increment — so every create/rename path has
to precheck for collisions and every update anchors its `WHERE` to the
*old* id (see comments in the handlers; this was the source of a few
now-fixed silent-no-op bugs where a rename would update zero rows).

## API

Base path `/api/v1`, one route group per entity (see `cmd/server/main.go`
for the full table) — standard `GET` (list + by-id), `GET .../grouped`
or `.../by-<parent>` for pre-joined views, `POST`, `PATCH`, `DELETE`.
`PATCH` handlers only touch fields actually present in the request body,
so a partial update never zeroes out the rest of the row.

- `GET /api/v1/healthz` — unauthenticated, just proves the process is up.
  Deliberately outside the auth boundary: mixing "is the backend alive"
  with "is this caller logged in" is what caused the frontend's health
  badge to read "offline" for what was actually just a logged-out
  session.
- `/docs/kma.yaml` + `/swagger/*` — OpenAPI spec and Swagger UI.
- `/uploads/client-items/*` — catalogue photos, gated by the same
  `RequireAuth` as the API itself (not a public static folder).

## Auth

This service holds no passwords or sessions. Every route under
`/api/v1` (except `/healthz`) goes through `RequireAuth`
(`internal/middleware/auth.go`), which forwards the caller's session
cookie to a separate **KMA-Auth** service's `/internal/validate`
endpoint over a shared internal network, gated by `AUTH_INTERNAL_KEY`.
Fails closed: if the key isn't configured, or the auth service is
unreachable, or the session doesn't validate, the request is rejected
here — not left to nginx or the frontend to enforce.

## Running locally

```bash
go run ./cmd/server        # needs ./db_data/kma.sqlite (auto-created)
                            # and AUTH_INTERNAL_KEY set to match KMA-Auth
```

Or via Docker Compose (see below) for the full stack including the
sqlite-db and backup sidecars.

## Deployment topology

Three independent `docker-compose.yaml` stacks (this one, KMA-Auth,
and the frontend) join one external network, `kma_network`, which
**this** stack creates — the other two join it with `external: true`.
nginx fronts everything; the backend's `:8000` is not published to the
host, so the browser only ever reaches this API through nginx, never
directly.

## Backups

The `backup` service (alpine + sqlite3, `backup/backup.sh` +
`backup/crontab`) takes a live, WAL-safe `.backup` snapshot of
`kma.sqlite` weekly (Sunday 2 AM), gzips it into `./backups/`, and
prunes anything older than `BACKUP_RETENTION_DAYS` (default 30, so
~4 backups kept). `init: true` on that service is load-bearing, not
cosmetic — busybox `crond` needs a real PID 1 to own process groups,
and without it the container crash-loops instead of running on
schedule (see the comment in `docker-compose.yaml` if this ever
regresses).
