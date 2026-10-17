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
