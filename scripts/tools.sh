# Sourced by scripts/ci-tools.sh. Single place for pinned CI tool versions and the SHA256 of each
# downloaded release asset (linux/amd64), taken from the projects' published checksum files.
# Versions follow ADR-015 (kind, helm). To bump: change version AND checksum together.
TRIVY_VERSION=0.75.0
TRIVY_SHA256=c6e65abddb348e25f10549df887045629cf28cc72453cd1c63acb717316b3f3f   # trivy_0.75.0_Linux-64bit.tar.gz
KIND_VERSION=0.33.0
KIND_SHA256=aee6151561422756b764a4ae28e7f44cda5af5a9eead3cc9985112b1de8d8e0d    # kind-linux-amd64
HELM_VERSION=4.3.0
HELM_SHA256=86584a54def73570558f66f5111cc53dfed56689637ae32c1201205d494f54fb    # helm-v4.3.0-linux-amd64.tar.gz
KUBECTL_VERSION=1.33.1
KUBECTL_SHA256=5de4e9f2266738fd112b721265a0c1cd7f4e5208b670f811861f699474a100a3 # kubectl

# Local env (make env-up). Pinned like the tools above; env-up.sh and env-images.sh read these.
KIND_NODE_IMAGE=kindest/node:v1.34.11@sha256:44e222ee2132dab25ff87301682f89eb82c7880ea3a1bf543bfe9708fd08d67d
STRIMZI_VERSION=1.2.0
STRIMZI_CHART_SHA256=0f8a50b2f19bd99482f9fd6e17cf42902f72f9e594a136813ac3f0b7af422efd   # strimzi-kafka-operator-1.2.0.tgz (helm pull oci://quay.io/strimzi-helm/strimzi-kafka-operator)
STRIMZI_KAFKA_VERSION=4.2.1   # must match spec.kafka.version in deploy/local/kafka.yaml
INGRESS_NGINX_SHA256=cb84b0ea747c9149cce08aef4e95b1e55f183f07299f40e7009043e960a0133f   # deploy/local/ingress-nginx.yaml (controller-v1.12.1, provider/kind)
# Images preloaded into kind (cached as a docker save tarball in CI); must match the manifest and chart.
ENV_IMAGES="registry.k8s.io/ingress-nginx/controller:v1.12.1@sha256:d2fbc4ec70d8aa2050dd91a91506e998765e86c96f32cffb56c503c9c34eed5b registry.k8s.io/ingress-nginx/kube-webhook-certgen:v1.5.2@sha256:e8825994b7a2c7497375a9b945f386506ca6a3eda80b89b74ef2db743f66a5ea quay.io/strimzi/operator:${STRIMZI_VERSION} quay.io/strimzi/kafka:${STRIMZI_VERSION}-kafka-${STRIMZI_KAFKA_VERSION}"
