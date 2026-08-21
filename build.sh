#!/bin/sh

set -eu

if [ "$(node --version)" != "v24.15.0" ] || [ "$(npm --version)" != "11.12.1" ]; then
    echo "Node.js 24.15.0 and npm 11.12.1 are required" >&2
    exit 1
fi

frontend_evidence=$(mktemp -d "${TMPDIR:-/tmp}/kessoku-frontends.XXXXXX")
trap 'rm -rf -- "$frontend_evidence"' EXIT HUP INT TERM

build_frontend() {
    name=$1
    source_dir=$2
    runtime_dir=$3
    license_file=$4
    sbom_scope=${5:-production}
    if [ ! -f "${source_dir}/package-lock.json" ] || [ ! -f "${license_file}" ]; then
        echo "missing reviewed ${name} source, lock, or license" >&2
        exit 1
    fi
    (
        cd "${source_dir}"
        npm ci
        npm run lint
        npm test
        npm audit --omit=dev --audit-level=high
        npm audit signatures
        npm run build
        find dist -type f -print0 | LC_ALL=C sort -z \
            | xargs -0 sha256sum > "${frontend_evidence}/${name}-dist-1.sha256"
        npm run build
        find dist -type f -print0 | LC_ALL=C sort -z \
            | xargs -0 sha256sum > "${frontend_evidence}/${name}-dist-2.sha256"
        diff -u "${frontend_evidence}/${name}-dist-1.sha256" \
            "${frontend_evidence}/${name}-dist-2.sha256"
        if [ "${sbom_scope}" = complete ]; then
            npm sbom --sbom-format cyclonedx \
                > "${frontend_evidence}/kessoku-${name}.cdx.json"
        else
            npm sbom --omit=dev --sbom-format cyclonedx \
                > "${frontend_evidence}/kessoku-${name}.cdx.json"
        fi
        node -e 'const fs=require("fs");const s=JSON.parse(fs.readFileSync(process.argv[1]));const missing=s.components.filter(c=>!(c.licenses||[]).length);if(missing.length)throw new Error("missing production licence metadata");' \
            "${frontend_evidence}/kessoku-${name}.cdx.json"
        if [ "${name}" = web-client ]; then
            node -e 'const fs=require("fs");const s=JSON.parse(fs.readFileSync(process.argv[1]));if(!s.components.some(c=>c.name==="@bufbuild/protobuf"&&c.version==="2.9.0"&&(c.licenses||[]).length))throw new Error("web client runtime dependency missing from SBOM");' \
                "${frontend_evidence}/kessoku-${name}.cdx.json"
        fi
    )
    rm -rf -- "${runtime_dir}"
    mkdir -p "${runtime_dir}"
    cp -a "${source_dir}/dist/." "${runtime_dir}/"
    if [ "${name}" = web-client ]; then
        test -s "${runtime_dir}/third-party-licenses/@bufbuild-protobuf-2.9.0.txt"
    fi
    cp -a "${license_file}" "${frontend_evidence}/${name}-LICENSE"
}

# Both reviewed frontends are built from this exact repository revision.
build_frontend admin-web admin-web resources/admin admin-web/LICENSE
build_frontend web-client web-client resources/client web-client/LICENSE complete

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

# Copy only reviewed runtime resources, including both repository-owned
# frontend builds; historical browser-client directories remain forbidden.
sh scripts/copy-runtime-resources.sh release/resources resources require-admin require-client
cp -ar docs release/
cp -ar conf release/
cp README.md README.zh-CN.md README_EN.md CONTAINER.md CONTAINER.zh-CN.md \
  RELEASE-NOTES-v2.8.1.md RELEASE-NOTES-v2.8.1.zh-CN.md \
  SECURITY-MODEL.md MIGRATION.md OPERATOR-RUNBOOK.md ROLLBACK-RUNBOOK.md \
  WEB-CLIENT.md WEB-CLIENT.zh-CN.md ADMIN-WEB-PROVENANCE.md RELEASE-CHECKLIST.md \
  RELEASE-PROCESS.md RELEASE_STATUS LICENSE release/
cp admin-web/LICENSE release/ADMIN-WEB-LICENSE
cp web-client/LICENSE release/WEB-CLIENT-LICENSE
cp web-client/NOTICE.md release/WEB-CLIENT-NOTICE.md
cp "${frontend_evidence}"/* release/

# Create necessary directory structures
mkdir -p release/data
mkdir -p release/runtime

echo "Build and setup completed successfully."
