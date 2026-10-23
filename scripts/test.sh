#!/bin/sh
# make test: go test -race -cover, then fail when a gated package is below COVER_MIN percent.
# ydk:crosscut:ops/ci-pipeline stage 2.
set -eu
cd "$(dirname "$0")/.."
. scripts/env.sh

require go awk
: "${COVER_MIN:=80}"
: "${COVER_PKGS:=internal/messaging internal/httpapi internal/config}"
set --
# -race needs cgo and a C compiler.
if [ "$(go env CGO_ENABLED)" = 1 ] && command -v "$(go env CC)" >/dev/null 2>&1; then
  set -- -race
else
  echo "test: WARNING -race skipped (needs CGO_ENABLED=1 and a C compiler such as gcc on PATH)" >&2
fi
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
# Stream output while it runs; capture go test's exit code (a pipe would hide it).
{ go test "$@" -cover -coverprofile="$tmp/cover.out" ./... 2>&1; echo $? >"$tmp/rc"; } | tee "$tmp/out.txt"
[ "$(cat "$tmp/rc")" = 0 ] || exit 1
fail=0
for pkg in $COVER_PKGS; do
  pct=$(awk -v p="/$pkg" '
    $1 == "ok" && substr($2, length($2) - length(p) + 1) == p {
      for (i = 1; i <= NF; i++) if ($i == "coverage:") { sub(/%/, "", $(i + 1)); print $(i + 1) }
    }' "$tmp/out.txt")
  if [ -z "$pct" ]; then
    echo "test: FAIL no coverage reported for $pkg" >&2
    fail=1
  elif awk -v c="$pct" -v m="$COVER_MIN" 'BEGIN { exit !(c + 0 < m + 0) }'; then
    echo "test: FAIL $pkg coverage $pct% is below $COVER_MIN%" >&2
    fail=1
  else
    echo "test: $pkg coverage $pct% (min $COVER_MIN%)"
  fi
done
[ "$fail" = 0 ] || exit 1
echo "test: ok"
