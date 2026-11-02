#!/bin/sh
set -eu
family=$(dirname "$0")
archive=${ARCHIVE_BIN:-archive}

$archive schema migrate "$family/schema.sql" -confirm "$($archive schema migrate "$family/schema.sql" | sed -n 's/.*"confirm": "\([0-9a-f]*\)".*/\1/p' | head -n 1)" >/dev/null
ARCHIVE_RUN_TIMEOUT=600s $archive run "$family/seed.ts"
