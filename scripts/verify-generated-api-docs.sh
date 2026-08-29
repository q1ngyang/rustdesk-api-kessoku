#!/usr/bin/env bash

set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

command -v swag >/dev/null 2>&1
go version -m "$(command -v swag)" \
  | grep -F $'\tmod\tgithub.com/swaggo/swag\tv1.16.6\t' >/dev/null

swag init -g cmd/apimain.go --output docs/api \
  --instanceName api --exclude http/controller/admin
swag init -g cmd/apimain.go --output docs/admin \
  --instanceName admin --exclude http/controller/api

if ! git diff --exit-code -- docs; then
  echo "Generated API documentation is stale." >&2
  echo "Regenerate it with swag v1.16.6 before merging or preparing a release." >&2
  exit 1
fi
