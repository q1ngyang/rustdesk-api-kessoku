#!/usr/bin/env bash

set -Eeuo pipefail

if [[ $# -ne 1 ]] || [[ ! $1 =~ ^web-target-[A-Za-z0-9_-]+$ ]]; then
  echo "usage: $0 WEB_TARGET_CONTAINER" >&2
  exit 64
fi

target_container=$1
test "$(docker inspect --format '{{.State.Running}}' "$target_container")" = true

mouse_state=$(docker exec "$target_container" runuser -u client -- env DISPLAY=:99 \
  xdotool getmouselocation --shell)
grep -Fx 'X=320' <<<"$mouse_state" >/dev/null
grep -Fx 'Y=240' <<<"$mouse_state" >/dev/null

key_log=$(docker exec "$target_container" cat /tmp/xev.log)
grep -F 'keysym 0x4b, K' <<<"$key_log" >/dev/null
grep -F 'keysym 0xffe3, Control_L' <<<"$key_log" >/dev/null
awk '
  /^KeyPress event/ { in_press = 1; block = $0 "\n"; next }
  in_press { block = block $0 "\n" }
  in_press && /^$/ {
    if (block ~ /state 0x4,/ && block ~ /keysym 0x73, s/) found_control_s = 1
    in_press = 0
  }
  END { exit(found_control_s ? 0 : 1) }
' <<<"$key_log"

printf 'web_client_target_input=PASS mouse=320,240 keyboard=K,Ctrl+S\n'
