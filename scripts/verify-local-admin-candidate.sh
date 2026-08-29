#!/usr/bin/env bash

set -euo pipefail
umask 022

if [[ $# -ne 0 ]]; then
  echo "usage: $0" >&2
  exit 64
fi

repo_root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
release_tag=$(sed -n 's/^release_tag: //p' "$repo_root/RELEASE_STATUS")
printf '%s' "$release_tag" \
  | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+([.-][A-Za-z0-9.]+)?$'
local_artifact_label="${release_tag}-local-candidate"
release_notes_relative="docs/releases/${release_tag}"
admin_web_root="$repo_root/admin-web"
web_client_root="$repo_root/web-client"
evidence_dir=${KESSOKU_LOCAL_EVIDENCE_DIR:-}
admin_import_commit=2a9d037fc271cf96b39fd4add4b97c4ff4477f12
admin_seed_commit=3998c2a9213fcd047252776d0f0db33e6717026c
go_image=golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36
node_image=node:24.15.0-bookworm@sha256:f22d6a1f082c02f292e86929b5b0442ac2e5eaf438a5dea9b1566601c3e05940
debian_test_image=debian:bookworm-slim@sha256:abd67ffcfa541b485a3dff59865ab629aa048a6c613e639d36e7456b0b229241

if [[ -n $(git -C "$repo_root" status --porcelain --untracked-files=all) ]]; then
  echo "local candidate requires a clean reviewed Git commit" >&2
  exit 65
fi
source_sha=$(git -C "$repo_root" rev-parse HEAD)
printf '%s' "$source_sha" | grep -Eq '^[0-9a-f]{40}$'
if [[ -n "$evidence_dir" ]]; then
  case "$evidence_dir" in
    /*) ;;
    *) echo "KESSOKU_LOCAL_EVIDENCE_DIR must be an absolute path" >&2; exit 64 ;;
  esac
  if [[ "$evidence_dir" == / || -e "$evidence_dir" ]]; then
    echo "evidence destination must be a new non-root path: $evidence_dir" >&2
    exit 64
  fi
  case "$evidence_dir" in
    "$repo_root"|"$repo_root"/*)
      echo "evidence destination must be outside the source repository" >&2
      exit 64
      ;;
  esac
fi

if [[ "$admin_web_root" == / || ! -f "$admin_web_root/package-lock.json" ]]; then
  echo "refusing invalid admin-web source: $admin_web_root" >&2
  exit 64
fi
if [[ "$web_client_root" == / || ! -f "$web_client_root/package-lock.json" ]]; then
  echo "refusing invalid web-client source: $web_client_root" >&2
  exit 64
fi
if ! grep -Fq "$admin_import_commit" "$admin_web_root/PROVENANCE.md" || \
   ! grep -Fq "$admin_seed_commit" "$admin_web_root/PROVENANCE.md"; then
  echo "embedded admin-web provenance is incomplete" >&2
  exit 65
fi

python3 "$repo_root/scripts/check_docs.py"
python3 "$repo_root/scripts/check_release_identity.py"
python3 -m unittest discover -s "$repo_root/scripts" -p 'test_documentation.py'
docker compose --env-file "$repo_root/examples/compose.env.example" \
  -f "$repo_root/docker-compose.yaml" config --quiet

candidate_root=$(mktemp -d /tmp/kessoku-local-candidate.XXXXXX)
case "$candidate_root" in
  /tmp/kessoku-local-candidate.*) ;;
  *) echo "unsafe temporary path" >&2; exit 70 ;;
esac
candidate_suffix=${candidate_root##*.}
image_tag="kessoku-local-candidate:${candidate_suffix}"
container_name="kessoku-local-http-${candidate_suffix}"
current_uid=$(id -u)
current_gid=$(id -g)

cleanup() {
  if docker container inspect "$container_name" >/dev/null 2>&1; then
    docker stop "$container_name" >/dev/null
  fi
  for _ in $(seq 1 20); do
    if ! docker container inspect "$container_name" >/dev/null 2>&1; then
      break
    fi
    sleep 0.1
  done
  if docker container inspect "$container_name" >/dev/null 2>&1; then
    docker rm "$container_name" >/dev/null
  fi
  if docker image inspect "$image_tag" >/dev/null 2>&1; then
    docker image rm "$image_tag" >/dev/null
  fi
  if [[ -d "$candidate_root" ]]; then
    chmod -R u+w -- "$candidate_root" 2>/dev/null || true
    rm -r -- "$candidate_root"
  fi
}
trap cleanup EXIT HUP INT TERM

backend_source="$candidate_root/backend-source"
admin_source="$backend_source/admin-web"
web_client_source="$backend_source/web-client"
build_output="$candidate_root/build"
candidate="$candidate_root/candidate"
mkdir -p "$build_output" \
  "$candidate/release" "$candidate_root/packages-a" \
  "$candidate_root/packages-b"

git clone --quiet --no-local "$repo_root" "$backend_source"
git -C "$backend_source" checkout --quiet --detach "$source_sha"
test "$(git -C "$backend_source" rev-parse HEAD)" = "$source_sha"
test -z "$(git -C "$backend_source" status --porcelain --untracked-files=all)"

docker run --rm \
  -v "$backend_source:/src" -w /src/admin-web "$node_image" \
  bash -euo pipefail -c "
    cleanup_admin() { chown -R ${current_uid}:${current_gid} /src/admin-web; }
    trap cleanup_admin EXIT
    test \"\$(node --version)\" = v24.15.0
    test \"\$(npm --version)\" = 11.12.1
    npm ci
    npm run lint
    npm test
    npm audit --omit=dev --audit-level=high
    npm audit signatures
    npm run build >/tmp/kessoku-admin-build-1.log
    find dist -type f -print0 | LC_ALL=C sort -z \
      | xargs -0 sha256sum > dist-1.sha256
    npm run build >/tmp/kessoku-admin-build-2.log
    find dist -type f -print0 | LC_ALL=C sort -z \
      | xargs -0 sha256sum > dist-2.sha256
    diff -u dist-1.sha256 dist-2.sha256
    npm sbom --omit=dev --sbom-format cyclonedx > admin-web.cdx.json
    node -e 'const fs=require(\"fs\"); const s=JSON.parse(fs.readFileSync(\"admin-web.cdx.json\")); const m=s.components.filter(c=>!(c.licenses||[]).length); if(m.length) throw new Error(\"missing license metadata\"); console.log(\"production_components=\"+s.components.length+\" missing_licenses=0\")'
    chown -R ${current_uid}:${current_gid} /src/admin-web
  "

docker run --rm \
  -v "$backend_source:/src" -w /src/web-client "$node_image" \
  bash -euo pipefail -c "
    cleanup_client() { chown -R ${current_uid}:${current_gid} /src/web-client; }
    trap cleanup_client EXIT
    test \"\$(node --version)\" = v24.15.0
    test \"\$(npm --version)\" = 11.12.1
    npm ci
    npm run lint
    npm test
    npm audit --omit=dev --audit-level=high
    npm audit signatures
    npm run build >/tmp/kessoku-client-build-1.log
    find dist -type f -print0 | LC_ALL=C sort -z \
      | xargs -0 sha256sum > dist-1.sha256
    npm run build >/tmp/kessoku-client-build-2.log
    test -s dist/third-party-licenses/@bufbuild-protobuf-2.9.0.txt
    find dist -type f -print0 | LC_ALL=C sort -z \
      | xargs -0 sha256sum > dist-2.sha256
    diff -u dist-1.sha256 dist-2.sha256
    npm sbom --sbom-format cyclonedx > web-client.cdx.json
    node -e 'const fs=require(\"fs\"); const s=JSON.parse(fs.readFileSync(\"web-client.cdx.json\")); const m=s.components.filter(c=>!(c.licenses||[]).length); if(m.length) throw new Error(\"missing license metadata\"); if(!s.components.some(c=>c.name===\"@bufbuild/protobuf\"&&c.version===\"2.9.0\"&&(c.licenses||[]).length)) throw new Error(\"web client runtime dependency missing from SBOM\"); console.log(\"components=\"+s.components.length+\" missing_licenses=0\")'
    chown -R ${current_uid}:${current_gid} /src/web-client
  "

mkdir -p "$backend_source/resources/admin"
cp -a "$admin_source/dist/." "$backend_source/resources/admin/"
mkdir -p "$backend_source/resources/client"
cp -a "$web_client_source/dist/." "$backend_source/resources/client/"
if [[ -n $(git -C "$backend_source" status --porcelain --untracked-files=all) ]]; then
  echo "local backend snapshot is not clean" >&2
  exit 65
fi

docker run --rm \
  -v "$backend_source:/src:ro" \
  -v "$build_output:/out" \
  -v kessoku-go126-mod:/go/pkg/mod \
  -v kessoku-go126-build:/root/.cache/go-build \
  -w /src "$go_image" \
  bash -euo pipefail -c "
    cleanup_output() { chown -R ${current_uid}:${current_gid} /out; }
    trap cleanup_output EXIT
    git config --global --add safe.directory /src
    {
      go mod verify
      printf 'go_mod_verify=PASS\n'
      test -z \"\$(git ls-files -z -- '*.go' | xargs -0 -r gofmt -l)\"
      printf 'gofmt=PASS\n'
      go vet ./...
      printf 'go_vet=PASS\n'
      go test -count=1 -timeout=10m ./...
      printf 'go_test=PASS\n'
      go test -count=1 -race -timeout=10m ./...
      printf 'go_test_race=PASS\n'
    } | tee /out/GO-VERIFY.txt
    test -z \"\$(git status --porcelain --untracked-files=all)\"
    CGO_ENABLED=1 go build -trimpath -buildvcs=true -ldflags '-s -w' \
      -o /out/kessoku-api ./cmd
    CGO_ENABLED=1 go build -trimpath -buildvcs=true -ldflags '-s -w' \
      -o /out/kessoku-api.rebuild ./cmd
    cmp /out/kessoku-api /out/kessoku-api.rebuild
    rm /out/kessoku-api.rebuild
    go version -m /out/kessoku-api > /out/GO-BUILD-INFO.txt
    grep -F $'\tpath\tgithub.com/q1ngyang/rustdesk-api-kessoku/v3/cmd' \
      /out/GO-BUILD-INFO.txt
    grep -F $'\tbuild\tvcs=git' /out/GO-BUILD-INFO.txt
    grep -F $'\tbuild\tvcs.revision=${source_sha}' /out/GO-BUILD-INFO.txt
    grep -F $'\tbuild\tvcs.modified=false' /out/GO-BUILD-INFO.txt
    chown -R ${current_uid}:${current_gid} /out
  "

release="$candidate/release"
install -m 0755 "$build_output/kessoku-api" "$release/kessoku-api"
install -m 0644 "$build_output/GO-BUILD-INFO.txt" \
  "$candidate/GO-BUILD-INFO.txt"
install -m 0644 "$build_output/GO-VERIFY.txt" \
  "$candidate/GO-VERIFY.txt"
sh "$backend_source/scripts/copy-runtime-resources.sh" \
  "$release/resources" "$backend_source/resources" require-admin require-client
printf '%s\n' "$local_artifact_label" > "$release/resources/version"
cp -a "$backend_source/conf" "$backend_source/docs" "$release/"
for document in README.md README.zh-CN.md README_EN.md RELEASE_STATUS LICENSE; do
  cp -a "$backend_source/$document" "$release/"
done
install -m 0644 "$admin_source/LICENSE" "$release/ADMIN-WEB-LICENSE"
install -m 0644 "$web_client_source/LICENSE" "$release/WEB-CLIENT-LICENSE"
install -m 0644 "$web_client_source/NOTICE.md" "$release/WEB-CLIENT-NOTICE.md"
install -m 0644 "$admin_source/dist-2.sha256" \
  "$candidate/ADMIN-WEB-DIST-SHA256SUMS"
install -m 0644 "$admin_source/admin-web.cdx.json" \
  "$candidate/kessoku-admin-web.cdx.json"
install -m 0644 "$web_client_source/dist-2.sha256" \
  "$candidate/WEB-CLIENT-DIST-SHA256SUMS"
install -m 0644 "$web_client_source/web-client.cdx.json" \
  "$candidate/kessoku-web-client.cdx.json"
{
  printf 'repository=%s\n' 'q1ngyang/rustdesk-api-kessoku'
  printf 'source_commit=%s\n' "$source_sha"
  printf 'release_tag=%s\n' 'UNPUBLISHED'
  printf 'artifact_label=%s\n' "$local_artifact_label"
  printf 'go_version=%s\n' '1.26.6'
  printf 'admin_web_path=%s\n' 'admin-web'
  printf 'admin_web_source_commit=%s\n' "$source_sha"
  printf 'admin_web_import_commit=%s\n' "$admin_import_commit"
  printf 'admin_web_seed_commit=%s\n' "$admin_seed_commit"
  printf 'web_client_path=%s\n' 'web-client'
  printf 'web_client_source_commit=%s\n' "$source_sha"
  printf 'web_client_license=%s\n' 'MIT'
} > "$candidate/BUILD-INPUTS.txt"

tool_dir="$candidate_root/security-tools"
sh "$backend_source/scripts/install-ci-tools.sh" "$tool_dir" \
  | tee "$candidate/SECURITY-TOOL-VERSIONS.txt"
"$tool_dir/actionlint"
"$tool_dir/gitleaks" git "$backend_source" \
  --redact --no-banner --exit-code 1
"$tool_dir/gitleaks" dir "$release" \
  --redact --no-banner --exit-code 1

docker run --rm \
  -v "$backend_source:/src:ro" \
  -v "$build_output:/out" \
  -v kessoku-go126-mod:/go/pkg/mod \
  -v kessoku-go126-build:/root/.cache/go-build \
  -w /src "$go_image" \
  bash -euo pipefail -c "
    cleanup_output() { chown -R ${current_uid}:${current_gid} /out; }
    trap cleanup_output EXIT
    go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./... \
      | tee /out/GOVULNCHECK.txt
  "
install -m 0644 "$build_output/GOVULNCHECK.txt" \
  "$candidate/GOVULNCHECK.txt"
{
  printf 'source_commit=%s\n' "$source_sha"
  printf 'actionlint=PASS\n'
  printf 'gitleaks_git_history=PASS\n'
  printf 'gitleaks_runtime_tree=PASS\n'
  printf 'govulncheck_reachable=0\n'
  printf 'govulncheck_imported_packages=0\n'
} > "$candidate/SECURITY-SCAN-SUMMARY.txt"

source_sbom_input="$candidate_root/source-sbom-input"
mkdir -p "$source_sbom_input"
git -C "$backend_source" archive "$source_sha" \
  | tar -x -C "$source_sbom_input"
"$tool_dir/syft" scan dir:"$source_sbom_input" \
  -o spdx-json="$candidate/kessoku-source.spdx.json"
"$tool_dir/syft" scan dir:"$release" \
  -o spdx-json="$candidate/kessoku-runtime.spdx.json"

test ! -e "$release/resources/web"
test ! -e "$release/resources/web2"
test -s "$release/resources/admin/index.html"
test -s "$release/resources/client/index.html"
test -s "$release/resources/client/third-party-licenses/@bufbuild-protobuf-2.9.0.txt"
find "$release/resources/admin/static/chunk" -type f \
  -name 'server_control-*.js' -print -quit | grep -q .
test -z "$(find "$release" -type l -print -quit)"

tar --sort=name --mtime='UTC 1970-01-01' \
  --owner=0 --group=0 --numeric-owner -C "$candidate" -cf - release \
  | gzip -n > "$candidate_root/candidate-a.tar.gz"
tar --sort=name --mtime='UTC 1970-01-01' \
  --owner=0 --group=0 --numeric-owner -C "$candidate" -cf - release \
  | gzip -n > "$candidate_root/candidate-b.tar.gz"
cmp "$candidate_root/candidate-a.tar.gz" \
  "$candidate_root/candidate-b.tar.gz"

sh "$backend_source/scripts/build-deb.sh" "$release" \
  '2.8.3~local.1-1' amd64 "$candidate_root/packages-a"
sh "$backend_source/scripts/build-deb.sh" "$release" \
  '2.8.3~local.1-1' amd64 "$candidate_root/packages-b"
cmp "$candidate_root/packages-a/"*.deb "$candidate_root/packages-b/"*.deb
dpkg-deb --contents "$candidate_root/packages-a/"*.deb \
  | grep -F '/resources/admin/index.html'
dpkg-deb --contents "$candidate_root/packages-a/"*.deb \
  | grep -F '/resources/client/index.html'
dpkg-deb --contents "$candidate_root/packages-a/"*.deb \
  | grep -F '/resources/client/third-party-licenses/@bufbuild-protobuf-2.9.0.txt'
dpkg-deb --contents "$candidate_root/packages-a/"*.deb \
  | grep -F '/usr/share/doc/kessoku-api/copyright'
if dpkg-deb --contents "$candidate_root/packages-a/"*.deb \
  | grep -Eq '/resources/(web|web2)/'; then
  echo "browser-client directory entered Debian package" >&2
  exit 65
fi

docker run --rm --platform linux/amd64 \
  -v "$candidate_root/packages-a:/packages:ro" \
  "$debian_test_image" sh -euxc '
    printf "#!/bin/sh\nexit 101\n" > /usr/sbin/policy-rc.d
    chmod 0755 /usr/sbin/policy-rc.d
    apt-get update
    DEBIAN_FRONTEND=noninteractive apt-get install -y /packages/*.deb
    test "$(dpkg-query -W -f=\${Version} kessoku-api)" = "2.8.3~local.1-1"
    test "$(id -un kessoku-api)" = kessoku-api
    test "$(stat -c %a /var/lib/kessoku-api/data)" = 700
    test "$(stat -c %a /var/lib/kessoku-api/runtime)" = 700
    test -s /var/lib/kessoku-api/resources/admin/index.html
    test -s /var/lib/kessoku-api/resources/client/index.html
    test -s /var/lib/kessoku-api/resources/client/third-party-licenses/@bufbuild-protobuf-2.9.0.txt
    test -s /usr/share/doc/kessoku-api/copyright
    grep -F "Copyright (c) 2016-2021 vue-manage-system" \
      /usr/share/doc/kessoku-api/copyright
    test ! -e /var/lib/kessoku-api/resources/web
    test ! -e /var/lib/kessoku-api/resources/web2
    /usr/bin/kessoku-api --help
    echo debian_package_install_runtime=PASS
  '

docker_context="$candidate_root/docker-context"
mkdir -p "$docker_context"
cp "$backend_source/Dockerfile" "$docker_context/Dockerfile"
cp -a "$release" "$docker_context/release"
docker build --quiet --tag "$image_tag" "$docker_context"
test "$(docker image inspect --format '{{.Config.User}}' "$image_tag")" \
  = '65534:65534'
docker run --rm "$image_tag" ./kessoku-api --help >/dev/null
docker run --rm "$image_tag" sh -c \
  'test ! -e resources/web && test ! -e resources/web2 && test -s resources/admin/index.html && test -s resources/client/index.html && test -s resources/client/third-party-licenses/@bufbuild-protobuf-2.9.0.txt'
docker image inspect --format '{{json .Config.ExposedPorts}}' "$image_tag" \
  | grep -F '21122/tcp'

docker run --rm -d --name "$container_name" \
  -p 127.0.0.1::21114 "$image_tag" >/dev/null
http_port=$(docker port "$container_name" 21114/tcp | sed -n 's/.*://p')
printf '%s' "$http_port" | grep -Eq '^[1-9][0-9]*$'
http_ready=0
for _ in $(seq 1 40); do
  if curl -fsS "http://127.0.0.1:${http_port}/dash/" \
    > "$candidate_root/admin-index.html" 2>/dev/null; then
    http_ready=1
    break
  fi
  sleep 0.25
done
if [[ $http_ready -ne 1 ]]; then
  docker logs "$container_name"
  exit 1
fi
curl -fsSI "http://127.0.0.1:${http_port}/dash/" \
  | tr -d '\r' > "$candidate_root/admin-headers.txt"
grep -Fi 'Content-Security-Policy:' "$candidate_root/admin-headers.txt" \
  | grep -F "frame-ancestors 'none'" \
  | grep -F "object-src 'none'" \
  | grep -F "script-src 'self'"
grep -Fxi 'X-Frame-Options: DENY' "$candidate_root/admin-headers.txt"
grep -Fxi 'X-Content-Type-Options: nosniff' "$candidate_root/admin-headers.txt"
grep -Fxi 'Referrer-Policy: no-referrer' "$candidate_root/admin-headers.txt"
test "$(curl -sS -o /dev/null -w '%{http_code}' \
  "http://127.0.0.1:${http_port}/api/admin/config/server")" = 404
test "$(curl -sS -o /dev/null -w '%{http_code}' \
  "http://127.0.0.1:${http_port}/dash/static/")" = 404

{
  printf 'image_id=%s\n' "$(docker image inspect --format '{{.Id}}' "$image_tag")"
  printf 'runtime_user=%s\n' "$(docker image inspect --format '{{.Config.User}}' "$image_tag")"
  printf 'source_commit=%s\n' "$source_sha"
} > "$candidate/LOCAL-IMAGE-IDENTITY.txt"

release_assets="$candidate_root/release-assets"
mkdir -p "$release_assets"
install -m 0644 "$candidate_root/candidate-a.tar.gz" \
  "$release_assets/kessoku-${local_artifact_label}-linux-amd64.tar.gz"
install -m 0644 "$candidate_root/packages-a/"*.deb "$release_assets/"
for artifact in ADMIN-WEB-DIST-SHA256SUMS WEB-CLIENT-DIST-SHA256SUMS BUILD-INPUTS.txt \
  GO-BUILD-INFO.txt GO-VERIFY.txt GOVULNCHECK.txt LOCAL-IMAGE-IDENTITY.txt \
  SECURITY-SCAN-SUMMARY.txt SECURITY-TOOL-VERSIONS.txt \
  kessoku-admin-web.cdx.json kessoku-web-client.cdx.json \
  kessoku-source.spdx.json kessoku-runtime.spdx.json; do
  install -m 0644 "$candidate/$artifact" "$release_assets/$artifact"
done
install -m 0644 "$admin_source/LICENSE" "$release_assets/ADMIN-WEB-LICENSE"
install -m 0644 "$web_client_source/LICENSE" "$release_assets/WEB-CLIENT-LICENSE"
install -m 0644 "$web_client_source/NOTICE.md" "$release_assets/WEB-CLIENT-NOTICE.md"
install -m 0644 "$backend_source/RELEASE_STATUS" \
  "$backend_source/docs/releases/RELEASE-CHECKLIST.md" \
  "$backend_source/$release_notes_relative/RELEASE-NOTES-${release_tag}.md" \
  "$backend_source/$release_notes_relative/RELEASE-NOTES-${release_tag}.zh-CN.md" \
  "$backend_source/docs/deployment/CONTAINER.md" \
  "$backend_source/docs/deployment/CONTAINER.zh-CN.md" \
  "$backend_source/docker-compose.yaml" \
  "$backend_source/examples/compose.env.example" \
  "$backend_source/internal/starrycontrol/CONTRACT_VERSION" \
  "$release_assets/"
install -m 0644 "$backend_source/examples/config.docker-builtin.yaml" \
  "$release_assets/config.docker-builtin.yaml"

"$tool_dir/syft" scan dir:"$release_assets" \
  -o spdx-json="$candidate_root/kessoku-candidate.spdx.json"
install -m 0644 "$candidate_root/kessoku-candidate.spdx.json" \
  "$release_assets/kessoku-candidate.spdx.json"
test -z "$(find "$release_assets" -type f \
  \( -name '*.key' -o -name '*.pem' -o -name '*.p12' \) -print -quit)"
(
  cd "$release_assets"
  checksum_file=$(mktemp "$candidate_root/SHA256SUMS.XXXXXX")
  find . -maxdepth 1 -type f ! -name SHA256SUMS \
    -printf '%f\0' | LC_ALL=C sort -z | xargs -0 sha256sum -- \
    > "$checksum_file"
  mv "$checksum_file" SHA256SUMS
  sha256sum --check SHA256SUMS
)

if [[ -n "$evidence_dir" ]]; then
  install -d -m 0755 "$evidence_dir"
  cp -a "$release_assets/." "$evidence_dir/"
  printf 'local_evidence_dir=%s\n' "$evidence_dir"
fi

printf 'source_sha=%s\n' "$source_sha"
sha256sum "$build_output/kessoku-api" \
  "$candidate_root/candidate-a.tar.gz" \
  "$candidate_root/packages-a/"*.deb \
  "$candidate/ADMIN-WEB-DIST-SHA256SUMS"
sha256sum "$candidate/WEB-CLIENT-DIST-SHA256SUMS"
printf 'local_candidate_runtime_and_headers=PASS\n'
