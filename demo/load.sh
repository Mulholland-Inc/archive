#!/bin/sh
set -eu
demo=$(dirname "$0")
archive=${ARCHIVE_BIN:-archive}

$archive space list >/dev/null 2>&1 || $archive user add "${1:-me@example.com}" -name Me -admin >/dev/null
if [ "$($archive list person -limit 1 2>/dev/null || echo '[]')" != "[]" ]; then
	echo "demo: this archive already has people in it; load the demo into an empty one" >&2
	exit 1
fi

schema=$(mktemp -d)
trap 'rm -rf "$schema"' EXIT
$archive schema pull "$schema" >/dev/null
cp "$demo"/schema/*.sql "$schema"
$archive schema apply "$schema" >/dev/null

$archive space add family >/dev/null
$archive user add sanne@example.com -name "Sanne de Vries" >/dev/null
$archive space grant family sanne@example.com editor >/dev/null

$archive file add "$demo"/files/*.* >/dev/null
$archive file add -space family "$demo"/files/family/* >/dev/null

ARCHIVE_RUN_TIMEOUT=600s $archive run "$demo/seed.ts"

for v in \
	'{"name": "Task board", "type": "task", "layout": "board", "by": "status"}' \
	'{"name": "Family", "type": "person", "where": {"relation": "family"}}' \
	'{"name": "Doctors", "type": "person", "where": {"relation": "doctor"}}' \
	'{"name": "Rent", "type": "payment", "where": {"category": "rent"}}' \
	'{"name": "Renewals", "type": "subscription", "layout": "calendar", "by": "renews"}' \
	'{"name": "Things by category", "type": "possession", "layout": "board", "by": "category"}'; do
	$archive view add "$v" >/dev/null
done
$archive view add -space family '{"name": "Trips", "type": "trip", "layout": "calendar", "by": "starts"}' >/dev/null
echo "demo: loaded; restart archive serve if it was running, so it sees the new tables" >&2
