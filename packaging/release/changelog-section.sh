#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# changelog-section.sh VERSION [FILE]: print the section "## [VERSION]" of
# CHANGELOG.md (Keep a Changelog) without its heading, without the leading
# and trailing empty lines. Exit 1 if the section is missing or empty.
# release.yml uses it for the body of the draft release (VLT-IS-012 TASK
# 03-00-00-006 step 3.3).
set -eu
v="${1:?usage: changelog-section.sh VERSION [FILE]}"
f="${2:-CHANGELOG.md}"
out="$(awk -v h="## [$v]" '
  index($0, h) == 1 && (length($0) == length(h) || substr($0, length(h) + 1, 1) == " ") { on = 1; next }
  on && /^## / { exit }
  on { print }
' "$f" | awk 'NF { seen = 1 } seen { lines[++n] = $0 } END { while (n > 0 && lines[n] ~ /^[[:space:]]*$/) n--; for (i = 1; i <= n; i++) print lines[i] }')"
if [ -z "$out" ]; then
  echo "changelog-section.sh: no section \"## [$v]\" in $f" >&2
  exit 1
fi
printf '%s\n' "$out"
