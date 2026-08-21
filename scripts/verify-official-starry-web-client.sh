#!/usr/bin/env bash

set -Eeuo pipefail
umask 077

if [[ $# -ne 0 ]]; then
  echo "usage: STARRY_REPO=/path/to/rustdesk-server-starry $0" >&2
  exit 64
fi

repo_root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
starry_repo=${STARRY_REPO:-}
browser_image=mcr.microsoft.com/playwright@sha256:dcc5531e97840b9b5e794f2814476b21571c5124a3fca2267d73041f56e7580e
node_image=node@sha256:f22d6a1f082c02f292e86929b5b0442ac2e5eaf438a5dea9b1566601c3e05940
devtools_port=49222
browser_name="kessoku-playwright-browser-$$"
fixture_log=$(mktemp /tmp/kessoku-web-browser-fixture.XXXXXX)
fixture_pid=
stop_file=

cleanup() {
  if [[ -n "$stop_file" && "$stop_file" == /tmp/kessoku-starry-web-client.*/stop ]]; then
    touch "$stop_file" 2>/dev/null || true
  fi
  if docker container inspect "$browser_name" >/dev/null 2>&1; then
    docker stop "$browser_name" >/dev/null 2>&1 || true
  fi
  if [[ -n "$fixture_pid" ]]; then
    wait "$fixture_pid" 2>/dev/null || true
  fi
  if [[ -f "$fixture_log" ]]; then
    find "$fixture_log" -delete
  fi
}
trap cleanup EXIT HUP INT TERM

if [[ -z "$starry_repo" ]] || \
  ! git -C "$starry_repo" rev-parse --is-inside-work-tree 2>/dev/null \
    | grep -qx true; then
  echo "STARRY_REPO must name the local rustdesk-server-starry checkout" >&2
  exit 64
fi
if curl --fail --silent --max-time 1 \
  "http://127.0.0.1:${devtools_port}/json/version" >/dev/null 2>&1; then
  echo "DevTools port ${devtools_port} is already in use" >&2
  exit 69
fi
docker image inspect "$browser_image" "$node_image" >/dev/null

STARRY_REPO="$starry_repo" \
  "$repo_root/scripts/run-official-starry-web-client-fixture.sh" \
  >"$fixture_log" 2>&1 &
fixture_pid=$!

for _ in $(seq 1 240); do
  if grep -Fx 'web_client_fixture=READY' "$fixture_log" >/dev/null 2>&1; then
    break
  fi
  if ! kill -0 "$fixture_pid" 2>/dev/null; then
    cat "$fixture_log" >&2
    wait "$fixture_pid"
  fi
  sleep 0.5
done
grep -Fx 'web_client_fixture=READY' "$fixture_log" >/dev/null

fixture_value() {
  local key=$1
  sed -n "s/^${key}=//p" "$fixture_log" | tail -1
}

client_url=$(fixture_value client_url)
api_url=$(fixture_value api_url)
account_username=$(fixture_value account_username)
account_password=$(fixture_value account_password)
target_id=$(fixture_value target_id)
target_password=$(fixture_value target_password)
target_container=$(fixture_value target_container)
stop_file=$(fixture_value stop_file)

[[ "$client_url" =~ ^https://127\.0\.0\.1:[0-9]+/$ ]]
[[ "$api_url" =~ ^https://127\.0\.0\.1:[0-9]+/$ ]]
[[ "$target_container" =~ ^web-target-[A-Za-z0-9_-]+$ ]]
[[ "$stop_file" == /tmp/kessoku-starry-web-client.*/stop ]]

docker run -d --rm --name "$browser_name" --network host --ipc=host \
  "$browser_image" \
  /ms-playwright/chromium-1234/chrome-linux64/chrome \
  --headless=new --disable-gpu --no-sandbox --no-proxy-server \
  --disable-background-networking --disable-extensions \
  --ignore-certificate-errors --remote-debugging-address=127.0.0.1 \
  "--remote-debugging-port=${devtools_port}" \
  --user-data-dir=/tmp/kessoku-playwright-profile about:blank >/dev/null

for _ in $(seq 1 80); do
  if curl --fail --silent --max-time 1 \
    "http://127.0.0.1:${devtools_port}/json/version" >/dev/null 2>&1; then
    break
  fi
  sleep 0.25
done
curl --fail --silent --max-time 1 \
  "http://127.0.0.1:${devtools_port}/json/version" >/dev/null

docker run --rm --network host -v "$repo_root:/src:ro" -w /src \
  "$node_image" node scripts/verify-web-client-browser.mjs \
  "$devtools_port" "$client_url" "$api_url" "$account_username" \
  "$account_password" "$target_id" "$target_password"
"$repo_root/scripts/verify-web-client-target-input.sh" "$target_container"

touch "$stop_file"
wait "$fixture_pid"
fixture_pid=
grep -Fx 'web_client_fixture=STOPPED' "$fixture_log" >/dev/null
printf 'official_starry_web_client=PASS\n'
