---
title: Deployment
nav_order: 6
has_children: false
---

# Deployment

For initial setup see [Getting Started](getting-started.html). This page covers production-specific concerns.

## How the Docker Setup Works

Meerkat CRM runs as a single all-in-one container. Inside it, nginx serves the React SPA and proxies all `/api/`, `/carddav/`, and `/.well-known/carddav` requests to the Go backend on `127.0.0.1:8080`; the backend is never exposed to the host directly. Only the nginx port is published (default `7300`). This same-origin proxy is built in, so nothing extra is required.

You only need an external reverse proxy for TLS termination. Point it at the published port (default `7300`):

```nginx
server {
    listen 443 ssl;
    server_name meerkat.example.com;

    location / {
        proxy_pass http://localhost:7300;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
    }
}
```

### Security headers

Meerkat sets its security headers itself; a reverse proxy in front should pass them through and **not add its own** (a second `Content-Security-Policy` is combined with the first and can only make the policy stricter, a second `Strict-Transport-Security` or `X-Frame-Options` is sent twice). In nginx do not `add_header` them, in Caddy do not use `header` for them, in Traefik leave the security-headers middleware out.

| Response | Headers |
|---|---|
| The web app (HTML, JS, CSS, images) | `Content-Security-Policy` (below), `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, `Permissions-Policy` (no camera, microphone, geolocation, payment, USB), `Cross-Origin-Opener-Policy: same-origin`, `Cross-Origin-Resource-Policy: same-origin` |
| The API (`/api/`, `/carddav/`, calendar feeds, downloads), sent by the backend | the same, but `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'` and `Cross-Origin-Resource-Policy: same-site` |

The web app policy is enforced (not report-only):

```
default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:;
font-src 'self' data:; connect-src 'self'; manifest-src 'self'; worker-src 'self'; object-src 'none';
base-uri 'self'; form-action 'self'; frame-ancestors 'none'
```

Only the bundle may run (no inline scripts, no `eval`). `style-src 'unsafe-inline'` is needed because the UI library injects style tags at runtime; nothing else is loaded from other origins. It lives in one file, `frontend/security-headers.conf`, shared by the all-in-one image and the separate frontend image, and the E2E suite runs against it and fails on any violation.

**HSTS** is sent only when the browser is on HTTPS: nginx looks at `X-Forwarded-Proto` (set it to `https` in your TLS proxy, as in the example above, and keep it when you chain proxies; only `http` and `https` are accepted) and sends `Strict-Transport-Security: max-age=31536000`. The backend sends the same when the request came over TLS or with `X-Forwarded-Proto: https`, and adds `includeSubDomains` only when you declared HTTPS yourself with `COOKIE_SECURE=true`. A header a client can set never commits your other subdomains to HTTPS. Plain-HTTP access never gets it, so a test setup on a LAN address is not locked to HTTPS. Browsers remember HSTS for a year: only enable HTTPS on a hostname you will keep serving over HTTPS.

**Gap:** responses that nginx generates itself carry no security headers: its own error pages (for example 502 or 504 while the backend is down) and the `/.well-known/carddav` redirect. They contain no application data and no scripts, so this is accepted.

**Relaxing the policy** for a custom integration (an API on another origin, a self-hosted font, an embedded widget): edit `frontend/security-headers.conf` and rebuild, or mount your own copy over `/etc/nginx/security-headers.conf` in the container. Add only what you need, for example `connect-src 'self' https://api.example.com`. Do not add `'unsafe-inline'` or `'unsafe-eval'` to `script-src`. An API on another origin additionally needs `FRONTEND_URL` (CORS) and photos from it need `img-src` for that origin, and the API must be on the same site as the web app, because the backend sends `Cross-Origin-Resource-Policy: same-site`. If you embed Meerkat in a page of your own, you must also change `frame-ancestors` and `X-Frame-Options` (not recommended).

`make i18n-check` (which also runs `scripts/security-headers.test.mjs`) fails when one of the two nginx configs stops including the shared file, adds headers of its own, or adds them to a proxied location.

## Production Environment

Set these variables in `.env` when running over HTTPS:

| Variable | Value |
|---|---|
| `FRONTEND_URL` | Exact origin, e.g. `https://meerkat.example.com` (never `*`) |
| `COOKIE_SECURE` | `true` |
| `COOKIE_DOMAIN` | Leave empty. Only set it (e.g. `.example.com`) if frontend and API live on different subdomains |
| `JWT_SECRET_KEY` | Generate with `openssl rand -base64 32` |
| `BLOCK_PRIVATE_URLS` | `true` on any instance with more than one user (see [Private network access](#private-network-access-ssrf)) |

## Private network access (SSRF)

Several features make the server fetch an address a user typed in: webhooks, CalDAV/ICS calendar sync, CardDAV contact sync (including the `/.well-known/carddav` discovery) and the Monica import (including avatars). On a shared instance a user could point them at the server's own network (a database, the cloud metadata service at `169.254.169.254`, other containers).

`BLOCK_PRIVATE_URLS=true` refuses loopback, private (RFC 1918, IPv6 unique local), link-local, carrier-grade NAT (`100.64.0.0/10`), multicast, `0.0.0.0/8`, the reserved `240.0.0.0/4`, benchmarking (`198.18.0.0/15`) and IPv6 forms that embed an IPv4 address (IPv4-mapped `::ffff:127.0.0.1`, NAT64 `64:ff9b::/96`, 6to4 `2002::/16`, Teredo) addresses for all of these features at once. The check runs when the connection is made, on the address that is dialled (a name is resolved once and only the checked addresses are dialled), so redirects and DNS rebinding are covered too.

- **Default: `false`.** Self-hosters often sync from a CalDAV, CardDAV or Monica server on their LAN, which needs private addresses. Turn it on when you do not trust every user.
- When `BLOCK_PRIVATE_URLS` is set it decides for all features and overrides the older `WEBHOOK_BLOCK_PRIVATE_URLS`, `CALDAV_BLOCK_PRIVATE_URLS`, `CARDDAV_BLOCK_PRIVATE_URLS` and `MONICA_BLOCK_PRIVATE_URLS`. When it is unset, those still work as before (`CARDDAV_BLOCK_PRIVATE_URLS` defaults to `CALDAV_BLOCK_PRIVATE_URLS`).
- Accepted values: `true`/`false`, `1`/`0`, `yes`/`no`, `on`/`off` (any case). Anything else (including an empty value) is treated as `true` and logged as an error: for a security switch a typo must not leave the server open.
- Features guarded by this switch ignore the `HTTP_PROXY`/`HTTPS_PROXY` environment variables and always connect directly. A proxy would be dialled instead of the target, and the check would look at the wrong address. If your network needs an outbound proxy, these features cannot use it.
- If blocking is off for any feature and more than one user exists, the server logs a warning at startup and admins see a dismissible banner on the Users page.
- Photo URLs in vCards and photo uploads by URL are always checked, whatever the setting.
- Not covered, on purpose: the OIDC provider (`OIDC_PROVIDER_URL`) and the SMTP host. Only the administrator sets them, and they are often internal services.


## Sessions and audit log

**Sessions.** Every login creates a session. Users see their sessions under Settings, Profile (browser, IP address, when it was created and last active), can sign out one or all of them, and changing the password signs out every other browser. A reset of the password by e-mail or by an admin ends all sessions. A signed token stops working at once when its session is revoked, even though the token itself would be valid for `JWT_EXPIRY_HOURS`. "Sign out everywhere" also raises the user's token version, which invalidates tokens issued before sessions existed. Tokens issued by an older version keep working until they expire, unless the user signs out everywhere. API tokens and the calendar feed are managed separately and are not affected.

**Audit log.** Admins see it under Settings, Audit log (filter by event and user). It records logins and failed logins (with the account, never what was typed for an unknown account), logouts, password changes and resets, sessions signed out, data exports, API token, calendar feed and backup actions, and admin changes to users. Each entry has the time, user, IP address (behind a proxy this is the address of the proxy unless `TRUSTED_PROXIES` is set) and user agent. Entries of deleted accounts stay until the retention period ends. Entries older than `AUDIT_LOG_RETENTION_DAYS` (default 365, `0` = keep) are deleted daily at 03:30, together with ended sessions.

## Monitoring with Prometheus

Meerkat can expose metrics in the Prometheus text format. It is **off by default**. Set `METRICS_ENABLED=true` and choose how to protect it:

| Variable | Meaning |
|---|---|
| `METRICS_ENABLED` | `true` to serve `/metrics`. Default `false` (no route exists) |
| `METRICS_TOKEN` | If set, scrapers must send `Authorization: Bearer <token>` (compared in constant time) |
| `METRICS_ADDR` | If set (for example `127.0.0.1:9090` or `:9090` on a private Docker network), `/metrics` is served only on this extra listener and not on the main port |

The bundled nginx proxies only `/api/`, `/health` and `/carddav/`, so `/metrics` on the main port is not reachable through it. If your own reverse proxy forwards everything, set a token or use `METRICS_ADDR`. Without either, Meerkat logs a warning at startup. The metrics contain no user data (routes are patterns such as `/api/v1/contacts/:id`, never URLs), but they reveal traffic and failure patterns.

| Metric | Type | Meaning |
|---|---|---|
| `meerkat_http_requests_total{method,route,status}` | counter | Requests by route pattern (`unmatched` for 404s without a route) |
| `meerkat_http_request_duration_seconds{method,route}` | histogram | Latency |
| `meerkat_job_last_success_timestamp_seconds{job}` | gauge | Last successful run of the jobs kept in the `job_executions` table (`daily_reminders`, `calendar_sync`, `carddav_contact_sync`) |
| `meerkat_job_running{job}` | gauge | 1 while the job holds its lock |
| `meerkat_job_runs_total{job,result}` | counter | Runs of `reminders`, `trash-purge` and `backup` since the process started, `success` or `failure` |
| `meerkat_webhook_deliveries_total{result}` | counter | Webhook delivery attempts (every retry counts) |
| `meerkat_emails_total{result}` | counter | Emails where at least one channel succeeded (`success`) or all failed (`failure`) |
| `go_goroutines`, `go_memstats_alloc_bytes`, `process_start_time_seconds` | gauge | Process basics |

Example scrape config:

```yaml
scrape_configs:
  - job_name: meerkat
    metrics_path: /metrics
    authorization:
      type: Bearer
      credentials_file: /etc/prometheus/meerkat-token
    static_configs:
      - targets: ["meerkat-backend:8080"]   # or the METRICS_ADDR host and port
```

Alert ideas (adapt the thresholds):

- Backend down: `up{job="meerkat"} == 0` for 5 minutes.
- Reminders did not run for over 26 hours: `time() - meerkat_job_last_success_timestamp_seconds{job="daily_reminders"} > 26 * 3600`. The same for `calendar_sync` and `carddav_contact_sync`, with your sync interval.
- A scheduled job failed: `increase(meerkat_job_runs_total{result="failure"}[1h]) > 0`.
- Email failing: `increase(meerkat_emails_total{result="failure"}[1h]) > 0`.
- Many failing webhooks: `increase(meerkat_webhook_deliveries_total{result="failure"}[1h]) > 10`.
- Server errors: `sum(rate(meerkat_http_requests_total{status=~"5.."}[5m])) > 0.05`.
- Slow API: `histogram_quantile(0.95, sum by (le) (rate(meerkat_http_request_duration_seconds_bucket[5m]))) > 1`.

Counters start at zero on every restart and the metrics are per instance. Job run counters of a job that never ran do not exist yet; use `absent()` with care.

## Two-factor authentication

Each user can turn on a second login step (TOTP, works with any authenticator app) under Settings, Profile: scan the QR code (or type the key), enter the code the app shows, and save the ten recovery codes, which are shown only once. From then on a password login also asks for a code from the app, or for one recovery code (each works once). Turning it on needs the password and a code from the app; disabling it and creating new recovery codes need the password and a code. Turning it on or off signs out the account's other sessions. Accounts without a password (single sign-on only) cannot use it.

- **Lost phone and recovery codes:** an admin can reset the user's two-factor on the Users page. The user is signed out everywhere and can log in with the password alone, then enrol again. Both the enrolment and the reset are in the [audit log](#sessions-and-audit-log).
- **Brute force:** wrong codes, and wrong passwords in the two-factor dialogs, count toward the same lockout as wrong passwords (per account, with growing delays); a code is accepted once, so a code that was just used (or sniffed) cannot be replayed.
- **Secrets at rest:** the secret is encrypted with the key derived from `JWT_SECRET_KEY`, like the CardDAV and calendar credentials. If you change `JWT_SECRET_KEY`, secrets cannot be decrypted and users with two-factor are locked out until an admin resets them (the recovery codes are stored as HMACs keyed from the same secret, so they stop working too).
- **Not affected:** single sign-on (the identity provider is the second factor), API tokens and the calendar feed.

## Database storage

Meerkat stores everything in one SQLite file (`meerkat.db`, `SQLITE_DB_PATH`). It opens it in WAL mode with `busy_timeout` of 5 seconds, `synchronous=NORMAL` and foreign keys enforced, so you will see `meerkat.db-wal` and `meerkat.db-shm` next to it (see [The WAL files](#the-wal-files)). WAL needs shared memory between processes on the same machine, so **keep the database on a local disk**. Network file systems (NFS, SMB/CIFS and the like) are not supported and can corrupt the database. Mount `DATA_PATH` from a local disk, and run only one Meerkat instance per database.

## Single Sign-On (OIDC)

Meerkat CRM supports SSO via any OpenID Connect provider (Keycloak, Google, Authentik, Authelia, etc.). When enabled, a **Sign in with provider** button appears on the login page.

### Setup

1. Register a new OAuth2 client with your provider. Set the redirect URI to:
   ```
   https://meerkat.example.com/api/v1/auth/oidc/callback
   ```
   This is derived automatically from `FRONTEND_URL`, no separate variable needed.

2.  Set the OIDC environment variables in the docker compose. See [Getting-Started → Environment variables](getting-started.html#environment-variables) for details. SSO is disabled if any of the first three variables are missing.

### Account linking

On first SSO login, the backend attempts to match the OIDC identity to an existing account in this order:

1. **Subject match** — if the user has logged in via this provider before, their account is found directly.
2. **Email match** — if the provider returns a *verified* email that matches an existing account, the OIDC identity is linked to that account automatically. Unverified emails are ignored to prevent account takeover (except if `OIDC_TRUST_EMAIL=true` is set).
3. **Auto-provision** — if `OIDC_AUTO_PROVISION=true` and no account matched, a new account is created using the email/name from the provider.

If auto-provisioning is disabled and no match is found, the user sees an error and must be registered manually first.

### Passwords

Accounts created through SSO have no password and can only log in via SSO. Existing password-based accounts that get linked retain their password.

## Upgrades

```sh
docker compose pull
docker compose up -d
```

Database migrations run automatically on startup.

## Backups

Meerkat has built-in backups. Every day at `BACKUP_TIME` (default `02:30`, in `REMINDER_TIMEZONE`) it writes one file, `meerkat-backup-YYYYMMDD-HHMMSS.tar.gz`, into `BACKUP_DIR` (default: the `backups` directory next to the database, e.g. `/app/data/backups` in the Docker image, so it is on your data volume) and keeps the `BACKUP_RETENTION_COUNT` (default 7) most recent ones. Set `BACKUP_ENABLED=false` to turn the schedule off. See [Getting started](getting-started.md#environment-variables) for all settings.

Each archive holds:

- `meerkat.db`: a consistent snapshot of the whole SQLite database, taken with `VACUUM INTO`. This is safe while the app is running and the database is in WAL mode.
- `photos/`: all profile photo files, including the small thumbnails (`*_thumb.jpg`). Thumbnails used to be stored inside the database; at startup after the upgrade Meerkat copies them into this directory once (the old copy stays in the database, so a downgrade keeps working for them, and a later version removes it). The photo directory must therefore be writable; if it is not, thumbnails stay hidden and the log says so. Thumbnails of photos added after the upgrade exist only as files.

**A backup contains the data of all users** (contacts, notes, reminders, account data and password hashes, CardDAV and calendar credentials in encrypted form). Treat it like the database itself: the directory is created (or, if it exists, set) to mode `0700` and the files with `0600`, and downloads are limited to administrators.

Administrators manage backups under **Settings > Backups**: list, **Create backup now**, download and delete. The same is available through the [admin API](api-reference.md#admin).

Backups live on the same disk as your data, so they do not protect against losing the disk. Copy them regularly to another machine (for example `rsync -a /path/to/data/backups/ backup-host:meerkat/`) or download them from the admin page. Creating and downloading a backup are exempt from `HTTP_WRITE_TIMEOUT`, so big backups do not time out. The browser download holds the whole archive in memory before saving it, so for very large backups copy the file from `BACKUP_DIR` (for example with `rsync`) instead.

### The WAL files

Meerkat runs SQLite in WAL mode, so next to `meerkat.db` you will see `meerkat.db-wal` and `meerkat.db-shm`. Recent changes can live only in the `-wal` file. Copying `meerkat.db` alone while the app runs can therefore give you a stale or inconsistent database. Use the built-in backup (or `sqlite3 meerkat.db "VACUUM INTO 'copy.db'"`) instead of copying the file, or stop the app first and copy all three files.

### Restoring a backup

**Keep `JWT_SECRET_KEY` and your `.env` file safe alongside the backups.** The `.env` file is not in the archive. CardDAV and CalDAV passwords in the database are encrypted with a key derived from `JWT_SECRET_KEY`, so a restore needs the same `JWT_SECRET_KEY`; with a different one those connections must be set up again. Without your `.env` you also lose the other settings (SMTP, OIDC and so on).

1. Stop Meerkat (`docker compose down`).
2. Unpack the archive in an empty directory: `tar -xzf meerkat-backup-YYYYMMDD-HHMMSS.tar.gz`.
3. On the host, in the directory mounted at `/app/data` in the container (`DATA_PATH`, default `./data`), move the current `meerkat.db`, `meerkat.db-wal` and `meerkat.db-shm` out of the way, then put the unpacked `meerkat.db` in their place. Do not keep the old `-wal`/`-shm` files: they belong to the old database.
4. Replace the contents of the photo directory with the unpacked `photos/` directory. Do this on the host too, in the directory mounted at `/app/static/photos` in the container (`PHOTOS_PATH`, default `./photos`).
5. Make sure the files are owned by the user the container runs as, then start Meerkat. Migrations run on startup, so a backup from an older version is upgraded.
