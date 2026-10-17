#!/bin/sh
# Chart conformance: helm template assertions for kind and OpenShift values.
set -eu
cd "$(dirname "$0")/.."
. scripts/env.sh

require helm grep

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
DUMMY="dummy-pw-5f3a9c"

fail() { echo "chart-check: FAIL: $*" >&2; exit 1; }
has() { grep -Eq -- "$2" "$1" || fail "$3 ($1)"; }
hasnt() { if grep -Eq -- "$2" "$1"; then fail "$3 ($1)"; fi; }
count() { grep -Ec -- "$2" "$1" || true; }

render() { # render <out> <extra helm args>...
  out=$1; shift
  helm template t "$CHART_DIR" --namespace "$NAMESPACE" \
    --set-string kafka.sasl.existingSecret=creds \
    --set-string instanceName=side-a \
    --set-string ingress.host=a.clopy-paste.local \
    "$@" >"$out" || fail "helm template failed ($out)"
}

render "$tmp/kind.yaml" --set openshift.enabled=false --set ingress.enabled=true --set-string ingress.className=nginx \
  --set 'networkPolicy.ingressNamespaceSelector.policy-group\.network\.openshift\.io/ingress=null' \
  --set-string 'networkPolicy.ingressNamespaceSelector.kubernetes\.io/metadata\.name=ingress-nginx'
render "$tmp/ocp.yaml" --set openshift.enabled=true

for f in "$tmp/kind.yaml" "$tmp/ocp.yaml"; do
  has "$f" '^  replicas: 1$' "replicas must be 1"
  has "$f" '^    type: Recreate$' "strategy must be Recreate"
  hasnt "$f" 'runAsUser|fsGroup' "runAsUser/fsGroup must be absent"
  has "$f" 'automountServiceAccountToken: false' "automountServiceAccountToken must be false"
  has "$f" 'cpu: 50m' "cpu request 50m"
  has "$f" 'memory: 64Mi' "memory request 64Mi"
  has "$f" 'cpu: 500m' "cpu limit 500m"
  has "$f" 'memory: 128Mi' "memory limit 128Mi"
  has "$f" 'name: GOMEMLIMIT' "GOMEMLIMIT env"
  for v in KAFKA_SASL_USERNAME KAFKA_SASL_PASSWORD; do
    grep -A3 -- "name: $v\$" "$f" | grep -q secretKeyRef || fail "$v must use secretKeyRef ($f)"
  done
  has "$f" 'periodSeconds: 2$' "readiness periodSeconds 2"
  has "$f" 'initialDelaySeconds: 0$' "readiness initialDelaySeconds 0"
  [ "$(count "$f" '^kind: NetworkPolicy$')" -eq 1 ] || fail "exactly one NetworkPolicy ($f)"
  [ "$(count "$f" '^  - from:|^    - from:')" -eq 1 ] || fail "exactly one ingress rule ($f)"
  hasnt "$f" 'namespaceSelector: \{\}' "empty namespaceSelector"
  has "$f" 'namespaceSelector:' "namespaceSelector present"
done

# Route xor Ingress
[ "$(count "$tmp/kind.yaml" '^kind: Ingress$')" -eq 1 ] || fail "kind: one Ingress"
hasnt "$tmp/kind.yaml" '^kind: Route$' "kind: no Route"
[ "$(count "$tmp/ocp.yaml" '^kind: Route$')" -eq 1 ] || fail "openshift: one Route"
hasnt "$tmp/ocp.yaml" '^kind: Ingress$' "openshift: no Ingress"
has "$tmp/ocp.yaml" 'policy-group.network.openshift.io/ingress' "openshift default selector"
has "$tmp/kind.yaml" 'kubernetes.io/metadata.name: ingress-nginx' "kind selector"
hasnt "$tmp/kind.yaml" 'policy-group.network.openshift.io' "kind must not AND the openshift label"
# Route renders even when ingress.enabled is true or false
render "$tmp/ocp2.yaml" --set openshift.enabled=true --set ingress.enabled=true
[ "$(count "$tmp/ocp2.yaml" '^kind: Route$')" -eq 1 ] || fail "openshift+ingress.enabled: one Route"
hasnt "$tmp/ocp2.yaml" '^kind: Ingress$' "openshift+ingress.enabled: no Ingress"

# deploy.sh renders an Ingress for both releases
for r in a b; do
  set --
  for kv in $(sed -n "/clopy-paste-$r \"/,/^$/p" scripts/deploy.sh | grep -o -- '--set[a-z-]* [^ ]*' | sed 's/ /@/'); do
    flag=${kv%%@*}; kv=${kv#*@}
    kv=$(printf '%s' "$kv" | sed "s/^'//;s/'\$//")
    k=${kv%%=*}; v=$(printf '%s' "${kv#*=}" | sed 's/^"//;s/"$//')
    case "$v" in *'${'*) v=x ;; esac
    case "$k" in image.*) continue ;; esac
    set -- "$@" "$flag" "$k=$v"
  done
  helm template t "$CHART_DIR" "$@" >"$tmp/deploy-$r.yaml" || fail "deploy.sh release $r does not render"
  has "$tmp/deploy-$r.yaml" '^kind: Ingress$' "deploy.sh release $r must render an Ingress"
  has "$tmp/deploy-$r.yaml" 'kubernetes.io/metadata.name: ingress-nginx' "deploy.sh release $r kind selector"
  hasnt "$tmp/deploy-$r.yaml" 'policy-group.network.openshift.io' "deploy.sh release $r must drop openshift selector"
done

# no secret value in output; schema rejects unknown keys
render "$tmp/pw.yaml" --set openshift.enabled=false
hasnt "$tmp/pw.yaml" "$DUMMY" "dummy password leaked"
if helm template t "$CHART_DIR" --set-string "kafka.sasl.password=$DUMMY" >"$tmp/bad.yaml" 2>"$tmp/bad.err"; then
  fail "schema must reject kafka.sasl.password"
fi
echo "chart-check: ok"
