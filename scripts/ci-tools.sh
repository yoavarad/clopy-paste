#!/bin/sh
# Install pinned trivy, kind, helm and kubectl into $TOOLS_BIN (plain downloads, no marketplace
# actions; ADR-010). Used by GitHub workflows only. Override versions from the environment.
set -eu
cd "$(dirname "$0")/.."
. scripts/env.sh

: "${TRIVY_VERSION:=0.58.1}"
: "${KIND_VERSION:=0.25.0}"
: "${HELM_VERSION:=3.16.3}"
: "${KUBECTL_VERSION:=1.31.3}"
: "${TOOLS_BIN:=$HOME/.local/bin}"
require curl tar
mkdir -p "$TOOLS_BIN"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

curl -fsSL "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz" | tar -xz -C "$tmp" trivy
install -m 0755 "$tmp/trivy" "$TOOLS_BIN/trivy"

curl -fsSL -o "$tmp/kind" "https://github.com/kubernetes-sigs/kind/releases/download/v${KIND_VERSION}/kind-linux-amd64"
install -m 0755 "$tmp/kind" "$TOOLS_BIN/kind"

curl -fsSL "https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz" | tar -xz -C "$tmp" linux-amd64/helm
install -m 0755 "$tmp/linux-amd64/helm" "$TOOLS_BIN/helm"

curl -fsSL -o "$tmp/kubectl" "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl"
install -m 0755 "$tmp/kubectl" "$TOOLS_BIN/kubectl"

echo "ci-tools: installed trivy $TRIVY_VERSION, kind $KIND_VERSION, helm $HELM_VERSION, kubectl $KUBECTL_VERSION into $TOOLS_BIN"
