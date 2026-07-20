#!/bin/bash
set -euo pipefail

VERSION=${1:-}
if [ -z "$VERSION" ]; then
  echo "Usage: $0 <version>"
  echo "Example: $0 0.2.0"
  exit 1
fi

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Error: version must be in semver format (X.Y.Z)"
  exit 1
fi

TAG="v${VERSION}"
echo "Preparing release ${TAG}..."

# 1. Verify build reproducibility
echo "1. Verifying build reproducibility..."
go build -trimpath -ldflags="-s -w" -o /tmp/build1 ./cmd/ophidian-server
go build -trimpath -ldflags="-s -w" -o /tmp/build2 ./cmd/ophidian-server
if ! cmp -s /tmp/build1 /tmp/build2; then
  echo "ERROR: Builds are not reproducible!"
  exit 1
fi
echo "   Build reproducibility: OK"

# 2. Run full test suite
echo "2. Running tests..."
go test -race -count=1 ./...
echo "   Tests: OK"

# 3. Build release binaries
echo "3. Building release binaries..."
mkdir -p dist
for os in linux darwin; do
  for arch in amd64 arm64; do
    echo "   Building ${os}/${arch}..."
    GOOS=${os} GOARCH=${arch} go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION} -X main.commit=$(git rev-parse HEAD)" \
      -o "dist/ophidian-server-${os}-${arch}" \
      ./cmd/ophidian-server
    GOOS=${os} GOARCH=${arch} go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION} -X main.commit=$(git rev-parse HEAD)" \
      -o "dist/ophidian-cli-${os}-${arch}" \
      ./cmd/ophidian-cli
  done
done

# 4. Generate checksums
echo "4. Generating checksums..."
cd dist
sha256sum * > checksums.txt
cd ..

# 5. Generate SBOM
echo "5. Generating SBOM..."
which syft >/dev/null 2>&1 || go install github.com/anchore/syft/cmd/syft@latest
syft packages dir:dist -o spdx-json > "dist/ophidian-${VERSION}.sbom.spdx.json"

# 6. Create archives
echo "6. Creating archives..."
for os in linux darwin; do
  for arch in amd64 arm64; do
    tar -czf "dist/ophidian-${VERSION}-${os}-${arch}.tar.gz" \
      -C dist \
      "ophidian-server-${os}-${arch}" \
      "ophidian-cli-${os}-${arch}"
  done
done

# 7. Generate changelog
echo "7. Generating changelog..."
git cliff --tag "v${VERSION}" --latest --output CHANGELOG.md

# 8. Create git tag
echo "8. Creating git tag..."
git tag -a "v${VERSION}" -m "Release v${VERSION}"
echo ""
echo "Release ${TAG} prepared successfully!"
echo "Files: dist/"
echo ""
echo "Next steps:"
echo "  1. Review CHANGELOG.md"
echo "  2. git push origin v${VERSION}"
echo "  3. Create GitHub Release with dist/ files"
ls -la dist/
