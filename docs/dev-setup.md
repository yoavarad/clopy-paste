# Dev setup

`make check` runs lint + unit tests (no cluster). `make e2e` needs Linux tooling plus Docker, so on
Windows run it from WSL Ubuntu.

## Local loop

- `make check` - lint + test. `scripts/test.sh` streams `go test` output and enforces the coverage gate.
- `-race` needs cgo and a C compiler. Without `gcc` on PATH, test.sh prints `test: WARNING -race skipped`
  and runs without it; install `gcc` to get race coverage.

## Windows host: e2e through WSL Ubuntu

1. Install WSL Ubuntu (`wsl --install -d Ubuntu`) and add to `%UserProfile%\.wslconfig`:

   ```ini
   [wsl2]
   memory=8GB
   ```

   Then run `wsl --shutdown` and reopen Ubuntu.
2. Install packages in Ubuntu:

   ```sh
   sudo apt-get update
   sudo apt-get install -y docker.io docker-buildx make gcc curl
   sudo usermod -aG docker "$USER"   # re-login afterwards
   sudo service docker start
   ```
3. Install Go from go.dev, matching the version in `go.mod`. Download the tarball and its sha256 from
   https://go.dev/dl/, verify, then install:

   ```sh
   echo "<sha256>  go<version>.linux-amd64.tar.gz" | sha256sum -c -
   sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go<version>.linux-amd64.tar.gz
   export PATH=$PATH:/usr/local/go/bin   # add to ~/.profile
   ```
4. Install pinned trivy, kind, helm and kubectl (checksum-verified):

   ```sh
   TOOLS_BIN=/usr/local/bin sudo -E sh scripts/ci-tools.sh
   ```
5. Clone the repo inside the WSL filesystem (e.g. `~/clopy-paste`), not under `/mnt/c`.

## Running e2e

```sh
make e2e              # creates the kind cluster, runs tests, tears the cluster down
KEEP_ENV=1 make e2e   # keep the cluster afterwards for debugging
```

## Cleaning up a left-over kind cluster

```sh
kind get clusters
make env-down                          # deletes the cluster named by $CLUSTER (default clopy-paste)
kind delete cluster --name clopy-paste # equivalent
```
