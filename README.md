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
backup/                   backup.sh + crontab for the backup container,
                           restore.sh to put a backup back, and the
                           Dockerfile of the small image (sqlite3, cron)
                           the backup and sqlite-db containers run on
update.sh                 back up, then rebuild and restart this stack
Dockerfile                two-stage build (CGO needed for mattn/go-sqlite3)
docker-compose.yaml       sqlite-db + backend + backup, on kma_network
.env.example              the settings, copy to .env
```

Not in the repository (git-ignored, customer data): `db_data/` (the
database), `uploads/` (client-item photos), `backups/`, and `.env`.

## Data model

Fifteen entities, each with its own handler file in `internal/handler/`:

- **Orders → Items → Invoice** — an order carries line items and gets
  invoiced (dp/pelunasan split, tracked via `Invoice.Type`).
- **Delivery → DeliveryItem** — what actually shipped, separate from
  what was ordered.
- **Client → ClientContact → ClientItem → ClientItemPrice** — each
  client has its own independent item catalogue (no shared master
  price list) with full year-by-year price history per item.
- **FinanceHeader → ProductionItem / OperationItem** — one shared
  parent ("Kas Bon") for two kinds of cost lines; a Kas Bon can hold
  both. Each line can point at the order it was for (`order_id`, kept
  in step when an order is renamed, cleared when it's deleted).
  Production quantities are decimals (2.5 meters).
- **Budget** (a monthly limit per scope and category) and
  **RecurringCost** (rent, salaries… posted once a month as one Kas Bon).
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
- `POST /api/v1/finance/batch` — any set of Kas Bon changes in one
  transaction: new Kas Bons, lines to create, update (whitelisted fields)
  or delete, header date/description edits. All or nothing; the response
  carries every changed or deleted row as it was, which is what the
  frontend's undo sends back. A Kas Bon left without lines is removed.
- `GET/PUT /api/v1/budget`, `/api/v1/recurring-cost` (CRUD) and
  `POST /api/v1/recurring-cost/post-month` (adds the month's Kas Bon;
  posting a month twice adds nothing).
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
cp .env.example .env       # set AUTH_INTERNAL_KEY to match KMA-Auth's
go run ./cmd/server        # needs ./db_data/kma.sqlite (auto-created)
                            # and AUTH_INTERNAL_KEY set to match KMA-Auth
go test ./...              # the auth check's tests (internal/middleware)
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

Start them in that order — this one, then KMA-Auth, then KMA-Frontend:

```bash
cd KMA          && docker compose up -d --build
cd ../KMA-Auth  && docker compose up -d --build
cd ../KMA-Frontend && docker compose up -d --build
```

After a code change, update a stack with `./update.sh` instead: it takes
a backup first, then rebuilds and restarts what changed.

**Staying up.** Every container restarts by itself after a crash, but
only while Docker Desktop runs: turn on Docker Desktop's **Settings →
General → Start Docker Desktop when you sign in**, or nothing comes back
after the Mac restarts. (In September the backends sat stopped for over
two weeks before anyone noticed.) A stack stopped with `docker compose
stop` stays stopped until it's started again.

- **Health checks.** The backend is checked every 30s
  (`/api/v1/healthz`); Docker Desktop and `docker compose ps` show it as
  healthy or unhealthy. The backup container shows unhealthy when the
  newest backup is more than 8 days old.
- **Clean stops.** On a stop, restart or update, the backend finishes
  the requests it's working on (up to 20s) before exiting, and closes the
  database properly. It used to be killed mid-request (exit code 2).
- **Logs rotate**: 5 files of 10 MB per container, so they never fill
  the disk. `docker logs kma_backend` shows the backend's.
- **The auth service restarting doesn't log anyone out.** The backend
  asks the auth service about every request; when that service is busy
  (rate limiting a burst of requests) or restarting, it retries once and
  then answers 503 "try again", not 401. A 401 sends the browser to the
  login page, so before this, a burst of page loads could sign everyone
  out. Only the auth service saying "this session is invalid" is a 401.

## Backups

The `backup` container backs up the database **and the client-item
photos** into `./backups/` every 7 days:

- `kma-<date>.sqlite.gz`: a live, WAL-safe `sqlite3 .backup` snapshot of
  `kma.sqlite`, which must pass SQLite's integrity check before it
  counts. One that fails is kept as `…_FAILED-CHECK.bad`, nothing is
  pruned, and the failure shows in `docker logs kma_backup`.
- `kma-uploads-<date>.tar.gz`: the `uploads/` folder.
- The newest 8 of each are kept: pruned by count, never by age, so a
  stack that was down for a month still has its last 8 backups when it
  comes back.

**When.** Cron checks every hour and takes a backup when the newest is 7
days old or more, and it also checks when the container starts. A fixed
weekly time (it used to be Sunday 2 AM UTC) is missed whenever the Mac
is asleep or off then; this way the week's backup is taken within the
hour of the Mac being on. Times are Jakarta time. Change the number kept
and the interval with `BACKUP_KEEP` and `BACKUP_EVERY_DAYS` in
`docker-compose.yaml`.

**A second copy.** The backups sit on the same disk as the database, so
a failed disk takes both. Set `BACKUP_COPY_DIR` in `.env` to an external
drive or a synced folder (iCloud Drive, Google Drive) and every backup
is copied there too, with the same number kept.

```bash
docker exec kma_backup sh /backup.sh                    # take a backup now
docker logs kma_backup                                  # when backups ran
./backup/restore.sh backups/kma-<date>.sqlite.gz        # put one back
tar -xzf backups/kma-uploads-<date>.tar.gz -C uploads   # and its photos
```

`restore.sh` checks the backup first, asks you to type `yes`, stops the
backend, moves the current database (with its `-wal`/`-shm` files) to
`db_data/before-restore-<date>/` so the restore can itself be undone,
and starts the backend again.

Things preserved in that container's setup, in case they regress:
- `init: true` is load-bearing, not cosmetic — busybox `crond` needs a
  real PID 1 to own process groups, and without it the container
  crash-loops instead of running on schedule (see the comment in
  `docker-compose.yaml`).
- Cron starts jobs with an empty environment, so the container saves its
  settings to `/run/backup.env` when it starts and `backup.sh` reads them
  back. Without that, the scheduled run in the auth stack looked for the
  wrong file.
- The database folder is mounted read-write, though the backup only
  reads: after the backend stops cleanly SQLite has removed its
  `-wal`/`-shm` files, and opening the database then has to create them.
  Read-only, no backup could be taken while the backend was down.
- `sqlite3`, `dcron` and `tzdata` are built into the image
  (`backup/Dockerfile`) instead of installed at every start, which needed
  the internet: booting without a connection left the container
  crash-looping with no backups.

## Photos

Client-item photos are saved to `uploads/client-items/` in this folder
(mounted into the backend at `/app/uploads`), and backed up with the
database. Until October 2026 they were saved inside the backend
container only, where any rebuild would have deleted them.

## On a phone

The frontend is served to the tailnet over HTTPS by `tailscale serve`
(at `https://<mac-name>.<tailnet>.ts.net`), and can be installed as an
app from there: Chrome's menu → **Install app** on Android, Safari's
Share → **Add to Home Screen** on an iPhone. See KMA-Frontend's README.
It's reachable while the Mac is awake and Tailscale and Docker Desktop
are running.

## Security notes

- **This repository is public.** Everything committed can be read by
  anyone, including its history. `.gitignore` keeps out the database,
  backups, photos, `.env` files, keys and certificates; check `git
  status` before every commit all the same.
- If `.env` or the database is ever committed by mistake, deleting it
  in a later commit isn't enough: it stays in the history. Rotate the
  key it held (KMA-Auth's `AuthRotate.md`) and treat its data as public.
- `.dockerignore` keeps the database, backups, photos and `.env` out of
  image builds (they used to be sent to Docker, and kept in its build
  cache, on every build).
- **Ids from the URL are values, never SQL.** Every lookup by id is
  `Where("id = ?", id)`. GORM's shorthand `db.First(&x, id)` treats a
  non-numeric string as raw SQL, so `/client/999 OR 1=1` used to return a
  client, and the same in a supplier delete could have deleted them all.
  Keep to the `?` form (`internal/handler/safety_test.go` checks it).
- **A catalogue item can't name its own photo file.** `photo_path` is set
  only by the photo upload, and deleting a photo only ever touches files in
  `uploads/client-items/`. Taken from a request, it used to let deleting an
  item delete any file the server could reach, the database included.
- Dependencies are checked with
  `GOTOOLCHAIN=go1.25.14 go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...`
  (clean as of October 2026). The project builds with Go 1.25, so pin
  library versions that still support it (`golang.org/x/text` v0.42+ needs
  Go 1.26).
