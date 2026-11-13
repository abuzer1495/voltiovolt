#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# VLT-IS-009 TASK 03-00-00-004: HTTP check of official help links.
# Always exits 0 so a down official site cannot block the merge.
set -u
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
urls=$(grep -oE 'https://(core\.telegram\.org|support\.google\.com|support\.microsoft\.com)[^"]*' internal/gui/openhelp.go | sort -u)
if [ -z "$urls" ]; then
  echo "WARNING: no official help URLs in openhelp.go"
  exit 0
fi
ok=0
fail=0
while IFS= read -r u; do
  [ -n "$u" ] || continue
  code=$(curl -sS -o /dev/null -w '%{http_code}' -L --max-time 20 -A 'VoltioVolt-link-check' "$u" || echo 000)
  echo "$code  $u"
  if [ "$code" = 200 ]; then
    ok=$((ok + 1))
  else
    fail=$((fail + 1))
  fi
done <<< "$urls"
echo "RESULT ok=$ok fail=$fail"
exit 0
