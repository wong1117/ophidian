# Ophidian Release Automation

## CI/CD Pipeline Overview

```
PR/Push to main
  └─ CI Pipeline
       ├─ Lint (golangci-lint + go-arch-lint)
       ├─ Test (matrix: Go 1.22/1.23, PostgreSQL, Redis, race detector)
       ├─ Coverage Gate (50% minimum, enforced)
       ├─ Fuzz (120s scheduler + queue + secrets, 60s feature flags)
       ├─ Benchmark (2x3s, regression detection vs baseline)
       └─ Build (all binaries + SBOM)

Tag Push (vX.Y.Z)
  └─ Release Pipeline
       ├─ Validate version (semver check)
       ├─ Build Matrix (linux/darwin × amd64/arm64)
       ├─ SBOM (SPDX JSON via Syft)
       ├─ Sign (cosign blob signing + certificate)
       ├─ Release Notes (git-cliff conventional commits)
       ├─ Package (tar.gz + checksums)
       ├─ Docker (multi-tag, build cache, signed)
       └─ Create GitHub Release
```

## Coverage Gate

Enforced at **50% minimum** coverage. Fails the CI if coverage drops below threshold.

```yaml
- name: Coverage Gate
  run: |
    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
    if (( $(echo "$COVERAGE < 50" | bc -l) )); then
      echo "Coverage ${COVERAGE}% is below 50% threshold!"
      exit 1
    fi
```

## Benchmark Regression Detection

Compares current benchmark results against the saved baseline on the main branch.

- **QueueEnqueue** is the key metric monitored
- Regression > 20% triggers a warning (non-blocking)
- Baseline is saved as an artifact for future comparison

## Artifact Signing

All release artifacts are signed using cosign:

```bash
# Sign binary
cosign sign-blob --yes --output-signature=binary.sig binary

# Verify
cosign verify-blob --certificate-identity=github.com/ophidian --certificate-oidc-issuer=https://token.actions.githubusercontent.com binary
```

## Release Tagging

```bash
# Stable release
git tag v0.2.0

# Pre-release
git tag v0.2.0-rc.1

# Push to trigger release
git push origin v0.2.0
```

## Docker Images

Tags pushed: `v0.2.0`, `0.2`, `sha-abc1234`
Signed with cosign keyless via GitHub OIDC.

## Version File

```
version: 0.1.0
commit:  dev
built:   2026-07-20T00:00:00Z
```

Embedded at build time via `-ldflags "-X main.version=... -X main.commit=... -X main.buildTime=..."`

## Manual Release

1. Update version in code if needed
2. Create tag: `git tag v0.2.0`
3. Push: `git push origin v0.2.0`
4. Pipeline handles: build, test, SBOM, sign, package, release, docker

Or use workflow dispatch:
1. Go to Actions → Release → Run workflow
2. Enter version number
3. Pipeline runs with manual version input
