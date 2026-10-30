#!/bin/sh
set -eu
cd "$(dirname "$0")"
for size in 120 215 512 1024; do
  rsvg-convert -w "$size" -h "$size" icon.svg -o "icon-$size.png"
done
rsvg-convert -w 1024 -h 1024 icon-dark.svg -o icon-dark-1024.png
rsvg-convert -w 1024 -h 1024 mark.svg -o mark.png
rsvg-convert -w 1024 -h 1024 mark-white.svg -o mark-white.png
rsvg-convert -w 2000 wordmark.svg -o wordmark.png
rsvg-convert -w 2000 wordmark-white.svg -o wordmark-white.png
