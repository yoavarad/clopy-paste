#!/bin/sh
# make test-integration: Kafka adapter tests (build tag integration) against the Strimzi cluster from make env-up.
# The listeners are cluster-internal, so the compiled test binary runs in a throwaway pod in the cluster.
# To run against brokers you already can reach, export KAFKA_IT_PLAINTEXT_BOOTSTRAP, KAFKA_IT_SASL_BOOTSTRAP,
# KAFKA_IT_USERNAME, KAFKA_IT_PASSWORD and KAFKA_IT_CA_FILE and the tests run locally instead.
set -eu
cd "$(dirname "$0")/.."
. scripts/env.sh

require go
PKG=./internal/adapters/kafka

if [ -n "${KAFKA_IT_SASL_BOOTSTRAP:-}" ] && [ -n "${KAFKA_IT_PLAINTEXT_BOOTSTRAP:-}" ] &&
  [ -n "${KAFKA_IT_USERNAME:-}" ] && [ -n "${KAFKA_IT_PASSWORD:-}" ] && [ -n "${KAFKA_IT_CA_FILE:-}" ]; then
  go test -tags integration -count=1 -v "$PKG"
  exit 0
fi

require kubectl
: "${KAFKA_IT_USER_SECRET:=clopy-paste-a}"
: "${KAFKA_IT_POD:=kafka-it}"
tmp=$(mktemp -d)
trap 'kubectl delete pod "$KAFKA_IT_POD" -n "$NAMESPACE" --ignore-not-found --wait=false >/dev/null 2>&1 || true; rm -rf "$tmp"' EXIT

kubectl get secret "$KAFKA_CLUSTER-cluster-ca-cert" -n "$NAMESPACE" -o go-template='{{index .data "ca.crt" | base64decode}}' >"$tmp/ca.crt"
password=$(kubectl get secret "$KAFKA_IT_USER_SECRET" -n "$NAMESPACE" -o go-template='{{.data.password | base64decode}}')
[ -n "$password" ] || die "secret $KAFKA_IT_USER_SECRET has no password; is the KafkaUser ready?"

# Static linux binary for the kind node architecture (the nodes run on this host).
GOOS=linux GOARCH=$(go env GOARCH) CGO_ENABLED=0 go test -c -tags integration -o "$tmp/kafka.test" "$PKG"

kubectl delete pod "$KAFKA_IT_POD" -n "$NAMESPACE" --ignore-not-found >/dev/null
kubectl run "$KAFKA_IT_POD" -n "$NAMESPACE" --image="${KAFKA_IT_IMAGE:-alpine:3.20}" --restart=Never --command -- sleep 3600
kubectl wait "pod/$KAFKA_IT_POD" -n "$NAMESPACE" --for=condition=Ready --timeout=120s
kubectl cp "$tmp/kafka.test" "$NAMESPACE/$KAFKA_IT_POD:/tmp/kafka.test"
kubectl cp "$tmp/ca.crt" "$NAMESPACE/$KAFKA_IT_POD:/tmp/ca.crt"
kubectl exec -n "$NAMESPACE" "$KAFKA_IT_POD" -- chmod +x /tmp/kafka.test

kubectl exec -n "$NAMESPACE" "$KAFKA_IT_POD" -- env \
  "KAFKA_IT_PLAINTEXT_BOOTSTRAP=$KAFKA_CLUSTER-kafka-bootstrap.$NAMESPACE.svc:9092" \
  "KAFKA_IT_SASL_BOOTSTRAP=$KAFKA_CLUSTER-kafka-bootstrap.$NAMESPACE.svc:9093" \
  "KAFKA_IT_USERNAME=$KAFKA_IT_USER_SECRET" \
  "KAFKA_IT_PASSWORD=$password" \
  "KAFKA_IT_CA_FILE=/tmp/ca.crt" \
  /tmp/kafka.test -test.v -test.count=1 -test.timeout=10m
