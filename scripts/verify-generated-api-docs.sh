#!/usr/bin/env bash

set -euo pipefail

repo_root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

command -v swag >/dev/null 2>&1
go version -m "$(command -v swag)" \
  | grep -F $'\tmod\tgithub.com/swaggo/swag\tv1.16.6\t' >/dev/null

generated_docs=(
  docs/api/api_docs.go
  docs/api/api_swagger.json
  docs/api/api_swagger.yaml
  docs/admin/admin_docs.go
  docs/admin/admin_swagger.json
  docs/admin/admin_swagger.yaml
)
verification_dir=$(mktemp -d "${TMPDIR:-/var/tmp}/kessoku-api-docs.XXXXXX")
trap 'rm -rf "$verification_dir"' EXIT HUP INT TERM
sha256sum "${generated_docs[@]}" > "$verification_dir/before.sha256"

swag init -g cmd/apimain.go --output docs/api \
  --instanceName api --exclude http/controller/admin
swag init -g cmd/apimain.go --output docs/admin \
  --instanceName admin --exclude http/controller/api,http/controller/internalapi

sha256sum "${generated_docs[@]}" > "$verification_dir/after.sha256"
if ! cmp -s "$verification_dir/before.sha256" "$verification_dir/after.sha256"; then
  echo "Generated API documentation is stale." >&2
  echo "Regenerate it with swag v1.16.6 before merging or preparing a release." >&2
  exit 1
fi
