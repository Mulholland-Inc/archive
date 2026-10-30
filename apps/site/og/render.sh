#!/bin/sh
set -eu
cd "$(dirname "$0")"
chrome="${CHROME:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
static=../static
mark=$static/favicon.svg
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

"$chrome" --headless --disable-gpu --hide-scrollbars --allow-file-access-from-files \
  --window-size=1200,630 --virtual-time-budget=5000 \
  --screenshot="$tmp/og.png" "file://$PWD/card.html" 2>/dev/null
magick "$tmp/og.png" -strip "$static/og.png"

rsvg-convert -w 1024 -h 1024 "$mark" -o "$tmp/mark.png"

magick "$tmp/mark.png" -define icon:auto-resize=48,32,16 "$static/favicon.ico"

icon() { magick -size "$1x$1" xc:'#f7f8fa' \( "$tmp/mark.png" -resize "$2x$2" \) -gravity center -composite -strip "$3"; }
icon 180 120 "$static/apple-touch-icon.png"
icon 192 128 "$static/icon-192.png"
icon 512 340 "$static/icon-512.png"
