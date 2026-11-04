#!/bin/bash
# Installs the render toolchain on a fresh pod. Piped over ssh by `motion up`. Exits 3 if this host refuses NVENC.
set -eo pipefail
export DEBIAN_FRONTEND=noninteractive
CHROME=157.0.8103.38
NODE=22.20.0
FFMPEG=n8.1-latest-linux64-gpl-8.1
start=$(date +%s)
say() { echo "  $1 at $(($(date +%s) - start))s"; }
mkdir -p /opt/node /opt/ff /opt/chrome /w /cache/chunks

curl -fsSL "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-$FFMPEG.tar.xz" | tar -xJ -C /opt/ff --strip-components=2 --wildcards '*/bin/ffmpeg' '*/bin/ffprobe'
say ffmpeg
if ! /opt/ff/ffmpeg -hide_banner -loglevel error -y -f lavfi -i testsrc2=s=1920x1080:r=60 -frames:v 5 -pix_fmt yuv420p -c:v h264_nvenc /tmp/nvenc.mp4 2>/tmp/nvenc.log; then
  echo "  nvenc refused: $(grep -m1 -E 'OpenEncodeSession|Cannot|driver' /tmp/nvenc.log)"
  [ -n "$MOTION_CPU" ] || exit 3
else
  say nvenc-ok
fi

(curl -fsSL "https://nodejs.org/dist/v$NODE/node-v$NODE-linux-x64.tar.xz" | tar -xJ -C /opt/node --strip-components=1; say node) &
a=$!
(curl -fsSL -o /tmp/chrome.zip "https://storage.googleapis.com/chrome-for-testing-public/$CHROME/linux64/chrome-linux64.zip"; say chrome-download) &
c=$!
echo force-unsafe-io > /etc/dpkg/dpkg.cfg.d/unsafe
apt-get update -qq
apt-get install -y -qq --no-install-recommends rsync unzip libegl1 libgles2 libgl1 libglvnd0 libglx0 libopengl0 libnss3 libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 libxkbcommon0 libxcomposite1 libxdamage1 libxfixes3 libxrandr2 libgbm1 libasound2 libpango-1.0-0 libcairo2 >/dev/null 2>&1
say apt
wait $a
wait $c
unzip -q /tmp/chrome.zip -d /opt/chrome
rm /tmp/chrome.zip
say done
