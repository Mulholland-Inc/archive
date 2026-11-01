#!/bin/sh
set -eu
demo=$(dirname "$0")
archive=${ARCHIVE_BIN:-archive}
me=${1:-me@example.com}

$archive list member -limit 1 >/dev/null 2>&1 || $archive user add "$me" -name "${2:-Me}" >/dev/null
if [ "$($archive list person -limit 1 2>/dev/null || echo '[]')" != "[]" ]; then
	echo "demo: this archive already has people in it; load the demo into an empty one" >&2
	exit 1
fi

$archive schema migrate "$demo/schema.sql" >/dev/null

member() { $archive list member email="$1" | sed -n 's/.*"id": "\([^"]*\)".*/\1/p'; }
$archive user add sam@example.com -name "Sam Rivera" -role editor >/dev/null
$archive set member "$(member "$me")" '{"groups":["family"]}' >/dev/null
$archive set member "$(member sam@example.com)" '{"groups":["family"]}' >/dev/null

$archive file add "$demo"/files/*.* >/dev/null
$archive file add -access family "$demo"/files/family/* >/dev/null

$archive run "$demo/seed.ts"

echo "demo: loaded; restart archive serve if it was running, so it sees the new tables" >&2
