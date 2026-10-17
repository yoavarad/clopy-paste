# ADR-011: Pipeline stages, environments, versioning

## Status
Accepted

## Context
Need a complete CI/CD pipeline with Helm releases on GitHub (dev) and air-gapped GitLab (prod).
Prod has no mirrored vulnerability DB.

## Decision
Stages (all via make targets, ADR-010):
1. lint — gofmt, go vet, helm lint, values.schema.json validation.
2. test — `go test -race`, coverage gate >= 80% on core packages.
3. build — multi-stage container image, minimal base, tag = version.
4. scan — trivy image + chart config scan, fail on HIGH/CRITICAL. Controlled by `SCAN_ENABLED`:
   `true` on GitHub; `false` on GitLab until a vulnerability DB mirror exists.
5. e2e — kind + Strimzi, install `clopy-paste-a` + `clopy-paste-b`, assert A->B and B->A round-trip, censoring,
   limits, LRU eviction via HTTP.
6. publish — image + Helm chart (OCI) to registry: GHCR on GitHub, GitLab registry on GitLab.
7. deploy — `helm upgrade --install --rollback-on-failure --wait` (Helm 4 name for --atomic; auto-rollback on failure).
8. smoke — post-deploy health + round-trip check.

Triggers: stages 1-5 on every push/PR/MR; 6-8 on main merge (env `dev`) and on `vX.Y.Z` tags
(env `prod`, manual approval).

Environments: `dev`, `prod`. No staging. On GitHub, deploy/smoke target an ephemeral kind
cluster; on GitLab they target OpenShift via CI-variable credentials.

Versioning: SemVer from git tag `vX.Y.Z`; chart `version` = `appVersion` = image tag. Main
builds: `X.Y.Z-dev.<shortsha>`.

## Alternatives
- Staging environment — not needed for this app.
- Skip scanning entirely — loses CVE gate where it is available (GitHub).

## Consequences
- Prod builds are not CVE-scanned until a DB mirror exists; mitigated by minimal base image and
  the GitHub scan of the same source.
