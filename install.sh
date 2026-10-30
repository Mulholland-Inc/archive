#!/bin/sh
set -eu

image="${ARCHIVE_IMAGE:-ghcr.io/mulholland-inc/archive:latest}"
web_image="${ARCHIVE_WEB_IMAGE:-ghcr.io/mulholland-inc/archive-web:latest}"
name="${ARCHIVE_NAME:-archive}"
port="${ARCHIVE_PORT:-8080}"
web_port="${ARCHIVE_WEB_PORT:-3000}"
volume="${ARCHIVE_VOLUME:-archive}"
export ARCHIVE_URL="${ARCHIVE_URL:-http://localhost:$web_port}"
export ARCHIVE_API_URL="${ARCHIVE_API_URL:-http://localhost:$port}"

say() { printf '%s\n' "$*"; }
fail() { printf 'archive: %s\n' "$*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || fail "Docker is required. Install it from https://docs.docker.com/get-docker/ and run this again."
docker info >/dev/null 2>&1 || fail "Docker is installed but not running. Start it and run this again."

for i in "$image" "$web_image"; do
  say "Getting $i"
  docker pull -q "$i" >/dev/null 2>&1 || docker image inspect "$i" >/dev/null 2>&1 || fail "could not get $i"
done

if docker container inspect "$name" >/dev/null 2>&1; then
  say "Updating $name. Your data stays in the $volume volume."
  docker stop -t 30 "$name" >/dev/null
  docker rm "$name" >/dev/null
fi
docker rm -f "$name-web" >/dev/null 2>&1 || true

set --
for var in ARCHIVE_LOCALE ARCHIVE_URL ARCHIVE_API_URL ARCHIVE_SECRET_KEY ARCHIVE_GOOGLE_CLIENT_ID ARCHIVE_GOOGLE_CLIENT_SECRET ARCHIVE_MICROSOFT_CLIENT_ID ARCHIVE_MICROSOFT_CLIENT_SECRET ARCHIVE_SLACK_CLIENT_ID ARCHIVE_SLACK_CLIENT_SECRET ARCHIVE_GITHUB_CLIENT_ID ARCHIVE_GITHUB_CLIENT_SECRET ARCHIVE_BLOB_URL ARCHIVE_BLOB_KEY ARCHIVE_BLOB_SECRET DATABASE_URL; do
  eval "set_=\${$var+x}"
  [ -n "$set_" ] && set -- "$@" -e "$var"
done

docker run -d --name "$name" \
  --restart unless-stopped \
  --stop-timeout 30 \
  --add-host host.docker.internal:host-gateway \
  -v "$volume:/data" \
  -p "$port:8080" \
  "$@" "$image" >/dev/null

docker run -d --name "$name-web" \
  --restart unless-stopped \
  -p "$web_port:8080" \
  -e ARCHIVE_API_URL \
  "$web_image" >/dev/null

say "Starting Archive"
tries=0
until [ "$(docker inspect -f '{{.State.Health.Status}}' "$name" 2>/dev/null)" = healthy ]; do
  tries=$((tries + 1))
  state="$(docker inspect -f '{{.State.Status}}' "$name" 2>/dev/null || echo gone)"
  if [ "$state" != running ] || [ "$tries" -gt 90 ]; then
    docker logs --tail 20 "$name" >&2 || true
    fail "Archive did not start. The log above says why."
  fi
  sleep 2
done

say ""
say "Archive is running at $ARCHIVE_URL, with its API and MCP at $ARCHIVE_API_URL"
docker logs "$name" 2>&1 | grep -i -E 'setup|sign.?in' | tail -n 3 || true
if docker logs "$name" 2>&1 | grep -q 'no users yet'; then
  say "Add yourself as the first owner: docker exec $name archive user add you@example.com"
fi
say "Run this command again any time to update."
