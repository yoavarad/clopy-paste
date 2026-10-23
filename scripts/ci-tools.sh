#!/bin/sh
# Install pinned trivy, kind, helm and kubectl into $TOOLS_BIN (plain downloads, no marketplace
# actions; ADR-010). Every asset is verified against the SHA256 in scripts/tools.sh; mismatch fails.
set -eu
cd "$(dirname "$0")/.."
. scripts/env.sh
. scripts/tools.sh

: "${TOOLS_BIN:=$HOME/.local/bin}"
stamp=$(cat scripts/tools.sh scripts/ci-tools.sh | sha256sum | cut -d" " -f1)
if [ "$(cat "$TOOLS_BIN/.ci-tools-stamp" 2>/dev/null || true)" = "$stamp" ]; then
  echo "ci-tools: cached tools up to date in $TOOLS_BIN"
  exit 0
fi
require curl tar sha256sum
mkdir -p "$TOOLS_BIN"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# fetch <url> <file> <sha256>: download into $tmp and verify before use.
fetch() {
  curl -fsSL -o "$tmp/$2" "$1"
  echo "$3  $tmp/$2" | sha256sum -c - || die "checksum mismatch for $2"
}

fetch "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz" trivy.tgz "$TRIVY_SHA256"
tar -xzf "$tmp/trivy.tgz" -C "$tmp" trivy
install -m 0755 "$tmp/trivy" "$TOOLS_BIN/trivy"

fetch "https://github.com/kubernetes-sigs/kind/releases/download/v${KIND_VERSION}/kind-linux-amd64" kind "$KIND_SHA256"
install -m 0755 "$tmp/kind" "$TOOLS_BIN/kind"

fetch "https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz" helm.tgz "$HELM_SHA256"
tar -xzf "$tmp/helm.tgz" -C "$tmp" linux-amd64/helm
install -m 0755 "$tmp/linux-amd64/helm" "$TOOLS_BIN/helm"

fetch "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl" kubectl "$KUBECTL_SHA256"
install -m 0755 "$tmp/kubectl" "$TOOLS_BIN/kubectl"

echo "$stamp" > "$TOOLS_BIN/.ci-tools-stamp"
echo "ci-tools: installed trivy $TRIVY_VERSION, kind $KIND_VERSION, helm $HELM_VERSION, kubectl $KUBECTL_VERSION into $TOOLS_BIN"
