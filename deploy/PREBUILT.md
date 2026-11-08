# No-build ARM64 deployment in Coolify

The current VPS is hosting Coolify and its own Go/Node source builds. If a deploy freezes the dashboard, **do not retry source builds**. Use this prebuilt deployment instead. It uses the same services and database volume names as the source Compose deployment, but pulls images built on GitHub's native ARM64 runners.

## Before switching

1. Disable Coolify's automatic deployment for the existing source-build app while preparing the change. Otherwise a push to `main` can start another resource-intensive build.
2. Merge the PR containing this workflow into `main`.
3. Open GitHub → Actions → **Publish ARM64 deployment images**. Run `workflow_dispatch` manually if the workflow has not already succeeded on the merge commit. **Do not proceed until all four image jobs succeed.**
4. GHCR packages are initially private, even for a public source repository. In GitHub's Packages settings, either make all four `restringing-ie-*` packages public, or configure Coolify's registry authentication with a read-only package token. Public packages have anonymous pull support. Do not paste tokens into Docker Compose.
5. In Coolify, update the existing app's **Docker Compose Location** to `/deploy/compose.prebuilt.yml`. Keep the root base directory and existing app environment variables. Add `IMAGE_TAG=main-arm64` (or a successfully built `<commit-sha>-arm64` tag). Keep `FRONTEND_ORIGIN=https://preview.restringing.ie`.
6. Confirm the dashboard domains still route `preview.restringing.ie` to service `web` internal port 80 and `restringing.ie` to service `landing` internal port 80. Coolify's internal proxy may require you to remove the loopback `ports:` mappings from this file; they are retained for portable self-managed proxy setups.
7. Redeploy. The logs should show **pulling images**, not `RUN go build`, `go install` or `npm ci`. Ensure migrations have completed before testing the preview API.

Do **not** run `docker compose down -v` or delete the PostgreSQL volume. This file intentionally preserves the Compose project/service names and `postgres_data` volume declaration.

## Deploy order and safety

The `main-arm64` tag is mutable. GitHub's image publishing can finish after Coolify receives a push event. **Keep auto-deploy off until an explicit release-trigger integration is added**, and redeploy only after image publishing succeeds. For reproducibility, use the exact release commit SHA tag (with `-arm64`) in `IMAGE_TAG` once all four images are published.

Nothing is enabled for Twilio or Stripe by switching Compose files; keep them disabled until configured.

## Troubleshooting

On the VPS, check free memory and OOM messages:

```sh
free -h
df -h /
sudo journalctl -k --since "60 minutes ago" | grep -iE 'oom|out of memory|killed process' || true
docker stats --no-stream
```

If GHCR refuses `docker pull` with `unauthorized`, configure package visibility or registry authentication first. Do not repeatedly trigger builds on the VPS.

Building native ARM64 images in GitHub Actions does not tie the app to Hetzner: the source Dockerfiles and original `deploy/compose.yml` remain available for other hosts. Separate AMD64 images can be published later.
