# Sourced by test/e2e/*.sh, test/smoke/*.sh and scripts/smoke.sh: curl helpers that reach each
# release through the kind ingress at http://localhost:$HOST_PORT with a per-release Host header.
: "${HOST_PORT:=8080}"
BASE_URL="http://localhost:$HOST_PORT"
: "${HOST_CLOPY_PASTE_A:=a.clopy-paste.local}"
: "${HOST_CLOPY_PASTE_B:=b.clopy-paste.local}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

# http_get <host> <path>: prints the body; non-zero on HTTP >= 400.
http_get() {
  curl -fsS -H "Host: $1" "$BASE_URL$2"
}

# http_post_json <host> <path> <json>: prints the body; non-zero on HTTP >= 400.
http_post_json() {
  curl -fsS -H "Host: $1" -H 'Content-Type: application/json' -d "$3" "$BASE_URL$2"
}

# http_status <host> <method> <path> [json]: prints only the HTTP status code.
http_status() {
  if [ "$#" -ge 4 ]; then
    curl -sS -o /dev/null -w '%{http_code}' -X "$2" -H "Host: $1" -H 'Content-Type: application/json' \
      -d "$4" "$BASE_URL$3"
  else
    curl -sS -o /dev/null -w '%{http_code}' -X "$2" -H "Host: $1" "$BASE_URL$3"
  fi
}

# poll_until <seconds> <cmd...>: retry cmd once a second until it succeeds.
poll_until() {
  limit=$1
  shift
  i=0
  while [ "$i" -lt "$limit" ]; do
    if "$@"; then
      return 0
    fi
    i=$((i + 1))
    sleep 1
  done
  return 1
}

# wait_ready <host>: GET /readyz until 2xx (up to 120 s).
wait_ready() {
  i=0
  until http_get "$1" /readyz >/dev/null; do
    i=$((i + 1))
    [ "$i" -lt 60 ] || fail "$1 not ready"
    sleep 2
  done
}
