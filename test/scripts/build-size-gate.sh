#!/bin/sh
# Dry test for scripts/build.sh: stubbed docker, no real build. The size gate must pass a small
# image, fail an oversized one, and the host binary must not be built when docker is present.
set -eu
cd "$(dirname "$0")/../.."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cat > "$tmp/docker" <<'STUB'
#!/bin/sh
case "$1" in
  buildx|image) exit 0 ;;
  save) head -c "${STUB_BYTES:-100}" /dev/zero | tr '\0' 'x' ; head -c "${STUB_BYTES:-100}" /dev/urandom ;;
esac
STUB
chmod +x "$tmp/docker"
PATH="$tmp:$PATH" STUB_BYTES=100 sh scripts/build.sh >/dev/null || { echo "FAIL: small image rejected"; exit 1; }
PATH="$tmp:$PATH" STUB_BYTES=100 sh scripts/build.sh | grep -q 'bin/' && { echo "FAIL: host binary built despite docker"; exit 1; }
PATH="$tmp:$PATH" CACHE_GHA=1 STUB_BYTES=100 sh scripts/build.sh >/dev/null || { echo "FAIL: CACHE_GHA branch"; exit 1; }
out=$(PATH="$tmp:$PATH" STUB_BYTES=2000 MAX_IMAGE_BYTES=1000 sh scripts/build.sh 2>&1) && { echo "FAIL: oversized image accepted"; exit 1; }
case "$out" in *"exceeds"*) ;; *) echo "FAIL: wrong failure: $out"; exit 1 ;; esac
echo "build-size-gate: ok"
