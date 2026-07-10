# CI/CD & Image Publishing

How container images are built, scanned, published, and released.

## The model: build once on main, promote by digest

Every push to `main` builds each service image, scans it, and publishes it to
Docker Hub under an **immutable `:<sha>` tag**. That image is the artifact — it
is what the tests ran against and what deployments should pin.

When a release is cut, nothing is rebuilt. The exact `:<sha>` image that `main`
already built and scanned is **retagged** to `:<version>`. Docker builds are not
reproducible (base images move, `npm ci` and `go mod` resolve against live
registries), so rebuilding at release time would ship bits that were never
tested. Retagging a digest makes that impossible and is also far faster.

| Tag | Meaning | Mutable? | Set by |
| --- | --- | --- | --- |
| `:<sha>` | Exactly what CI built for that commit | No | `ci.yaml` (main only) |
| `:latest` | Newest successful `main` build | Yes | `ci.yaml` `retag` job |
| `:<version>` (e.g. `0.7.1`) | A cut release | No | `release-please.yaml` `promote` job |

Deployments should pin `:<sha>` or `:<version>`, never `:latest`.

## Images

The image list is defined once in [`.github/images.json`](../.github/images.json)
and consumed by every workflow that needs it. `internal/config`'s
`TestCIImageMatrixMatchesContract` (run by the `envcheck` CI job) asserts this
file lists exactly the services in the [deployment contract](../deploy/contract.json),
so a service can never be added to the contract but forgotten in the pipeline.

All six images publish under the `gprins/` Docker Hub namespace, each named
`domain-os-<role>`: `domain-os-api`, `domain-os-worker`, `domain-os-epp`,
`domain-os-whois`, `domain-os-mcp`, `domain-os-frontend`.

## Workflows

### `ci.yaml` — on every push to `main` and every PR

- **Tests & checks**: secret scan, lint, env/contract drift, Go unit tests, Go
  API integration tests, frontend tests.
- **`build-images`** (matrix over `images.json`): builds each image and runs a
  Trivy scan gated on `CRITICAL`/`HIGH`.
  - On `main`: multi-arch (`amd64`+`arm64`), pushed as `:<sha>`.
  - On PRs: `amd64`-only, **not pushed** — loaded locally and scanned, so PRs
    never write to the registry and fork PRs without secrets still pass.
- **`retag`** (main only): retags each `:<sha>` as `:latest`.

### `release-please.yaml` — on push to `main`

- **`release-please`**: maintains the release PR; on merge, creates the GitHub
  release, tag, and changelog, and bumps the version.
- **`promote`** (only when a release was created): waits for the release
  commit's `:<sha>` images (CI builds them in parallel) and retags each to
  `:<version>`. No rebuild.

### `trivy-scheduled.yaml` — Mondays 06:00 UTC (and manual)

Non-blocking (`exit-code: 0`) Trivy scan of every `:latest` image, so base-image
CVE drift surfaces on a schedule instead of ambushing an unrelated PR. Fixing a
finding means bumping a base image (or adding `apk upgrade`) via a normal PR.

## Required GitHub configuration

Set these on the repository (Settings → Secrets and variables → Actions):

| Name | Kind | Value | Used by |
| --- | --- | --- | --- |
| `DOCKERHUB_USERNAME` | **Variable** | `gprins` | Docker Hub login (push/retag/promote/scan) |
| `DOCKERHUB_TOKEN` | **Secret** | Docker Hub access token | same |
| `GITHUB_TOKEN` | Automatic | — | secret scan, release-please (injected by Actions) |

`DOCKERHUB_USERNAME` is a public namespace, not a credential, so it is a
repository **variable** (`vars.`) — it shows up unmasked in logs, which makes a
failed push readable. Only `DOCKERHUB_TOKEN` is a secret. No `GITLEAKS_LICENSE`
is needed because the repo is under a personal account, not an organization.

### Rotating the Docker Hub token

1. Create a new access token at Docker Hub → Account Settings → Personal access
   tokens (scope: Read & Write).
2. Update the `DOCKERHUB_TOKEN` repository secret.
3. Revoke the old token.

## Why release promotion lives in `release-please.yaml`

A separate workflow triggered by the release tag would never fire: GitHub does
not trigger workflow runs from events created by the default `GITHUB_TOKEN`, and
release-please creates its tag with that token. Running `promote` as a job in
the same workflow as the release-please action sidesteps this entirely — no PAT,
no second workflow, no cross-workflow event to break.
