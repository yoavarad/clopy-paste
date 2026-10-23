#!/bin/sh
# Make the env-up images available in the local docker daemon. With a cache tarball at
# $ENV_IMAGES_TAR (restored by CI), load it; otherwise pull the images and write the tarball.
# Keyed on scripts/tools.sh in CI, so a version bump rebuilds it. env-up.sh kind-loads the images.
set -eu
cd "$(dirname "$0")/.."
. scripts/env.sh
. scripts/tools.sh

: "${ENV_IMAGES_TAR:=$HOME/.cache/clopy-paste/env-images.tar}"
require docker
if [ -s "$ENV_IMAGES_TAR" ]; then
  docker load -i "$ENV_IMAGES_TAR"
else
  for img in $ENV_IMAGES; do docker pull "$img"; done
  mkdir -p "$(dirname "$ENV_IMAGES_TAR")"
  # shellcheck disable=SC2086
  docker save -o "$ENV_IMAGES_TAR" $ENV_IMAGES
fi
echo "env-images: ok"
