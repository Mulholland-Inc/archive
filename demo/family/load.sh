#!/bin/sh
set -eu
family=$(dirname "$0")
archive=${ARCHIVE_BIN:-archive}

$archive schema migrate "$family/schema.sql" >/dev/null
ARCHIVE_RUN_TIMEOUT=600s $archive run "$family/seed.ts"
