# Repository cleanup record (16 November 2026)

This dated record describes cleanup against `main` at `57dc3ce` after integration PR #42 merged.
Backend, Frontend CI and all native image publishing jobs on that main commit
completed successfully. No application, migration, dependency or deployment
behaviour is changed by this cleanup.

## Open PR review

| PR | Finding | Disposition |
| --- | --- | --- |
| #30 | Draft foundations are included by #42. Its remaining unique commit is merge metadata with no additional file changes. Its original description predates the integrated implementation. | Closed as superseded, not merged or approved as operative legal wording. |
| #35 | Financial-facts commit `88c1716` is an ancestor of current main. Its feature-branch base is obsolete. | Closed as superseded by #42. |
| #36 | Hardening and account-lock commits through `b4d1323` are ancestors of current main. Its feature-branch base is obsolete. | Closed as superseded by #42. |

Each PR's existing backend/frontend checks was successful, but no new merge was
needed: the integrated main has newer successful checks. Closing these PRs does
not complete the legal or operational approvals in the launch checklist.

## Branch review

Removed 15 redundant remote branches. Twelve heads were ancestors of main:
`feat/compliance-hardening`, `feat/gdpr-account-management`, `feat/grip-addons`,
`feat/integrate-compliance`, `feat/marketplace-transparency`, `feat/order-tracking`,
`feat/prebuilt-arm64-images`, `feat/tax-reporting-readiness`,
`feat/trader-compliance`, `fix/coolify-build-contexts`,
`fix/lightweight-goose-build`, `fix/publish-amd64-and-arm64`.

The foundations and checkout heads had only additional merge metadata and no
merge-base-to-head file differences. The phone-auth branch's non-merge dashboard
commit was patch-equivalent to a commit already on main (`git cherry` reported
`-`, not `+`). These three branches were redundant as well.

Retained `feat/portable-docker-deployment`: commit `c235859` adds an 83-line
`.github/workflows/deployment.yml` absent from main. This is unique work, despite
PR #29 having been merged earlier. The workflow needs review before reuse: its
Compose commands predate the repository-root build-context fix and lack explicit
`--project-directory .`. It is not silently integrated or discarded here.

Before deletion, all fetched refs and local refs were saved to a verified complete
Git bundle outside the checkout. Remote deletions used an atomic push with
explicit expected-SHA leases, so a concurrent branch update would block deletion.
PR discussions and commits reachable from main remain in GitHub history.

## File cleanup

Updated the obsolete frontend README: the app is no longer a static first
milestone. It now documents actual preview startup, Node 24, checks, generated
contracts and current product scope. Removed the empty root `tests/e2e/.gitkeep`;
real browser tests remain under `frontend/tests/e2e`, as configured by Playwright.
