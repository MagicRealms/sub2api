#!/usr/bin/env bash
# Build a pinned MagicRealms checkout without changing a running deployment.
set -Eeuo pipefail
repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo"
commit=$(git rev-parse HEAD)
if [[ -n $(git status --porcelain --untracked-files=normal) ]]; then
  echo 'Commit or stash local changes before building a deployment image.' >&2
  exit 1
fi
version=${1:-$(cat backend/cmd/server/VERSION)-mr.${commit:0:12}}
[[ "$version" =~ ^[A-Za-z0-9][A-Za-z0-9.+-]*$ ]] || exit 2
image="magicrealms/sub2api:$version"
python3 deploy/tests/test_fork_deploy.py
cache=${SUB2API_BUILD_CACHE:-$HOME/.cache/sub2api-fork}
mkdir -p "$cache/gomod" "$cache/gobuild" "$cache/artifacts/$commit"
cache=$(cd "$cache" && pwd)
artifact="$cache/artifacts/$commit"
go_version=$(awk '$1 == "go" {print $2; exit}' backend/go.mod)

# Keep substantial headroom for the live API and host networking under load.
# Build stages stay sequential, low-priority, single-worker, and capped at one CPU.
docker run --rm --cpus 1 --cpu-shares 128 --memory 3072m --memory-swap 3072m \
  -e NODE_OPTIONS=--max-old-space-size=2200 \
  -e VITEST_MAX_THREADS=1 -e VITEST_MIN_THREADS=1 \
  -e VITEST_MAX_FORKS=1 -e VITEST_MIN_FORKS=1 \
  -v "$repo:/app" -w /app/frontend node:24-alpine \
  sh -ec 'corepack enable; corepack prepare pnpm@9.15.9 --activate; pnpm install --frozen-lockfile --network-concurrency=2 --child-concurrency=1 --prefer-offline; nice -n 10 pnpm exec vitest run --maxWorkers=1 --minWorkers=1 src/components/common/__tests__/VersionBadge.spec.ts src/stores/__tests__/app.spec.ts; nice -n 10 pnpm run build'

go_args=(--rm --cpus 1 --cpu-shares 128 --memory 2304m --memory-swap 2304m
  -e GOMAXPROCS=1 -e GOMEMLIMIT=1400MiB -e CGO_ENABLED=0
  -v "$repo:/src" -v "$cache/gomod:/go/pkg/mod"
  -v "$cache/gobuild:/root/.cache/go-build" -w /src/backend)
docker run "${go_args[@]}" "golang:$go_version-alpine" \
  nice -n 10 go test -tags=unit -p 1 -timeout 10m ./internal/repository -run 'TestHTTPUpstream|TestOpenAIHTTP2|TestDecompressResponseBody|TestOpsCurrentRatesRespectMonitoringWindow'
docker run "${go_args[@]}" "golang:$go_version-alpine" \
  nice -n 10 go test -tags=unit -p 1 ./internal/config -run TestOpsMonitoringStartAt
docker run "${go_args[@]}" "golang:$go_version-alpine" \
  nice -n 10 go test -tags=unit -p 1 -timeout 2m internal/service/update_service.go \
  internal/service/update_service_test.go internal/service/update_service_managed_test.go
docker run "${go_args[@]}" "golang:$go_version-alpine" \
  nice -n 10 go test -tags=unit -p 1 internal/service/ops_monitoring_window.go internal/service/ops_monitoring_window_test.go
docker run "${go_args[@]}" -v "$artifact:/out" "golang:$go_version-alpine" \
  nice -n 10 go build -p 1 -tags embed \
  -ldflags "-s -w -X main.Version=$version -X main.Commit=$commit -X main.Date=$(date -u +%FT%TZ) -X main.BuildType=source" \
  -o /out/sub2api ./cmd/server

cp -a backend/resources "$artifact/"
cp deploy/docker-entrypoint.sh "$artifact/docker-entrypoint.sh"
docker build -f deploy/Dockerfile.fork-runtime \
  --build-arg "VERSION=$version" --build-arg "COMMIT=$commit" -t "$image" "$artifact"
docker run --rm --network none "$image" -version
printf 'Built image: %s\nCommit: %s\n' "$image" "$commit"
