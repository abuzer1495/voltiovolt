#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Capture VoltioVolt demo screens under Xvfb.
# Tools: xvfb (X.Org / MIT), xdotool (BSD-3-Clause), scrot (MIT). All are
# on the SEC-061 allow list. ImageMagick is not used.
set -euo pipefail

BIN=${BIN:?set BIN to the voltiovolt test binary}
OUT=${OUT:?set OUT to the output directory}
SCREENS=${SCREENS:-"$(dirname "$0")/screenshot-screens.txt"}
WAIT=${WAIT:-60}

mkdir -p "$OUT"
chmod +x "$BIN" || true

file_name() {
  local lang=$1 screen=$2
  echo "${lang}-$(echo "$screen" | tr '/ ' '--')"
}

wait_ready() {
  local f=$1 n=0
  while [[ $n -lt $WAIT ]]; do
    if [[ -f $f ]]; then
      return 0
    fi
    sleep 1
    n=$((n + 1))
  done
  echo "timeout waiting for $f" >&2
  return 1
}

find_wid() {
  local n=0
  while [[ $n -lt 20 ]]; do
    local ids
    ids=$(xdotool search --onlyvisible --name 'VoltioVolt' 2>/dev/null || true)
    if [[ -n $ids ]]; then
      echo "$ids" | awk 'NR==1{print}'
      return 0
    fi
    sleep 0.5
    n=$((n + 1))
  done
  return 1
}

capture() {
  local lang=$1 screen=$2
  local data ready pid wid name
  data=$(mktemp -d "${TMPDIR:-/tmp}/vlt-shot.XXXXXX")
  ready="$data/ready"
  name=$(file_name "$lang" "$screen")
  echo "capture $name"
  "$BIN" --data-dir "$data" --demo --lang "$lang" --demo-screen "$screen" --demo-ready-file "$ready" >"$data/app.log" 2>&1 &
  pid=$!
  if ! wait_ready "$ready"; then
    echo "---- app.log ----" >&2
    cat "$data/app.log" >&2 || true
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    return 1
  fi
  sleep 2
  if ! wid=$(find_wid); then
    echo "no window for $name" >&2
    cat "$data/app.log" >&2 || true
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    return 1
  fi
  xdotool windowactivate "$wid" || true
  xdotool windowsize "$wid" 1280 720 || true
  sleep 0.4
  scrot -u "$OUT/${name}.png"
  kill "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
  rm -rf "$data"
}

while IFS= read -r screen; do
  [[ -z $screen || $screen == \#* ]] && continue
  for lang in en tr; do
    capture "$lang" "$screen"
  done
done < "$SCREENS"

ls -l "$OUT"
