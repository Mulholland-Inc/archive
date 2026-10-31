#!/bin/sh
set -eu
family=$(dirname "$0")
archive=${ARCHIVE_BIN:-archive}

schema=$(mktemp -d)
trap 'rm -rf "$schema"' EXIT
$archive schema pull "$schema" >/dev/null
cp "$family"/schema/*.sql "$schema"
$archive schema apply "$schema" >/dev/null
ARCHIVE_RUN_TIMEOUT=600s $archive run "$family/seed.ts"
