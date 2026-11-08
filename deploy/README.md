# Portable deployment (Docker Compose / Coolify)

This stack is independent of Coolify: PostgreSQL 17, an idempotent one-shot goose migration job, the Go API, and the compiled React site served by Nginx. It does **not** replace the repository's local-development `compose.yaml` or isolated `compose.preview.yaml`. Neither database is reused. The preview fixture seeder deliberately requires a loopback database and must not be run against this deployment.

## Local smoke test

1. Copy `deploy/.env.example` to `deploy/.env` and set matching, unique `POSTGRES_PASSWORD` and `DATABASE_URL` credentials.
2. Run from the repository root:

   ```sh
   docker compose --env-file deploy/.env -f deploy/compose.yml up -d --build
   docker compose --env-file deploy/.env -f deploy/compose.yml ps -a
   curl -fsS http://localhost:8080/api/v1/health
   ```

3. Browse http://localhost:8080. The production build uses relative `/api/v1` requests, routed to Go by Nginx. The API/DB are not published to the host.
4. View logs with `docker compose --env-file deploy/.env -f deploy/compose.yml logs -f api web migrate`.
5. Stop using `docker compose --env-file deploy/.env -f deploy/compose.yml down`. **Do not use `down -v`** unless you intend to delete the database.

The app may have empty marketplace content on first boot; this stack does not seed fictional preview data, configure Twilio, or turn on Stripe. Check that migrations have exited successfully before treating the site as ready.

## Coolify / VPS

- Deploy `deploy/compose.yml` as a Docker Compose application, using repository-root-relative build contexts. Set all required variables from `deploy/.env.example` as Coolify secrets/environment variables. Do not upload or commit `deploy/.env`.
- Point the public domain at the **web** service on container port **80**. Set `FRONTEND_ORIGIN=https://restringing.ie` exactly (no trailing slash), so session cookies use HTTPS. Terminate TLS at the reverse proxy; preserve the Host and forwarded scheme.
- The `web` service binds host port 8080 on loopback by default. Where Coolify handles domain routing via Docker service discovery, remove the `ports` entry or adapt it to the provider's Compose integration; use the service's internal port 80, **not** port 8080.
- Do not expose PostgreSQL or Go directly to the internet. Keep separate volumes and databases for staging and production.
- Leave `AUTH_SMS_PROVIDER` and `BILLING_PROVIDER` unset for a safe demo; enable only once provider credentials, webhook URLs, rate limits and flows are verified. The backend refuses insecure non-loopback `FRONTEND_ORIGIN`.
- For a public preview, use a separate Compose project name, DB volume and domain if another environment already runs on the same host.

## Database and portability

The `postgres_data` named volume is persistent but **not a backup**. Before upgrades or migrations, take a consistent `pg_dump` backup and regularly send encrypted copies off-server (S3/R2/B2). Test restores. To move providers, restore the database onto a new VPS, deploy the same Compose file and change DNS.

The migration job uses the checked-in `backend/migrations` and the goose CLI. For future releases, take a backup before rolling out changed migrations. Avoid simultaneous deployments against the same database.

Images are **not** stored in these containers. S3-compatible uploads can be added separately; no S3 settings or image-upload endpoints are implemented here.
