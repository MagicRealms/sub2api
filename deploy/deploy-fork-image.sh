#!/usr/bin/env bash
# Deploy an already built local fork image; never pull the upstream image.
set -Eeuo pipefail
umask 077
image=${1:?Usage: deploy-fork-image.sh magicrealms/sub2api:VERSION [compose-directory]}
cd "${2:-/root/sub2api-deploy}"
[[ "$image" == magicrealms/sub2api:* ]] || exit 2
source_url=$(docker image inspect "$image" --format '{{index .Config.Labels "org.opencontainers.image.source"}}')
[[ "$source_url" == https://github.com/MagicRealms/sub2api ]] || exit 2
new_commit=$(docker image inspect "$image" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}')
[[ "$new_commit" =~ ^[a-f0-9]{40}$ ]] || exit 2
old_image=$(docker inspect sub2api --format '{{.Config.Image}}')
rollback_image=${FORK_ROLLBACK_IMAGE:-$old_image}
docker image inspect "$rollback_image" >/dev/null
if [[ "$old_image" != magicrealms/sub2api:* && -z ${FORK_ROLLBACK_IMAGE:-} ]]; then
  echo 'Legacy online-updated container: first prepare a rollback image containing its actual running binary.' >&2
  exit 1
fi
backup="backups/$(date -u +%Y%m%dT%H%M%SZ)-fork-$new_commit"
mkdir -p "$backup"
cp -p docker-compose.yml docker-compose.override.yml "$backup/"
[[ ! -f .env ]] || cp -p .env "$backup/.env"
docker compose exec -T postgres pg_dump -U sub2api -d sub2api -Fc > "$backup/database.dump"
docker compose exec -T sub2api tar -C /app/data --exclude=logs -czf - . > "$backup/app-data.tar.gz"
printf '%s\n' "$rollback_image" > "$backup/rollback-image"
printf '%s\n' "$image" > "$backup/deployed-image"

# The operator permits immediate restarts. Draining remains an explicit opt-in.
if [[ ${FORK_DRAIN_BEFORE_RESTART:-false} == true ]]; then
  # Avoid interrupting an active stream at the server's short shutdown deadline.
  admin_key=$(docker compose exec -T postgres psql -X -At -U sub2api -d sub2api -c "SELECT value FROM settings WHERE key='admin_api_key'")
  [[ "$admin_key" =~ ^[A-Za-z0-9_-]{30,200}$ ]] || exit 2
  drained=false
  for ((attempt=0; attempt<60; attempt++)); do
    if docker compose exec -T sub2api curl --fail --silent --show-error --noproxy '*' --max-time 10 \
      --header "x-api-key: $admin_key" http://127.0.0.1:8080/api/v1/admin/ops/concurrency |
      python3 -c 'import json,sys; d=json.load(sys.stdin); a=d["data"]["account"]; sys.exit(0 if d["code"] == 0 and a and all(v["current_in_use"] == 0 and v["waiting_in_queue"] == 0 for v in a.values()) else 1)'; then
      drained=true
      break
    fi
    sleep 5
  done
  unset admin_key
  if [[ "$drained" != true ]]; then
    echo 'Active traffic did not drain; the deployment was left unchanged.' >&2
    exit 1
  fi
fi

# Change only the application's image; keep the existing limits and mounts.
set_image() {
  python3 - "$1" <<'PY'
import os,pathlib,re,sys,tempfile
p=pathlib.Path('docker-compose.override.yml')
s=p.read_text()
s,n=re.subn(r'(?m)(^  sub2api:\n(?:^    .*\n)*?^    image: )[^\n]+',lambda m:m[1]+sys.argv[1],s,count=1)
if n != 1: raise SystemExit('Cannot locate exactly one sub2api image in the override')
with tempfile.NamedTemporaryFile(mode='w',dir=p.parent,delete=False) as f:
    tmp=pathlib.Path(f.name)
    f.write(s)
try:
    os.chmod(tmp,p.stat().st_mode)
    os.replace(tmp,p)
finally:
    tmp.unlink(missing_ok=True)
PY
}
set_image "$image"
if ! docker compose config >/dev/null; then
  cp -p "$backup/docker-compose.override.yml" docker-compose.override.yml
  exit 1
fi
healthy=false
if docker compose up -d --no-deps --pull never sub2api; then
  for ((attempt=0; attempt<36; attempt++)); do
    state=$(docker inspect sub2api --format '{{if .State.Health}}{{.State.Health.Status}}{{end}}')
    if [[ "$state" == healthy ]]; then healthy=true; break; fi
    sleep 5
  done
fi
if [[ "$healthy" != true ]]; then
  cp -p "$backup/docker-compose.override.yml" docker-compose.override.yml
  set_image "$rollback_image"
  docker compose up -d --no-deps --pull never sub2api
  echo "New image failed health checks; restored $rollback_image. Database backup: $backup/database.dump" >&2
  exit 1
fi
printf '%s\n' "$image" > .last-built-image
printf 'Healthy image: %s\nBackup: %s\n' "$image" "$backup"
