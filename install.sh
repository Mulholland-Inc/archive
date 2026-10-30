#!/bin/sh
set -eu

image="${ARCHIVE_IMAGE:-ghcr.io/mulholland-inc/archive:latest}"
name="${ARCHIVE_NAME:-archive}"
port="${ARCHIVE_PORT:-8080}"
volume="${ARCHIVE_VOLUME:-archive}"

say() { printf '%s\n' "$*"; }
fail() { printf 'archive: %s\n' "$*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || fail "Docker is required. Install it from https://docs.docker.com/get-docker/ and run this again."
docker info >/dev/null 2>&1 || fail "Docker is installed but not running. Start it and run this again."

say "Getting $image"
docker pull -q "$image" >/dev/null 2>&1 || docker image inspect "$image" >/dev/null 2>&1 || fail "could not get $image"

if docker container inspect "$name" >/dev/null 2>&1; then
  say "Updating $name. Your data stays in the $volume volume."
  docker stop -t 30 "$name" >/dev/null
  docker rm "$name" >/dev/null
fi

set --
for var in ARCHIVE_LOCALE ARCHIVE_ADMIN_EMAIL ARCHIVE_URL ARCHIVE_SECRET_KEY ARCHIVE_GOOGLE_CLIENT_ID ARCHIVE_GOOGLE_CLIENT_SECRET ARCHIVE_MICROSOFT_CLIENT_ID ARCHIVE_MICROSOFT_CLIENT_SECRET ARCHIVE_SLACK_CLIENT_ID ARCHIVE_SLACK_CLIENT_SECRET ARCHIVE_GITHUB_CLIENT_ID ARCHIVE_GITHUB_CLIENT_SECRET ARCHIVE_BLOB_URL ARCHIVE_BLOB_KEY ARCHIVE_BLOB_SECRET DATABASE_URL; do
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
say "Archive is running at http://localhost:$port"
docker logs "$name" 2>&1 | grep -i -E 'setup|sign.?in' | tail -n 3 || true
say "Run this command again any time to update."
