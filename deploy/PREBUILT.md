# No-build AMD64 / ARM64 deployment in Coolify

The current VPS is hosting Coolify and its own Go/Node source builds. If a deploy freezes the dashboard, **do not retry source builds**. Use this prebuilt deployment instead. It uses the same services and database volume names as the source Compose deployment, but pulls images built on GitHub's native AMD64 and ARM64 runners.

## Before switching

1. Disable Coolify's automatic deployment for the existing source-build app while preparing the change. Otherwise a push to `main` can start another resource-intensive build.
2. Merge the PR containing this workflow into `main`.
3. Open GitHub → Actions → **Publish deployment images (AMD64 and ARM64)**. Run `workflow_dispatch` manually if the workflow has not already succeeded on the merge commit. **Do not proceed until all four image jobs for the selected architecture succeed.**
4. GHCR packages are initially private, even for a public source repository. In GitHub's Packages settings, either make all four `restringing-ie-*` packages public, or configure Coolify's registry authentication with a read-only package token. Public packages have anonymous pull support. Do not paste tokens into Docker Compose.
5. In Coolify, update the existing app's **Docker Compose Location** to `/deploy/compose.prebuilt.yml`. Keep the root base directory and existing app environment variables. Set `IMAGE_TAG=main-amd64` for an x86 server (or `main-arm64` for an ARM server). A successful `<commit-sha>-amd64` tag can be used to pin a release. Keep `FRONTEND_ORIGIN=https://preview.restringing.ie`.
6. Confirm the dashboard domains still route `preview.restringing.ie` to service `web` internal port 80 and `restringing.ie` to service `landing` internal port 80. Coolify's internal proxy may require you to remove the loopback `ports:` mappings from this file; they are retained for portable self-managed proxy setups.
7. Redeploy. The logs should show **pulling images**, not `RUN go build`, `go install` or `npm ci`. Ensure migrations have completed before testing the preview API.

Do **not** run `docker compose down -v` or delete the PostgreSQL volume. This file intentionally preserves the Compose project/service names and `postgres_data` volume declaration.

## Deploy order and safety

The `main-amd64` and `main-arm64` tags are mutable. GitHub's image publishing can finish after Coolify receives a push event. **Keep auto-deploy off until an explicit release-trigger integration is added**, and redeploy only after image publishing succeeds. For reproducibility, use the exact release commit SHA tag (with `-amd64` for x86 or `-arm64` for ARM) in `IMAGE_TAG` once all four images are published.

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

Building native AMD64 and ARM64 images in GitHub Actions does not tie the app to Hetzner: the original Dockerfiles and `deploy/compose.yml` remain available for other hosts.
