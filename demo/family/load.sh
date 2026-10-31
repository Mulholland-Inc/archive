#!/bin/sh
set -eu
pack=$(dirname "$0")
archive=${ARCHIVE_BIN:-archive}

schema=$(mktemp -d)
trap 'rm -rf "$schema"' EXIT
$archive schema pull "$schema" >/dev/null
cp "$pack"/schema/*.sql "$schema"
$archive schema apply "$schema" >/dev/null
ARCHIVE_RUN_TIMEOUT=600s $archive run "$pack/seed.ts"
