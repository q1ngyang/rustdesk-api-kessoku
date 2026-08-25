#!/usr/bin/env bash

set -Eeuo pipefail
umask 077

if [[ $# -ne 0 ]]; then
  echo "usage: STARRY_REPO=/path/to/rustdesk-server-starry $0" >&2
  exit 64
fi

repo_root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
starry_repo=${STARRY_REPO:-}
kessoku_image=${KESSOKU_MATRIX_IMAGE:-kessoku-v3.0.0-local-matrix:current}
starry_image=ghcr.io/q1ngyang/rustdesk-server-starry:1.1.16-patch-v1.2.0
starry_digest=sha256:3685543aee6e60c27bed5db1df2fa32af83e61a58e9bc4c0ea3464664863811b
client_image=starry-release-client:rustdesk-1.4.9-qa3
tls_proxy_image=starry-release-tls-proxy:socat
go_image=golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36

if [[ -z "$starry_repo" ]] || \
  ! git -C "$starry_repo" rev-parse --is-inside-work-tree 2>/dev/null \
    | grep -qx true; then
  echo "STARRY_REPO must name the local rustdesk-server-starry checkout" >&2
  exit 64
fi
audit_fixture="$starry_repo/scripts/fixtures/release-drill-audit.yaml"
enforce_fixture="$starry_repo/scripts/fixtures/release-drill-enforce.yaml"
test -s "$audit_fixture"
test -s "$enforce_fixture"
test -s "$repo_root/conf/config.yaml"

docker image inspect "$kessoku_image" >/dev/null
docker image inspect "$client_image" >/dev/null
docker image inspect "$tls_proxy_image" >/dev/null
repo_digests=$(docker image inspect "$starry_image" --format '{{join .RepoDigests "\n"}}')
printf '%s\n' "$repo_digests" \
  | grep -Fx "ghcr.io/q1ngyang/rustdesk-server-starry@${starry_digest}" >/dev/null

official_hashes=$(docker run --rm "$starry_image" \
  sha256sum /usr/bin/hbbs /usr/bin/hbbr /usr/bin/starry-control-agent)
printf '%s\n' "$official_hashes" \
  | grep -Fx 'a415d24ef42a3bf1b78ddacf07bd65931c7f18d6096181ce368adf994ff69c66  /usr/bin/hbbs' >/dev/null
printf '%s\n' "$official_hashes" \
  | grep -Fx '0e44526134b4e836b9b4c83f470af40829d90efa85b4c91537f996078df21f87  /usr/bin/hbbr' >/dev/null
printf '%s\n' "$official_hashes" \
  | grep -Fx 'd26c89c4b1203d7111491e1acdbaff1bdcbeb9f87a782927db8454a3482844b8  /usr/bin/starry-control-agent' >/dev/null

matrix_root=$(mktemp -d /tmp/kessoku-starry-official-matrix.XXXXXX)
case "$matrix_root" in
  /tmp/kessoku-starry-official-matrix.*) ;;
  *) echo "unsafe temporary path" >&2; exit 70 ;;
esac
matrix_id=${matrix_root##*.}
current_uid=$(id -u)
current_gid=$(id -g)
matrix_network="kessoku-starry-matrix-${matrix_id}"
kessoku_name="matrix-kessoku-${matrix_id}"
kessoku_tls_name="matrix-kessoku-tls-${matrix_id}"
hbbs_name="matrix-hbbs-${matrix_id}"
hbbs_tls_name="matrix-hbbs-tls-${matrix_id}"
hbbr_name="matrix-hbbr-${matrix_id}"
hbbr_tls_name="matrix-hbbr-tls-${matrix_id}"
native_target_name="matrix-native-target-${matrix_id}"
wss_target_name="matrix-wss-target-${matrix_id}"
created_containers=()

cleanup() {
  if ((${#created_containers[@]})); then
    docker rm -f "${created_containers[@]}" >/dev/null 2>&1 || true
  fi
  mapfile -t orphan_containers < <(
    docker ps -aq --filter "name=${matrix_id}"
  )
  if ((${#orphan_containers[@]})); then
    docker rm -f "${orphan_containers[@]}" >/dev/null 2>&1 || true
  fi
  docker network rm "$matrix_network" >/dev/null 2>&1 || true
  if [[ -d "$matrix_root" ]]; then
    docker run --rm -v "$matrix_root:/matrix" \
      debian:bookworm-slim@sha256:abd67ffcfa541b485a3dff59865ab629aa048a6c613e639d36e7456b0b229241 \
      chmod -R a+rwX /matrix >/dev/null 2>&1 || true
    chmod -R u+w -- "$matrix_root" 2>/dev/null || true
    rm -r -- "$matrix_root"
  fi
}
trap 'printf "official_matrix_failure line=%s status=%s\n" "$LINENO" "$?" >&2' ERR
trap cleanup EXIT HUP INT TERM

mkdir -p "$matrix_root/kessoku-data" "$matrix_root/kessoku-runtime" \
  "$matrix_root/starry-data/starry" \
  "$matrix_root/secrets" "$matrix_root/screens" \
  "$matrix_root/target-native" "$matrix_root/target-wss"
chmod 0777 "$matrix_root/kessoku-data" "$matrix_root/kessoku-runtime" \
  "$matrix_root/starry-data" \
  "$matrix_root/starry-data/starry" "$matrix_root/screens" \
  "$matrix_root/target-native" "$matrix_root/target-wss"

openssl genpkey -algorithm ED25519 \
  -out "$matrix_root/secrets/kessoku-access.pem"
openssl req -x509 -newkey rsa:2048 -sha256 -days 2 -nodes \
  -subj '/CN=Kessoku Starry local matrix CA' \
  -keyout "$matrix_root/secrets/ca-key.pem" \
  -out "$matrix_root/secrets/ca.pem" >/dev/null 2>&1
openssl req -newkey rsa:2048 -nodes -subj '/CN=kessoku.local' \
  -addext 'subjectAltName=DNS:kessoku.local' \
  -keyout "$matrix_root/secrets/kessoku-server-key.pem" \
  -out "$matrix_root/secrets/kessoku-server.csr" >/dev/null 2>&1
openssl x509 -req -in "$matrix_root/secrets/kessoku-server.csr" \
  -CA "$matrix_root/secrets/ca.pem" \
  -CAkey "$matrix_root/secrets/ca-key.pem" -CAcreateserial -days 2 \
  -sha256 -copy_extensions copy \
  -out "$matrix_root/secrets/kessoku-server.pem" >/dev/null 2>&1
openssl req -newkey rsa:2048 -nodes -subj '/CN=starry-hbbs' \
  -addext 'subjectAltName=URI:spiffe://example.com/starry/production' \
  -keyout "$matrix_root/secrets/client-key.pem" \
  -out "$matrix_root/secrets/client.csr" >/dev/null 2>&1
openssl x509 -req -in "$matrix_root/secrets/client.csr" \
  -CA "$matrix_root/secrets/ca.pem" \
  -CAkey "$matrix_root/secrets/ca-key.pem" -CAcreateserial -days 2 \
  -sha256 -copy_extensions copy \
  -out "$matrix_root/secrets/client.pem" >/dev/null 2>&1
openssl req -newkey rsa:2048 -nodes -subj '/CN=starry-wss.local' \
  -addext 'subjectAltName=DNS:hbbs.local,DNS:hbbr.local' \
  -keyout "$matrix_root/secrets/wss-key.pem" \
  -out "$matrix_root/secrets/wss.csr" >/dev/null 2>&1
openssl x509 -req -in "$matrix_root/secrets/wss.csr" \
  -CA "$matrix_root/secrets/ca.pem" \
  -CAkey "$matrix_root/secrets/ca-key.pem" -CAcreateserial -days 2 \
  -sha256 -copy_extensions copy \
  -out "$matrix_root/secrets/wss.pem" >/dev/null 2>&1
openssl rand -base64 24 > "$matrix_root/secrets/admin-password"
find "$matrix_root/secrets" -type f -exec chmod 0600 {} +

docker network create "$matrix_network" >/dev/null

kessoku_env_args=(
  -e RUSTDESK_API_AUTH_ENABLED=true
  -e RUSTDESK_API_AUTH_ISSUER=https://kessoku.local
  -e 'RUSTDESK_API_AUTH_AUDIENCES=kessoku-api,rustdesk-connect'
  -e RUSTDESK_API_AUTH_ACCESS_TOKEN_TTL=10m
  -e RUSTDESK_API_AUTH_MAXIMUM_TOKEN_TTL=10m
  -e RUSTDESK_API_AUTH_CURRENT_KEY_ID=matrix-current
  -e RUSTDESK_API_AUTH_CURRENT_KEY_PRIVATE_KEY_FILE=/test-secrets/kessoku-access.pem
  -e RUSTDESK_API_AUTH_INTERNAL_ENABLED=true
  -e RUSTDESK_API_AUTH_INTERNAL_LISTEN=0.0.0.0:21121
  -e RUSTDESK_API_AUTH_INTERNAL_SERVER_CERT_FILE=/test-secrets/kessoku-server.pem
  -e RUSTDESK_API_AUTH_INTERNAL_SERVER_KEY_FILE=/test-secrets/kessoku-server-key.pem
  -e RUSTDESK_API_AUTH_INTERNAL_CLIENT_CA_FILE=/test-secrets/ca.pem
  -e RUSTDESK_API_AUTH_INTERNAL_ALLOWED_URI_SANS=spiffe://example.com/starry/production
  -e RUSTDESK_API_RUSTDESK_API_SERVER=https://kessoku.local
)
kessoku_args=(
  --name "$kessoku_name"
  --user "${current_uid}:${current_gid}"
  --network "$matrix_network"
  --network-alias kessoku.local
  -v "$matrix_root/kessoku-data:/app/data"
  -v "$matrix_root/kessoku-runtime:/app/runtime"
  -v "$matrix_root/secrets:/test-secrets:ro"
  "${kessoku_env_args[@]}"
)

start_kessoku() {
  docker run -d "${kessoku_args[@]}" "$kessoku_image" >/dev/null
  created_containers+=("$kessoku_name")
  for _ in $(seq 1 60); do
    if docker run --rm --network "$matrix_network" "$client_image" \
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

start_kessoku
docker rm -f "$kessoku_name" >/dev/null
created_containers=()
docker run --rm --network "$matrix_network" \
  --user "${current_uid}:${current_gid}" \
  -v "$matrix_root/kessoku-data:/app/data" \
  -v "$matrix_root/kessoku-runtime:/app/runtime" \
  -v "$matrix_root/secrets:/test-secrets:ro" \
  "${kessoku_env_args[@]}" \
  "$kessoku_image" ./kessoku-api reset-admin-pwd \
  --password-file /test-secrets/admin-password >/dev/null
start_kessoku

docker run -d --name "$kessoku_tls_name" \
  --network "container:$kessoku_name" \
  -v "$matrix_root/secrets:/test-secrets:ro" "$tls_proxy_image" \
  OPENSSL-LISTEN:443,reuseaddr,fork,cert=/test-secrets/kessoku-server.pem,key=/test-secrets/kessoku-server-key.pem,verify=0 \
  TCP:127.0.0.1:21114 >/dev/null
created_containers+=("$kessoku_tls_name")

docker run --rm --user 0 --network "$matrix_network" \
  -v "$matrix_root/secrets/ca.pem:/usr/local/share/ca-certificates/matrix.crt:ro" \
  "$client_image" sh -lc \
  'update-ca-certificates >/dev/null; curl -fsS https://kessoku.local/api/version >/dev/null'
: > "$matrix_root/secrets/jwks.json"
chmod 0600 "$matrix_root/secrets/jwks.json"
docker run --rm --user "${current_uid}:${current_gid}" \
  --network "$matrix_network" \
  -v "$matrix_root/secrets:/test-secrets" "$client_image" sh -lc \
  'curl -fsS --cacert /test-secrets/ca.pem --cert /test-secrets/client.pem --key /test-secrets/client-key.pem https://kessoku.local:21121/api/internal/v1/auth/jwks -o /test-secrets/jwks.json'

start_starry() {
  local fixture=$1
  docker run -d --name "$hbbs_name" --network "$matrix_network" \
    --network-alias hbbs.local --entrypoint sh \
    -v "$matrix_root/starry-data:/root" \
    -v "$matrix_root/secrets:/test-secrets" \
    -v "$fixture:/root/starry/config.yaml:ro" \
    -v "$matrix_root/secrets/ca.pem:/usr/local/share/ca-certificates/matrix.crt:ro" \
    "$starry_image" -lc \
    'update-ca-certificates >/dev/null; exec hbbs --starry-config=/root/starry/config.yaml' >/dev/null
  created_containers+=("$hbbs_name")
  for _ in $(seq 1 60); do
    test -s "$matrix_root/starry-data/id_ed25519.pub" && break
    if [[ $(docker inspect --format '{{.State.Running}}' "$hbbs_name") != true ]]; then
      docker logs "$hbbs_name" >&2
      return 1
    fi
    sleep 0.25
  done
  test -s "$matrix_root/starry-data/id_ed25519.pub"
  if [[ $(docker inspect --format '{{.State.Running}}' "$hbbs_name") != true ]]; then
    docker logs "$hbbs_name" >&2
    return 1
  fi

  if ! docker container inspect "$hbbr_name" >/dev/null 2>&1; then
    docker run -d --name "$hbbr_name" --network "$matrix_network" \
      --network-alias hbbr.local -v "$matrix_root/starry-data:/root" \
      "$starry_image" hbbr -p 21117 >/dev/null
    created_containers+=("$hbbr_name")
    docker run -d --name "$hbbr_tls_name" \
      --network "container:$hbbr_name" \
      -v "$matrix_root/secrets:/test-secrets:ro" "$tls_proxy_image" \
      OPENSSL-LISTEN:443,reuseaddr,fork,cert=/test-secrets/wss.pem,key=/test-secrets/wss-key.pem,verify=0 \
      TCP:127.0.0.1:21119 >/dev/null
    created_containers+=("$hbbr_tls_name")
  fi
  docker run -d --name "$hbbs_tls_name" \
    --network "container:$hbbs_name" \
    -v "$matrix_root/secrets:/test-secrets:ro" "$tls_proxy_image" \
    OPENSSL-LISTEN:443,reuseaddr,fork,cert=/test-secrets/wss.pem,key=/test-secrets/wss-key.pem,verify=0 \
    TCP:127.0.0.1:21118 >/dev/null
  created_containers+=("$hbbs_tls_name")
  sleep 8
  for name in "$hbbs_name" "$hbbs_tls_name" "$hbbr_name" "$hbbr_tls_name"; do
    if [[ $(docker inspect --format '{{.State.Running}}' "$name") != true ]]; then
      docker logs "$name" >&2
      return 1
    fi
  done
  if docker logs "$hbbs_name" 2>&1 | grep -q 'Initial JWKS refresh retained'; then
    docker logs "$hbbs_name" >&2
    return 1
  fi
}

stop_hbbs() {
  docker rm -f "$hbbs_tls_name" "$hbbs_name" >/dev/null
  local kept=()
  for name in "${created_containers[@]}"; do
    if [[ "$name" != "$hbbs_tls_name" && "$name" != "$hbbs_name" ]]; then
      kept+=("$name")
    fi
  done
  created_containers=("${kept[@]}")
}

write_profile() {
  local directory=$1
  local id=$2
  local transport=$3
  local token=${4:-}
  local server_key
  local allow_websocket=N
  local disable_udp=N
  server_key=$(tr -d '\r\n' < "$matrix_root/starry-data/id_ed25519.pub")
  if [[ "$transport" == wss ]]; then
    allow_websocket=Y
    disable_udp=Y
  fi
  printf "id = '%s'\npassword = ''\nsalt = ''\nkey_pair = [[], []]\nkey_confirmed = false\n\n[keys_confirmed]\n" \
    "$id" > "$directory/RustDesk.toml"
  printf "rendezvous_server = ''\nnat_type = 0\nserial = 0\nunlock_pin = ''\ntrusted_devices = ''\n\n[options]\ncustom-rendezvous-server = 'hbbs.local:21116'\nrelay-server = 'hbbr.local:21117'\napi-server = 'https://kessoku.local'\nkey = '%s'\nallow-websocket = '%s'\ndisable-udp = '%s'\nforce-always-relay = 'Y'\nenable-hwcodec = 'N'\n" \
    "$server_key" "$allow_websocket" "$disable_udp" \
    > "$directory/RustDesk2.toml"
  if [[ -n "$token" ]]; then
    printf "[options]\naccess_token = '%s'\nuser_info = '{\"name\":\"admin\"}'\n" \
      "$token" > "$directory/RustDesk_local.toml"
  else
    printf '[options]\n' > "$directory/RustDesk_local.toml"
  fi
  chmod 0666 "$directory"/*.toml
}

start_target() {
  local name=$1
  local directory=$2
  local id=$3
  local transport=$4
  local transport_args=()
  local network_setup=:
  if [[ "$transport" == wss ]]; then
    transport_args=(--cap-add NET_ADMIN)
    network_setup='tc qdisc add dev eth0 root netem delay 12ms'
  fi
  write_profile "$directory" "$id" "$transport"
  docker run -d --name "$name" --init --user 0 \
    "${transport_args[@]}" \
    --network "$matrix_network" \
    -v "$directory:/home/client/.config/rustdesk" \
    -v "$matrix_root/secrets/ca.pem:/usr/local/share/ca-certificates/matrix.crt:ro" \
    "$client_image" sh -lc \
    "update-ca-certificates >/dev/null; ${network_setup}; mkdir -p /tmp/runtime-client; chown client:client /tmp/runtime-client; exec runuser -u client -- env HOME=/home/client XDG_CONFIG_HOME=/home/client/.config XDG_RUNTIME_DIR=/tmp/runtime-client DISPLAY=:99 dbus-run-session -- sh -lc 'Xvfb :99 -screen 0 1280x800x24 >/tmp/xvfb.log 2>&1 & sleep 0.8; xmessage -geometry 900x500+180+120 -center KESSOKU_STARRY_OFFICIAL_MATRIX >/tmp/xmessage.log 2>&1 & exec /usr/bin/rustdesk --server'" >/dev/null
  created_containers+=("$name")
  sleep 10
  if [[ $(docker inspect --format '{{.State.Running}}' "$name") != true ]]; then
    docker logs "$name" >&2
    return 1
  fi
  local actual_id
  actual_id=$(docker exec "$name" runuser -u client -- env \
    HOME=/home/client XDG_CONFIG_HOME=/home/client/.config \
    XDG_RUNTIME_DIR=/tmp/runtime-client DISPLAY=:99 \
    /usr/bin/rustdesk --get-id | tail -1)
  if [[ "$actual_id" != "$id" ]]; then
    printf 'target ID mismatch: expected=%s actual=%s\n' "$id" "$actual_id" >&2
    docker logs "$name" >&2
    return 1
  fi
  docker exec "$name" runuser -u client -- env \
    HOME=/home/client XDG_CONFIG_HOME=/home/client/.config \
    XDG_RUNTIME_DIR=/tmp/runtime-client DISPLAY=:99 \
    /usr/bin/rustdesk --password MatrixTarget-42 >/dev/null 2>&1
  if [[ "$transport" == wss ]]; then
    if ! docker exec "$name" sh -lc \
      "grep -Eq 'start (tcp: |websocket: )?wss://hbbs.local/ws/id' /home/client/.local/share/logs/RustDesk/server/rustdesk_rCURRENT.log"; then
      docker exec "$name" sh -lc \
        'tail -n 120 /home/client/.local/share/logs/RustDesk/server/rustdesk_rCURRENT.log' >&2 || true
      return 1
    fi
  fi
}

login_token() {
  local controller_id=$1
  local password
  local payload
  password=$(<"$matrix_root/secrets/admin-password")
  payload=$(jq -nc --arg password "$password" --arg id "$controller_id" \
    --arg uuid "matrix-${controller_id}" \
    '{username:"admin", password:$password, id:$id, uuid:$uuid, deviceInfo:{name:"official-matrix", os:"Linux", type:"Linux"}}')
  docker run --rm --network "$matrix_network" "$client_image" \
    curl -fsS -H 'Content-Type: application/json' --data "$payload" \
    http://kessoku.local:21114/api/login \
    | jq -er .access_token
}

run_case() {
  local label=$1
  local controller_transport=$2
  local target_id=$3
  local controller_id=$4
  local directory="$matrix_root/controller-${label}"
  local name="matrix-${label}-${matrix_id}"
  local token
  local transport_args=()
  local network_setup=:
  if [[ "$controller_transport" == wss ]]; then
    transport_args=(--cap-add NET_ADMIN)
    network_setup='tc qdisc add dev eth0 root netem delay 12ms'
  fi
  mkdir -p "$directory"
  chmod 0777 "$directory"
  token=$(login_token "$controller_id")
  write_profile "$directory" "$controller_id" "$controller_transport" "$token"
  docker run -d --name "$name" --init --user 0 \
    "${transport_args[@]}" \
    --network "$matrix_network" \
    -v "$directory:/home/client/.config/rustdesk" \
    -v "$matrix_root/secrets/ca.pem:/usr/local/share/ca-certificates/matrix.crt:ro" \
    "$client_image" sh -lc \
    "update-ca-certificates >/dev/null; ${network_setup}; mkdir -p /tmp/runtime-client; chown client:client /tmp/runtime-client; chown -R client:client /home/client/.config/rustdesk; exec runuser -u client -- env HOME=/home/client XDG_CONFIG_HOME=/home/client/.config XDG_RUNTIME_DIR=/tmp/runtime-client DISPLAY=:99 dbus-run-session -- sh -lc 'Xvfb :99 -screen 0 1280x800x24 >/tmp/xvfb.log 2>&1 & sleep 0.8; exec /usr/bin/rustdesk --connect ${target_id} --password MatrixTarget-42 --relay'" >/dev/null
  created_containers+=("$name")
  connected=0
  for _ in $(seq 1 45); do
    if docker exec "$name" sh -lc \
      "runuser -u client -- env DISPLAY=:99 xdotool search --name '^${target_id} - Remote Desktop - RustDesk$' >/dev/null 2>&1"; then
      connected=1
      break
    fi
    if [[ $(docker inspect --format '{{.State.Running}}' "$name") != true ]]; then
      break
    fi
    sleep 1
  done
  if [[ $connected -ne 1 ]]; then
    docker logs "$name" >&2
    docker logs "$hbbs_name" >&2
    docker logs "$hbbr_name" >&2
    return 1
  fi
  docker exec "$name" sh -lc \
    "runuser -u client -- env DISPLAY=:99 import -window root /tmp/${label}.png"
  docker cp "$name:/tmp/${label}.png" "$matrix_root/screens/${label}.png" >/dev/null
  docker run --rm -v "$matrix_root/screens:/screens:ro" "$client_image" \
    identify "/screens/${label}.png" >/dev/null
  docker exec "$hbbr_name" sh -lc \
    "awk '\$2 ~ /:(527D|527F)$/ && \$4 == \"01\" { found=1 } END { exit !found }' /proc/net/tcp /proc/net/tcp6"
  printf 'client_case=%s controller_transport=%s target=%s result=PASS\n' \
    "$label" "$controller_transport" "$target_id"
  docker rm -f "$name" >/dev/null
  local kept=()
  for item in "${created_containers[@]}"; do
    [[ "$item" == "$name" ]] || kept+=("$item")
  done
  created_containers=("${kept[@]}")
}

start_starry "$audit_fixture"
printf 'phase=audit official_starry=ready\n'
start_target "$native_target_name" "$matrix_root/target-native" 900000101 native
printf 'target=native id=900000101 ready\n'
start_target "$wss_target_name" "$matrix_root/target-wss" 900000102 wss
printf 'target=wss id=900000102 ready\n'
run_case audit-native-native native 900000101 900000201

stop_hbbs
start_starry "$enforce_fixture"
sleep 6
printf 'phase=enforce official_starry=ready\n'
run_case enforce-native-native native 900000101 900000202
run_case enforce-wss-wss wss 900000102 900000203
run_case enforce-wss-native wss 900000101 900000204
run_case enforce-native-wss native 900000102 900000205

test "$(docker image inspect "$starry_image" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}')" \
  = 5e73b3af1423acf5ee20ca32a2d747eef6df3494
docker run --rm -v "$repo_root:/src:ro" -w /src \
  -v kessoku-go-mod:/go/pkg/mod -v kessoku-go-build:/root/.cache/go-build \
  "$go_image" go test ./internal/starrycontrol/starry >/dev/null

printf 'official_starry_digest=%s\n' "$starry_digest"
printf 'official_hbbs_sha256=%s\n' a415d24ef42a3bf1b78ddacf07bd65931c7f18d6096181ce368adf994ff69c66
printf 'official_hbbr_sha256=%s\n' 0e44526134b4e836b9b4c83f470af40829d90efa85b4c91537f996078df21f87
printf 'audit_to_enforce_and_four_client_paths=PASS\n'
