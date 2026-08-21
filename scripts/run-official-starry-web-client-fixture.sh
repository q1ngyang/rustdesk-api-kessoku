#!/usr/bin/env bash

set -Eeuo pipefail
umask 077

if [[ $# -ne 0 ]]; then
  echo "usage: STARRY_REPO=/path/to/rustdesk-server-starry $0" >&2
  exit 64
fi

repo_root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
starry_repo=${STARRY_REPO:-}
kessoku_image=${KESSOKU_MATRIX_IMAGE:-rustdesk-api-kessoku:integration-local}
starry_image=ghcr.io/q1ngyang/rustdesk-server-starry:1.1.16-patch-v1.2.0
starry_digest=sha256:3685543aee6e60c27bed5db1df2fa32af83e61a58e9bc4c0ea3464664863811b
client_image=starry-release-client@sha256:79e92e8ddd992852682168a15914e791931d65b88747d13942342255a833c7b0
client_image_id=sha256:79e92e8ddd992852682168a15914e791931d65b88747d13942342255a833c7b0
go_image=golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36

if [[ -z "$starry_repo" ]] || \
  ! git -C "$starry_repo" rev-parse --is-inside-work-tree 2>/dev/null \
    | grep -qx true; then
  echo "STARRY_REPO must name the local rustdesk-server-starry checkout" >&2
  exit 64
fi
enforce_fixture="$starry_repo/scripts/fixtures/release-drill-enforce.yaml"
test -s "$enforce_fixture"
test -s "$repo_root/examples/config.docker-builtin.yaml"
test -s "$repo_root/scripts/fixtures/Dockerfile.web-client-target"
docker image inspect "$kessoku_image" "$client_image" "$go_image" >/dev/null
test "$(docker image inspect "$client_image" --format '{{.Id}}')" = "$client_image_id"
docker image inspect "$starry_image" --format '{{join .RepoDigests "\n"}}' \
  | grep -Fx "ghcr.io/q1ngyang/rustdesk-server-starry@${starry_digest}" >/dev/null

fixture_root=$(mktemp -d /tmp/kessoku-starry-web-client.XXXXXX)
case "$fixture_root" in
  /tmp/kessoku-starry-web-client.*) ;;
  *) echo "unsafe temporary path" >&2; exit 70 ;;
esac
fixture_id=${fixture_root##*.}
fixture_network="kessoku-starry-web-${fixture_id}"
target_test_image="kessoku-web-target:${fixture_id}"
current_uid=$(id -u)
current_gid=$(id -g)
kessoku_name="web-kessoku-${fixture_id}"
api_tls_name="web-api-tls-${fixture_id}"
client_tls_name="web-client-tls-${fixture_id}"
hbbs_name="web-hbbs-${fixture_id}"
hbbs_tls_name="web-hbbs-tls-${fixture_id}"
hbbr_name="web-hbbr-${fixture_id}"
hbbr_tls_name="web-hbbr-tls-${fixture_id}"
target_name="web-target-${fixture_id}"
target_bootstrap_name="web-target-bootstrap-${fixture_id}"
created_containers=()

cleanup() {
  if ((${#created_containers[@]})); then
    docker rm -f "${created_containers[@]}" >/dev/null 2>&1 || true
  fi
  mapfile -t orphans < <(docker ps -aq --filter "name=${fixture_id}")
  if ((${#orphans[@]})); then
    docker rm -f "${orphans[@]}" >/dev/null 2>&1 || true
  fi
  docker network rm "$fixture_network" >/dev/null 2>&1 || true
  docker image rm -f "$target_test_image" >/dev/null 2>&1 || true
  if [[ -d "$fixture_root" ]]; then
    docker run --rm -v "$fixture_root:/fixture" \
      debian:bookworm-slim@sha256:abd67ffcfa541b485a3dff59865ab629aa048a6c613e639d36e7456b0b229241 \
      chmod -R a+rwX /fixture >/dev/null 2>&1 || true
    chmod -R u+w -- "$fixture_root" 2>/dev/null || true
    rm -r -- "$fixture_root"
  fi
}
trap 'printf "web_client_fixture_failure line=%s status=%s\n" "$LINENO" "$?" >&2' ERR
trap cleanup EXIT HUP INT TERM

mkdir -p "$fixture_root/kessoku-data" "$fixture_root/kessoku-runtime" \
  "$fixture_root/starry-data/starry" "$fixture_root/secrets" \
  "$fixture_root/target"
chmod 0777 "$fixture_root/kessoku-data" "$fixture_root/kessoku-runtime" \
  "$fixture_root/starry-data" "$fixture_root/starry-data/starry" \
  "$fixture_root/target"

openssl genpkey -algorithm ED25519 \
  -out "$fixture_root/secrets/kessoku-access.pem"
openssl req -x509 -newkey rsa:2048 -sha256 -days 2 -nodes \
  -subj '/CN=Kessoku browser matrix CA' \
  -keyout "$fixture_root/secrets/ca-key.pem" \
  -out "$fixture_root/secrets/ca.pem" >/dev/null 2>&1
openssl req -newkey rsa:2048 -nodes -subj '/CN=kessoku.local' \
  -addext 'subjectAltName=DNS:kessoku.local' \
  -keyout "$fixture_root/secrets/kessoku-server-key.pem" \
  -out "$fixture_root/secrets/kessoku-server.csr" >/dev/null 2>&1
openssl x509 -req -in "$fixture_root/secrets/kessoku-server.csr" \
  -CA "$fixture_root/secrets/ca.pem" \
  -CAkey "$fixture_root/secrets/ca-key.pem" -CAcreateserial -days 2 \
  -sha256 -copy_extensions copy \
  -out "$fixture_root/secrets/kessoku-server.pem" >/dev/null 2>&1
openssl req -newkey rsa:2048 -nodes -subj '/CN=starry-hbbs' \
  -addext 'subjectAltName=URI:spiffe://example.com/starry/production' \
  -keyout "$fixture_root/secrets/client-key.pem" \
  -out "$fixture_root/secrets/client.csr" >/dev/null 2>&1
openssl x509 -req -in "$fixture_root/secrets/client.csr" \
  -CA "$fixture_root/secrets/ca.pem" \
  -CAkey "$fixture_root/secrets/ca-key.pem" -CAcreateserial -days 2 \
  -sha256 -copy_extensions copy \
  -out "$fixture_root/secrets/client.pem" >/dev/null 2>&1
openssl req -newkey rsa:2048 -nodes -subj '/CN=Kessoku browser fixture' \
  -addext 'subjectAltName=IP:127.0.0.1,DNS:api.localhost,DNS:client.localhost,DNS:hbbs.localhost,DNS:hbbr.localhost' \
  -keyout "$fixture_root/secrets/public-key.pem" \
  -out "$fixture_root/secrets/public.csr" >/dev/null 2>&1
openssl x509 -req -in "$fixture_root/secrets/public.csr" \
  -CA "$fixture_root/secrets/ca.pem" \
  -CAkey "$fixture_root/secrets/ca-key.pem" -CAcreateserial -days 2 \
  -sha256 -copy_extensions copy \
  -out "$fixture_root/secrets/public.pem" >/dev/null 2>&1
printf '%s\n' 'MatrixAdmin-42!' > "$fixture_root/secrets/admin-password"
find "$fixture_root/secrets" -type f -exec chmod 0600 {} +
docker run --rm --user "${current_uid}:${current_gid}" \
  -v "$repo_root:/src:ro" -v "$fixture_root:/out" -w /src \
  -e CGO_ENABLED=0 -e GOCACHE=/tmp/go-build "$go_image" \
  go build -trimpath -ldflags='-s -w' -o /out/tls-reverse-proxy \
  ./scripts/fixtures/tls-reverse-proxy.go
chmod 0755 "$fixture_root/tls-reverse-proxy"
docker build --build-arg "BASE_IMAGE=${client_image}" \
  -f "$repo_root/scripts/fixtures/Dockerfile.web-client-target" \
  -t "$target_test_image" "$repo_root" >/dev/null

docker network create "$fixture_network" >/dev/null

start_tls_proxy() {
  local name=$1
  local upstream=$2
  local network_alias=${3:-}
  local network_args=(--network "$fixture_network")
  if [[ -n "$network_alias" ]]; then
    network_args+=(--network-alias "$network_alias")
  fi
  docker run -d --name "$name" "${network_args[@]}" \
    -p 127.0.0.1::443 \
    -v "$fixture_root:/fixture:ro" \
    debian:bookworm-slim@sha256:abd67ffcfa541b485a3dff59865ab629aa048a6c613e639d36e7456b0b229241 \
    /fixture/tls-reverse-proxy -listen :443 -target "http://${upstream}" \
    -cert /fixture/secrets/public.pem -key /fixture/secrets/public-key.pem >/dev/null
  created_containers+=("$name")
}

published_port() {
  docker port "$1" 443/tcp | sed -n 's/.*://p' | head -1
}

start_tls_proxy "$api_tls_name" kessoku.local:21114
start_tls_proxy "$client_tls_name" kessoku.local:21122
start_tls_proxy "$hbbs_tls_name" hbbs.local:21118
start_tls_proxy "$hbbr_tls_name" hbbr.local:21119 hbbr.localhost
api_port=$(published_port "$api_tls_name")
client_port=$(published_port "$client_tls_name")
hbbs_port=$(published_port "$hbbs_tls_name")
hbbr_port=$(published_port "$hbbr_tls_name")
api_origin="https://127.0.0.1:${api_port}"
client_origin="https://127.0.0.1:${client_port}"
hbbs_wss="wss://127.0.0.1:${hbbs_port}/ws/id"
hbbr_wss="wss://127.0.0.1:${hbbr_port}/ws/relay"

kessoku_env_args=(
  -e RUSTDESK_API_AUTH_ENABLED=true
  -e "RUSTDESK_API_AUTH_ISSUER=${api_origin}"
  -e 'RUSTDESK_API_AUTH_AUDIENCES=kessoku-api,rustdesk-connect'
  -e RUSTDESK_API_AUTH_ACCESS_TOKEN_TTL=10m
  -e RUSTDESK_API_AUTH_MAXIMUM_TOKEN_TTL=10m
  -e RUSTDESK_API_AUTH_CURRENT_KEY_ID=web-matrix-current
  -e RUSTDESK_API_AUTH_CURRENT_KEY_PRIVATE_KEY_FILE=/test-secrets/kessoku-access.pem
  -e RUSTDESK_API_AUTH_INTERNAL_ENABLED=true
  -e RUSTDESK_API_AUTH_INTERNAL_LISTEN=0.0.0.0:21121
  -e RUSTDESK_API_AUTH_INTERNAL_SERVER_CERT_FILE=/test-secrets/kessoku-server.pem
  -e RUSTDESK_API_AUTH_INTERNAL_SERVER_KEY_FILE=/test-secrets/kessoku-server-key.pem
  -e RUSTDESK_API_AUTH_INTERNAL_CLIENT_CA_FILE=/test-secrets/ca.pem
  -e RUSTDESK_API_AUTH_INTERNAL_ALLOWED_URI_SANS=spiffe://example.com/starry/production
  -e "RUSTDESK_API_RUSTDESK_API_SERVER=${api_origin}"
)

start_kessoku() {
  local config_mount=()
  if [[ -s "$fixture_root/config.yaml" ]]; then
    config_mount=(-v "$fixture_root/config.yaml:/app/conf/config.yaml:ro")
  fi
  docker run -d --name "$kessoku_name" --user "${current_uid}:${current_gid}" \
    --network "$fixture_network" --network-alias kessoku.local \
    -v "$fixture_root/kessoku-data:/app/data" \
    -v "$fixture_root/kessoku-runtime:/app/runtime" \
    -v "$fixture_root/secrets:/test-secrets:ro" \
    "${config_mount[@]}" "${kessoku_env_args[@]}" "$kessoku_image" >/dev/null
  created_containers+=("$kessoku_name")
  for _ in $(seq 1 80); do
    if docker run --rm --network "$fixture_network" "$client_image" \
      curl -fsS http://kessoku.local:21114/api/version >/dev/null 2>&1; then
      return
    fi
    if [[ $(docker inspect --format '{{.State.Running}}' "$kessoku_name") != true ]]; then
      docker logs "$kessoku_name" >&2
      return 1
    fi
    sleep 0.25
  done
  docker logs "$kessoku_name" >&2
  return 1
}

remove_container() {
  local target=$1
  docker rm -f "$target" >/dev/null
  local kept=()
  for name in "${created_containers[@]}"; do
    [[ "$name" == "$target" ]] || kept+=("$name")
  done
  created_containers=("${kept[@]}")
}

start_kessoku
remove_container "$kessoku_name"
docker run --rm --user "${current_uid}:${current_gid}" \
  --network "$fixture_network" \
  -v "$fixture_root/kessoku-data:/app/data" \
  -v "$fixture_root/kessoku-runtime:/app/runtime" \
  -v "$fixture_root/secrets:/test-secrets:ro" \
  "${kessoku_env_args[@]}" "$kessoku_image" ./kessoku-api reset-admin-pwd \
  --password-file /test-secrets/admin-password >/dev/null
start_kessoku

: > "$fixture_root/secrets/jwks.json"
chmod 0600 "$fixture_root/secrets/jwks.json"
docker run --rm --user "${current_uid}:${current_gid}" \
  --network "$fixture_network" -v "$fixture_root/secrets:/test-secrets" \
  "$client_image" sh -lc \
  'curl -fsS --cacert /test-secrets/ca.pem --cert /test-secrets/client.pem --key /test-secrets/client-key.pem https://kessoku.local:21121/api/internal/v1/auth/jwks -o /test-secrets/jwks.json'

sed -e "s|  allowed_origins: \[\]|  allowed_origins:\n    - \"${client_origin}\"|" \
  -e "s|  issuer: https://kessoku.local|  issuer: ${api_origin}|" \
  -e 's|wss://hbbr.local/ws/relay|wss://hbbr.localhost/ws/relay|' \
  "$enforce_fixture" > "$fixture_root/starry-config.yaml"
chmod 0644 "$fixture_root/starry-config.yaml"

docker run -d --name "$hbbs_name" --network "$fixture_network" \
  --network-alias hbbs.local --entrypoint sh \
  -v "$fixture_root/starry-data:/root" \
  -v "$fixture_root/secrets:/test-secrets" \
  -v "$fixture_root/starry-config.yaml:/root/starry/config.yaml:ro" \
  -v "$fixture_root/secrets/ca.pem:/usr/local/share/ca-certificates/matrix.crt:ro" \
  "$starry_image" -lc \
  'update-ca-certificates >/dev/null; exec hbbs --starry-config=/root/starry/config.yaml' >/dev/null
created_containers+=("$hbbs_name")
for _ in $(seq 1 80); do
  test -s "$fixture_root/starry-data/id_ed25519.pub" && break
  if [[ $(docker inspect --format '{{.State.Running}}' "$hbbs_name") != true ]]; then
    docker logs "$hbbs_name" >&2
    exit 1
  fi
  sleep 0.25
done
test -s "$fixture_root/starry-data/id_ed25519.pub"

docker run -d --name "$hbbr_name" --network "$fixture_network" \
  --network-alias hbbr.local -v "$fixture_root/starry-data:/root" \
  "$starry_image" hbbr -p 21117 >/dev/null
created_containers+=("$hbbr_name")
sleep 8
for name in "$hbbs_name" "$hbbr_name" "$hbbs_tls_name" "$hbbr_tls_name"; do
  if [[ $(docker inspect --format '{{.State.Running}}' "$name") != true ]]; then
    docker logs "$name" >&2
    exit 1
  fi
done

# hbbs performs its first Relay health probe during startup. The initial hbbs
# instance above exists only to generate the shared server key; restart it once
# hbbr and the WSS proxy are ready so the official health gate sees an eligible
# Relay before any browser session is attempted.
remove_container "$hbbs_name"
docker run -d --name "$hbbs_name" --network "$fixture_network" \
  --network-alias hbbs.local --entrypoint sh \
  -v "$fixture_root/starry-data:/root" \
  -v "$fixture_root/secrets:/test-secrets" \
  -v "$fixture_root/starry-config.yaml:/root/starry/config.yaml:ro" \
  -v "$fixture_root/secrets/ca.pem:/usr/local/share/ca-certificates/matrix.crt:ro" \
  "$starry_image" -lc \
  'update-ca-certificates >/dev/null; exec hbbs --starry-config=/root/starry/config.yaml' >/dev/null
created_containers+=("$hbbs_name")
sleep 3
if [[ $(docker inspect --format '{{.State.Running}}' "$hbbs_name") != true ]]; then
  docker logs "$hbbs_name" >&2
  exit 1
fi

server_key=$(tr -d '\r\n' < "$fixture_root/starry-data/id_ed25519.pub")
cp "$repo_root/examples/config.docker-builtin.yaml" "$fixture_root/config.yaml"
sed -i \
  -e "s|public-origin: \"https://client.example.com\"|public-origin: \"${client_origin}\"|" \
  -e "s|api-origin: \"https://api.example.com\"|api-origin: \"${api_origin}\"|" \
  -e "s|rendezvous-wss-url: \"wss://rustdesk.example.com/ws/id\"|rendezvous-wss-url: \"${hbbs_wss}\"|" \
  -e 's|"rustdesk.example.com:21117":|"hbbr.local:21117":|' \
  -e "s|\"wss://rustdesk.example.com/ws/relay\"|\"${hbbr_wss}\"|" \
  -e "s|server-public-key: \"REPLACE_WITH_BASE64_32_BYTE_ED25519_PUBLIC_KEY\"|server-public-key: \"${server_key}\"|" \
  -e 's|connection-token-ttl: 15m|connection-token-ttl: 5m|' \
  -e "s|issuer: \"https://api.example.com\"|issuer: \"${api_origin}\"|" \
  -e '/    listen: "127.0.0.1:21121"/a\
    server-cert-file: "/test-secrets/kessoku-server.pem"\
    server-key-file: "/test-secrets/kessoku-server-key.pem"\
    client-ca-file: "/test-secrets/ca.pem"\
    allowed-uri-sans:\
      - "spiffe://example.com/starry/production"' \
  "$fixture_root/config.yaml"
chmod 0644 "$fixture_root/config.yaml"

remove_container "$kessoku_name"
start_kessoku

printf "id = '900000101'\npassword = ''\nsalt = ''\nkey_pair = [[], []]\nkey_confirmed = false\n\n[keys_confirmed]\n" \
  > "$fixture_root/target/RustDesk.toml"
printf "rendezvous_server = ''\nnat_type = 0\nserial = 0\nunlock_pin = ''\ntrusted_devices = ''\n\n[options]\ncustom-rendezvous-server = 'hbbs.local:21116'\nrelay-server = 'hbbr.local:21117'\napi-server = '%s'\nkey = '%s'\nallow-websocket = 'N'\ndisable-udp = 'N'\nforce-always-relay = 'Y'\nenable-hwcodec = 'N'\n" \
  "$api_origin" "$server_key" > "$fixture_root/target/RustDesk2.toml"
printf '[options]\n' > "$fixture_root/target/RustDesk_local.toml"
chmod 0666 "$fixture_root/target"/*.toml
docker run -d --name "$target_bootstrap_name" --init --user 0 \
  --network "$fixture_network" \
  -v "$fixture_root/target:/home/client/.config/rustdesk" \
  -v "$fixture_root/secrets/ca.pem:/usr/local/share/ca-certificates/matrix.crt:ro" \
  "$target_test_image" sh -lc \
  "update-ca-certificates >/dev/null; mkdir -p /tmp/runtime-root; exec env HOME=/home/client XDG_CONFIG_HOME=/home/client/.config XDG_RUNTIME_DIR=/tmp/runtime-root DISPLAY=:99 dbus-run-session -- sh -lc 'Xvfb :99 -screen 0 1280x800x24 >/tmp/xvfb-bootstrap.log 2>&1 & sleep 0.8; exec /usr/bin/rustdesk --server'" >/dev/null
created_containers+=("$target_bootstrap_name")
sleep 8
test "$(docker inspect --format '{{.State.Running}}' "$target_bootstrap_name")" = true
actual_id=$(docker exec "$target_bootstrap_name" env \
  HOME=/home/client XDG_CONFIG_HOME=/home/client/.config \
  XDG_RUNTIME_DIR=/tmp/runtime-root DISPLAY=:99 \
  /usr/bin/rustdesk --get-id | tail -1)
test "$actual_id" = 900000101
password_result=$(docker exec "$target_bootstrap_name" env \
  HOME=/home/client XDG_CONFIG_HOME=/home/client/.config \
  XDG_RUNTIME_DIR=/tmp/runtime-root DISPLAY=:99 \
  /usr/bin/rustdesk --password MatrixTarget-42)
test "$password_result" = 'Done!'
remove_container "$target_bootstrap_name"
docker run --rm --user 0 -v "$fixture_root/target:/home/client/.config/rustdesk" \
  "$target_test_image" chown -R client:client /home/client/.config/rustdesk
grep -E '^password = ' "$fixture_root/target/RustDesk.toml" | grep -Fvx "password = ''" >/dev/null
grep -E '^salt = ' "$fixture_root/target/RustDesk.toml" | grep -Fvx "salt = ''" >/dev/null

docker run -d --name "$target_name" --init --user 0 \
  --network "$fixture_network" \
  -v "$fixture_root/target:/home/client/.config/rustdesk" \
  -v "$fixture_root/secrets/ca.pem:/usr/local/share/ca-certificates/matrix.crt:ro" \
  "$target_test_image" sh -lc \
  "update-ca-certificates >/dev/null; mkdir -p /tmp/runtime-client; chown client:client /tmp/runtime-client; exec runuser -u client -- env HOME=/home/client XDG_CONFIG_HOME=/home/client/.config XDG_DATA_HOME=/home/client/.local/share XDG_RUNTIME_DIR=/tmp/runtime-client DISPLAY=:99 dbus-run-session -- sh -lc 'Xvfb :99 -screen 0 1280x800x24 >/tmp/xvfb.log 2>&1 & sleep 0.8; (xev -event keyboard >/tmp/xev.log 2>&1 & for i in \$(seq 1 50); do window=\$(xdotool search --name \"Event Tester\" 2>/dev/null | head -n1); if test -n \"\$window\"; then xdotool windowfocus \"\$window\"; break; fi; sleep 0.1; done) & exec /usr/bin/rustdesk --server'" >/dev/null
created_containers+=("$target_name")
sleep 10
test "$(docker inspect --format '{{.State.Running}}' "$target_name")" = true
actual_id=$(docker exec "$target_name" runuser -u client -- env \
  HOME=/home/client XDG_CONFIG_HOME=/home/client/.config \
  XDG_RUNTIME_DIR=/tmp/runtime-client DISPLAY=:99 \
  /usr/bin/rustdesk --get-id | tail -1)
test "$actual_id" = 900000101
docker exec "$target_name" runuser -u client -- env DISPLAY=:99 \
  xdotool search --name 'Event Tester' >/dev/null
test -s "$fixture_root/target/RustDesk.toml"

for _ in $(seq 1 80); do
  if curl -fsS --cacert "$fixture_root/secrets/ca.pem" \
    "${client_origin}/config/v1.json" >/dev/null 2>&1; then
    break
  fi
  sleep 0.25
done
profile=$(curl -fsS --cacert "$fixture_root/secrets/ca.pem" \
  "${client_origin}/config/v1.json")
printf '%s\n' "$profile" | jq -e \
  --arg api "$api_origin" --arg hbbs "$hbbs_wss" --arg hbbr "$hbbr_wss" \
  '.schema_version == 1 and .profile_generation == 1 and .api_origin == $api and .rendezvous_wss_url == $hbbs and .relay_wss_urls["hbbr.local:21117"] == $hbbr' >/dev/null

cat <<EOF
web_client_fixture=READY
fixture_root=${fixture_root}
client_url=${client_origin}/
api_url=${api_origin}/
account_username=admin
account_password=MatrixAdmin-42!
target_id=900000101
target_password=MatrixTarget-42
target_container=${target_name}
starry_digest=${starry_digest}
stop_file=${fixture_root}/stop
EOF

while [[ ! -e "$fixture_root/stop" ]]; do
  sleep 1
done

printf 'web_client_fixture=STOPPED\n'
