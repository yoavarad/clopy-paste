#!/bin/sh
# make load: load the locally built $IMAGE into the kind cluster (used before deploy on GitHub).
set -eu
cd "$(dirname "$0")/.."
. scripts/env.sh

require docker kind
docker image inspect "$IMAGE" >/dev/null 2>&1 || die "image $IMAGE not found (run make build)"
kind load docker-image "$IMAGE" --name "$CLUSTER"
