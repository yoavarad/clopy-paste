#!/bin/sh
# One functional round trip A->B (make smoke, ci-pipeline stage 8): a unique message sent on A must be
# visible on B within 10 s. The same message is resent every 1 s inside the window, so a consumer group
# that has not joined yet (it starts at latest, ADR-017) cannot make the check flaky.
set -eu
. "$(dirname "$0")/../e2e/lib.sh"

user="smoke-$$-$(date +%s)"
text="smoke$$x$(date +%s)"
i=0
while [ "$i" -lt 10 ]; do
  [ "$(http_status "$HOST_CLOPY_PASTE_A" POST /api/messages "{\"user\":\"$user\",\"text\":\"$text\"}")" = 202 ] || fail "POST on A not 202"
  sleep 1
  case "$(http_get "$HOST_CLOPY_PASTE_B" "/api/messages?user=$user" || true)" in
    *"$text"*) echo "smoke: round trip ok"; exit 0 ;;
  esac
  i=$((i + 1))
done
fail "message not seen on B within 10 s"
