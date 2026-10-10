---
title: Contributing
parent: Development
nav_order: 5
---

# Contributing

## Getting Started

1. Fork the repository and clone your fork locally.
2. Follow the setup steps in [Backend](backend.md) and [Frontend](frontend.md).
3. Create a feature branch from `main`.

## Development Workflow

1. Make your changes with tests.
2. Run `go test ./...` (backend) and the Playwright E2E suite (frontend)
3. Open a pull request against `main`.

## Code Style

**Backend:** Standard Go formatting (`gofmt`). Follow existing controller and service patterns with thin controllers, logic in services, always scope queries by `user_id`.

**Frontend:** TypeScript strict mode and MUI for all UI components. All user-facing strings through `i18next`. Custom hooks for data fetching, not inline `useEffect` + `fetch`.

## Continuous integration

Pull requests also run these GitHub Actions workflows:

- **Docker build and scan** (`docker-build.yml`): builds the three images (all-in-one, backend, frontend) without pushing them when a PR touches a Dockerfile, `backend/`, `frontend/` or `docker/`, and scans each with Trivy. It fails on fixable HIGH and CRITICAL vulnerabilities; unfixed ones are ignored. If a finding cannot be fixed by updating the base image or the dependency, add it to `.trivyignore` with a reason and an expiry date. Also runs on `main` and weekly.
- **Docs** (`docs.yml`): builds the Jekyll site on PRs that touch `docs/**` (deploys only from `main`). To build locally: `cd docs && bundle install && bundle exec jekyll build`.

Release images (`docker-publish.yml`) are pushed for `linux/amd64` and `linux/arm64` with an SBOM and SLSA provenance attestation attached (`docker buildx imagetools inspect <image> --format '{{ json .SBOM }}'`).

**Pinning.** Every GitHub Action is pinned to a full commit SHA with the version in a trailing comment (`uses: actions/checkout@<sha> # v4.4.0`), and Docker base images to a version tag (`golang:1.27-alpine`, `node:22-alpine`, `alpine:3.22`, `nginx-unprivileged:1.30-alpine`). Do not use `@v4`-style or `latest` references. Dependabot updates the SHAs and tags (and the comment) in its weekly PRs. To pin a new action, look up the commit of its release tag, for a tag that is annotated the commit it points to: `git ls-remote https://github.com/<owner>/<repo> 'refs/tags/<tag>*'` and use the `^{}` line if there is one. Check workflows with [actionlint](https://github.com/rhysd/actionlint).

## Pull Requests

- Keep PRs focused with one feature or fix per PR.
- AI tools may assist coding but you are responsible for the code quality. Do not open hands-off vibe-coded PRs. In those cases rather open a feature request instead.
- Describe what changed and why, not how.

## Reporting Issues

Open an issue on [GitHub](https://github.com/fbuchner/meerkat-crm/issues/new/choose). Try to include steps to reproduce for bugs. Use the feature request template for new ideas.
