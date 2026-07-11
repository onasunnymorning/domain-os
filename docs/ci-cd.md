# CI/CD & Image Publishing

How container images are built, scanned, published, and released.

## The model: build the release, deploy versions

We deploy tagged **releases**, not individual commits, so images are built and
published exactly once — when a release is cut. Between releases, `main` merges
run tests only and publish nothing.

- **Pull requests** run the full test suite and build every image (amd64) to
  prove it still builds and is CVE-clean. These images are loaded locally and
  scanned, never pushed.
- **Pushes to `main`** run tests only.
- **Releases** (cut by release-please) build every image multi-arch, push
  `:<version>` and `:latest`, and scan.

| Tag | Meaning | Mutable? | Set by |
| --- | --- | --- | --- |
| `:<version>` (e.g. `0.8.1`) | An immutable release | No | `release-please.yaml` → `release-build` |
| `:latest` | The most recent release | Yes | same |

Deployments pin `:<version>`, or use `:latest` to track the current release.

This replaced an earlier "build on every `main` push, promote the digest at
release" model. That guarantees every commit is a deployable artifact, but when
the deploy unit is the release it just means building the same code two or three
times per release cycle — so we build the thing we actually ship, once.

## Images

The image list is defined once in [`.github/images.json`](../.github/images.json)
and shared by `ci.yaml` and the release build. `internal/config`'s
`TestCIImageMatrixMatchesContract` (run by the `envcheck` job) asserts this file
lists exactly the services in the [deployment contract](../deploy/contract.json),
so a service can never be added to the contract but forgotten in the pipeline.

All six images publish under the `gprins/` Docker Hub namespace, each named
`domain-os-<role>`: `domain-os-api`, `domain-os-worker`, `domain-os-epp`,
`domain-os-whois`, `domain-os-mcp`, `domain-os-frontend`.

## Version

The version comes from the git tag (release-please owns tags) via
`git describe`, not a committed file — see the README's *Versioning* section. CI
stamps it into each binary (build-arg → `buildinfo.Version`, surfaced at
`/ping`), and the release build tags the images with it.

## Workflows

### `ci.yaml` — every PR and every push to `main`

- **Tests & checks** (both PRs and `main`): secret scan, lint, env/contract
  drift, Go unit + API integration tests, frontend tests.
- **`build-images`** (feature PRs only): builds each image `amd64`, loads it
  locally, and Trivy-scans (`CRITICAL`/`HIGH`). Never pushes — no Docker Hub
  login required, so fork PRs pass. Skipped on `main` pushes (tests only) and on
  release-please's version-bump PRs (no source changes to validate).

### `release-please.yaml` — push to `main`

- **`release-please`**: maintains the release PR; on merge, creates the GitHub
  release, tag, and changelog, and bumps the version.
- **`release-build`** (only when a release is created): builds every image
  multi-arch (`amd64`+`arm64`), pushes `:<version>` and `:latest`, and scans.
  This is the only place images are published.

### `trivy-scheduled.yaml` — Mondays 06:00 UTC (and manual)

Non-blocking (`exit-code: 0`) Trivy scan of every `:latest` image (= the latest
release), so base-image CVE drift surfaces on a schedule instead of ambushing a
release. Fixing a finding means bumping a base image (or adding `apk upgrade`)
via a normal PR.

## Required GitHub configuration

Set these on the repository (Settings → Secrets and variables → Actions):

| Name | Kind | Value | Used by |
| --- | --- | --- | --- |
| `DOCKERHUB_USERNAME` | **Variable** | `gprins` | Docker Hub login for the release build |
| `DOCKERHUB_TOKEN` | **Secret** | Docker Hub access token | same |
| `GITHUB_TOKEN` | Automatic | — | secret scan, release-please (injected by Actions) |

`DOCKERHUB_USERNAME` is a public namespace, not a credential, so it is a
repository **variable** (`vars.`) — it shows up unmasked in logs, which makes a
failed push readable. Only `DOCKERHUB_TOKEN` is a secret. No `GITLEAKS_LICENSE`
is needed because the repo is under a personal account, not an organization.
Pull requests need neither — they build and scan without pushing.

### Rotating the Docker Hub token

1. Create a new access token at Docker Hub → Account Settings → Personal access
   tokens (scope: Read & Write).
2. Update the `DOCKERHUB_TOKEN` repository secret.
3. Revoke the old token.

## Why the release build lives in `release-please.yaml`

A separate workflow triggered by the release tag would never fire: GitHub does
not trigger workflow runs from events created by the default `GITHUB_TOKEN`, and
release-please creates its tag with that token. Running `release-build` as a job
in the same workflow as the release-please action sidesteps this entirely — no
PAT, no second workflow, no cross-workflow event to break.
