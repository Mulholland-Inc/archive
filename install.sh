#!/bin/sh
set -eu

image="${ARCHIVE_IMAGE:-ghcr.io/mulholland-inc/archive:latest}"
web_image="${ARCHIVE_WEB_IMAGE:-ghcr.io/mulholland-inc/archive-web:latest}"
pg_image="${ARCHIVE_PG_IMAGE:-postgres:18-alpine}"
name="${ARCHIVE_NAME:-archive}"
port="${ARCHIVE_PORT:-8080}"
web_port="${ARCHIVE_WEB_PORT:-3000}"
volume="${ARCHIVE_VOLUME:-archive}"
export ARCHIVE_URL="${ARCHIVE_URL:-http://localhost:$web_port}"
export ARCHIVE_API_URL="${ARCHIVE_API_URL:-http://localhost:$port}"
export DATABASE_URL="${DATABASE_URL:-postgres://archive:archive@$name-postgres:5432/archive?sslmode=disable}"
export ARCHIVE_DATA=/home/nonroot

say() { printf '%s\n' "$*"; }
fail() { printf 'archive: %s\n' "$*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || fail "Docker is required. Install it from https://docs.docker.com/get-docker/ and run this again."
docker info >/dev/null 2>&1 || fail "Docker is installed but not running. Start it and run this again."

for i in "$image" "$web_image" "$pg_image"; do
  say "Getting $i"
  docker pull -q "$i" >/dev/null 2>&1 || docker image inspect "$i" >/dev/null 2>&1 || fail "could not get $i"
done

docker network inspect "$name" >/dev/null 2>&1 || docker network create "$name" >/dev/null
if ! docker container inspect "$name-postgres" >/dev/null 2>&1; then
  docker run -d --name "$name-postgres" --network "$name" --restart unless-stopped \
    -e POSTGRES_USER=archive -e POSTGRES_PASSWORD=archive -e POSTGRES_DB=archive \
    -v "$volume-postgres:/var/lib/postgresql" "$pg_image" >/dev/null
fi
docker start "$name-postgres" >/dev/null
tries=0
until docker exec "$name-postgres" pg_isready -q -h 127.0.0.1 -U archive 2>/dev/null; do
  tries=$((tries + 1))
  [ "$tries" -gt 60 ] && fail "Postgres did not start. docker logs $name-postgres says why."
  sleep 1
done

if docker container inspect "$name" >/dev/null 2>&1; then
  say "Updating $name. Your data stays in the $volume and $volume-postgres volumes."
  docker stop -t 30 "$name" >/dev/null
  docker rm "$name" >/dev/null
fi
docker rm -f "$name-web" >/dev/null 2>&1 || true

set --
for var in DATABASE_URL ARCHIVE_DATA ARCHIVE_LOCALE ARCHIVE_URL ARCHIVE_API_URL ARCHIVE_SECRET_KEY ARCHIVE_GOOGLE_CLIENT_ID ARCHIVE_GOOGLE_CLIENT_SECRET ARCHIVE_MICROSOFT_CLIENT_ID ARCHIVE_MICROSOFT_CLIENT_SECRET ARCHIVE_SLACK_CLIENT_ID ARCHIVE_SLACK_CLIENT_SECRET ARCHIVE_GITHUB_CLIENT_ID ARCHIVE_GITHUB_CLIENT_SECRET ARCHIVE_BLOB_URL ARCHIVE_BLOB_KEY ARCHIVE_BLOB_SECRET; do
  eval "set_=\${$var+x}"
  [ -n "$set_" ] && set -- "$@" -e "$var"
done

docker run -d --name "$name" --network "$name" \
  --restart unless-stopped \
  --stop-timeout 30 \
  --add-host host.docker.internal:host-gateway \
  -v "$volume:/home/nonroot" \
  -p "$port:8080" \
  "$@" "$image" serve >/dev/null

docker run -d --name "$name-web" \
  --restart unless-stopped \
  -p "$web_port:8080" \
  -e ARCHIVE_API_URL \
  "$web_image" >/dev/null

say "Starting Archive"
tries=0
until curl -fsS -o /dev/null "http://localhost:$port/healthz" 2>/dev/null; do
  tries=$((tries + 1))
  state="$(docker inspect -f '{{.State.Status}}' "$name" 2>/dev/null || echo gone)"
  if [ "$state" != running ] || [ "$tries" -gt 90 ]; then
    docker logs --tail 20 "$name" >&2 || true
    fail "Archive did not start. The log above says why."
  fi
  sleep 1
done

say ""
say "Archive is running at $ARCHIVE_URL, with its API and MCP at $ARCHIVE_API_URL"
docker logs "$name" 2>&1 | grep -i -E 'setup|sign.?in' | tail -n 3 || true
if docker logs "$name" 2>&1 | grep -q 'no users yet'; then
  say "Add yourself as the first owner: docker exec $name archive user add you@example.com"
fi
say "Run this command again any time to update."
