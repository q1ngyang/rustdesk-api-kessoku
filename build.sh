#!/bin/sh

set -eu

if [ ! -f ./admin-web/package-lock.json ]; then
    echo "embedded admin-web source is required; see ADMIN-WEB-PROVENANCE.md" >&2
    exit 1
fi

# Build the reviewed management UI from this same source revision.
if [ "$(node --version)" != "v24.15.0" ] || [ "$(npm --version)" != "11.12.1" ]; then
    echo "Node.js 24.15.0 and npm 11.12.1 are required" >&2
    exit 1
fi
(
    cd admin-web
    npm ci
    npm run lint
    npm test
    npm audit --omit=dev --audit-level=high
    npm audit signatures
    npm run build
)
rm -rf -- ./resources/admin
mkdir -p ./resources/admin
cp -a ./admin-web/dist/. ./resources/admin/

# Respect an explicitly supplied Go target architecture without persisting
# toolchain settings into the developer's global Go environment.
KESSOKU_BUILD_ARCH=${GOARCH:-$(go env GOARCH)}
DOCS="true"
# Safely remove the old release directory
rm -rf -- ./release

go mod download

# Generate Swagger documentation if DOCS is not empty
if [ -n "${DOCS}" ]; then
    echo "Generating Swagger documentation with swag v1.16.6..."
    go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/apimain.go --output docs/api --instanceName api --exclude http/controller/admin
    go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/apimain.go --output docs/admin --instanceName admin --exclude http/controller/api
else
    echo "Skipping Swagger documentation generation due to DOCS is empty."
fi

# Compile the Go code and output it to the release directory
mkdir -p release
CGO_ENABLED=1 GOOS=linux GOARCH="${KESSOKU_BUILD_ARCH}" go build -trimpath -buildvcs=true -o release/kessoku-api ./cmd
KESSOKU_SOURCE_COMMIT=$(git rev-parse HEAD)
go version -m release/kessoku-api > release/GO-BUILD-INFO.txt
grep -F "github.com/q1ngyang/rustdesk-api-kessoku/v2/cmd" release/GO-BUILD-INFO.txt >/dev/null
grep -F "vcs.revision=${KESSOKU_SOURCE_COMMIT}" release/GO-BUILD-INFO.txt >/dev/null
grep -F "vcs.modified=false" release/GO-BUILD-INFO.txt >/dev/null

# Copy only backend-owned resources. Browser clients are external providers
# and must never be included by this build.
sh scripts/copy-runtime-resources.sh release/resources resources require-admin
cp -ar docs release/
cp -ar conf release/
cp README.md README.zh-CN.md README_EN.md CONTAINER.md CONTAINER.zh-CN.md \
  RELEASE-NOTES-v2.8.0.md RELEASE-NOTES-v2.8.0.zh-CN.md \
  SECURITY-MODEL.md MIGRATION.md OPERATOR-RUNBOOK.md ROLLBACK-RUNBOOK.md \
  WEB-CLIENT-PROVIDER.md ADMIN-WEB-PROVENANCE.md RELEASE-CHECKLIST.md \
  RELEASE-PROCESS.md RELEASE_STATUS LICENSE release/
cp admin-web/LICENSE release/ADMIN-WEB-LICENSE

# Create necessary directory structures
mkdir -p release/data
mkdir -p release/runtime

echo "Build and setup completed successfully."
