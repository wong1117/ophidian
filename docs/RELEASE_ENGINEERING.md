# Release Engineering

## Versioning

Ophidian follows [Semantic Versioning](https://semver.org/):

- **MAJOR** (X.0.0): Breaking changes to API or architecture
- **MINOR** (0.X.0): New features, backward compatible
- **PATCH** (0.0.X): Bug fixes, backward compatible

Pre-release tags: `v0.2.0-alpha.1`, `v0.2.0-rc.1`

## Release Process

### 1. Automated Release (Recommended)

Push a semver tag to trigger the release pipeline:

```bash
git tag v0.2.0
git push origin v0.2.0
```

The CI pipeline will:
- Validate version format
- Run full test suite
- Build binaries for linux/darwin × amd64/arm64
- Generate SBOM (SPDX JSON)
- Generate changelog from conventional commits
- Create GitHub Release with artifacts
- Build and push Docker image to ghcr.io

### 2. Manual Release

Use the release script for local release preparation:

```bash
./scripts/release.sh 0.2.0
```

This performs:
1. Build reproducibility verification
2. Full test suite with race detector
3. Cross-compilation for all platforms
4. SHA-256 checksum generation
5. SBOM generation via Syft
6. tar.gz archive creation
7. Changelog generation
8. Git tag creation

### 3. Post-Release

After creating the release:

```bash
# Push the tag
git push origin v0.2.0

# Create GitHub Release manually:
# 1. Go to https://github.com/ophidian/ophidian/releases/new
# 2. Select the v0.2.0 tag
# 3. Paste CHANGELOG.md content as release notes
# 4. Attach files from dist/ directory
```

## Build Reproducibility

All builds use:
- `-trimpath` to remove local filesystem paths
- `-ldflags="-s -w"` to strip debug information
- Go module lockfile (`go.sum`) for deterministic dependencies
- `CGO_ENABLED=0` for static binaries

Verify reproducibility:
```bash
go build -trimpath -ldflags="-s -w" -o build1 ./cmd/ophidian-server
go build -trimpath -ldflags="-s -w" -o build2 ./cmd/ophidian-server
cmp -s build1 build2 && echo "Reproducible!" || echo "NOT reproducible!"
```

## SBOM

Software Bill of Materials is generated in SPDX JSON format using Syft.
The SBOM includes all Go dependencies with versions and licenses.

```bash
# Generate SBOM
go install github.com/anchore/syft/cmd/syft@latest
syft packages dir:. -o spdx-json > ophidian.sbom.spdx.json

# View SBOM
cat ophidian.sbom.spdx.json | jq '.packages[] | {name: .name, version: .versionInfo}'
```

## Docker Images

```bash
# Build
docker build -t ophidian:latest .
docker build --build-arg VERSION=0.2.0 --build-arg COMMIT=abc123 -t ophidian:0.2.0 .

# Run
docker run -p 8080:8080 ophidian:latest

# Multi-arch (requires buildx)
docker buildx build --platform linux/amd64,linux/arm64 -t ghcr.io/ophidian/ophidian:0.2.0 --push .
```

## CI/CD Pipeline

### CI (on push/PR to main)
1. Lint (golangci-lint)
2. Test (race detector, coverage, matrix: go 1.22 + 1.23)
3. Fuzz (60 second fuzz on scheduler, queue, secrets, feature flags)
4. Build (all binaries)

### CD (on tag push)
1. Validate version
2. Run full CI
3. Generate SBOM
4. Build matrix (linux/darwin × amd64/arm64)
5. Package + checksum
6. Generate changelog
7. Create GitHub Release with artifacts
8. Push Docker image
