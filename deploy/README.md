# Portable Restringing.ie deployment (VPS / Coolify)

This stack publishes **two separate websites**:
- `preview.restringing.ie` → `web` (React/Vite, Nginx) → `api` (Go) → PostgreSQL.
- `restringing.ie` → `landing` (standalone static coming-soon site, no API or database).

The website under development stays on the preview subdomain; the main domain never exposes the unfinished app. This stack does not reuse the repository's local-development `compose.yaml` or isolated `compose.preview.yaml`. The local fictional fixture seeder is restricted to a loopback database and **cannot** be used against the public-preview VPS database.

## Local smoke test

Copy `deploy/.env.example` to `deploy/.env`, and replace the password in both `POSTGRES_PASSWORD` and `DATABASE_URL` with matching, secure values.

From the repository root:

```sh
docker compose --project-directory . --env-file deploy/.env -f deploy/compose.yml up -d --build
docker compose --project-directory . --env-file deploy/.env -f deploy/compose.yml ps -a
curl -fsS http://localhost:8080/api/v1/health
curl -fsS http://localhost:8082/
```

- Preview application: http://localhost:8080
- Coming-soon page: http://localhost:8082

The frontend calls relative `/api/v1`, proxied by Nginx to the API. Neither the API nor database exposes a host port. The preview's initial database contains **no fictional fixtures**; it can be empty until you add approved catalogue/stringer content.

Logs:

```sh
docker compose --project-directory . --env-file deploy/.env -f deploy/compose.yml logs -f api web landing migrate
```

Stop with `docker compose --project-directory . --env-file deploy/.env -f deploy/compose.yml down`. **Do not use `down -v`**, which deletes the database volume.

## Coolify and DNS

1. Deploy `deploy/compose.yml` from GitHub as a Compose application. Add secrets/environment variables from `deploy/.env.example` within Coolify; do not commit `deploy/.env`. Coolify uses the repository root as the Compose project directory, so build contexts are relative to the repository root. The local commands above explicitly do the same with `--project-directory .`.
2. Route **`https://preview.restringing.ie` to the `web` service, internal port 80**. Route **`https://restringing.ie` to the `landing` service, internal port 80**. Optionally redirect `www.restringing.ie` to the apex domain.
3. Set `FRONTEND_ORIGIN=https://preview.restringing.ie` exactly, without a trailing slash. The backend requires HTTPS for a non-loopback frontend and will set secure session cookies.
4. Configure DNS for the main and preview hostnames to point to the VPS/proxy. Enable automatic HTTPS on both domains.
5. The Compose file binds `web` and `landing` to localhost ports 8080 and 8082, respectively, for self-managed reverse proxies. For Coolify's container-based routing, remove or adapt those host `ports` entries and use each service's **internal port 80**.
6. Keep PostgreSQL and Go private; never expose port 5432 or 8081. Configure `AUTH_TRUSTED_PROXY_CIDRS` only to known trusted proxies when needed. Do not use an open wildcard range.

**Public-preview safeguards:** An obscured subdomain is not access control. If the preview is private, put it behind proxy basic authentication or another access gateway. Keep Twilio and Stripe disabled or strictly in sandbox mode. Do not accept real transactions or personal data until the relevant flows and policies are ready. The main-domain landing page does not need these integrations.

## Database, deploys and portability

A one-off Goose migration container runs before the API is started. Back up the database before shipping schema migrations; avoid simultaneous deployments against one database. The `postgres_data` named volume survives normal restarts but is **not a backup**. Schedule off-VPS encrypted backups (S3/R2/B2), retain multiple generations, and periodically test restores.

You can move the stack to another provider by restoring PostgreSQL, deploying this same Compose file, configuring equivalent HTTPS routing and changing DNS. S3-compatible user-image uploading is not implemented in this PR; add it as a separate feature.
