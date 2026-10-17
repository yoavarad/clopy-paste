# ADR-010: CI/CD logic in Makefile + scripts, thin GitHub/GitLab wrappers

## Status
Accepted

## Context
Development on GitHub; prod CI/CD is air-gapped GitLab. Both pipelines must stay equivalent.

## Decision
All pipeline logic lives in `Makefile` targets and `scripts/` (POSIX sh). `.github/workflows/`
and `.gitlab-ci.yml` only invoke make targets. Core steps use no marketplace actions or remote
templates; required tools come from the CI runner image.

## Alternatives
- Two native pipelines — idiomatic but drift; GitHub actions unusable in air-gapped GitLab.
- GitHub only now — prod pipeline untested until migration.

## Consequences
- Developers run the exact pipeline locally via `make`.
- `.gitlab-ci.yml` is exercised locally (e.g. `gitlab-ci-local`) since no GitLab instance here.
- CI runner image must contain go, docker/buildah, helm, kubectl, kind (for e2e).
