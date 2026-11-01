FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod ./
COPY browser ./browser
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /browser ./browser

# Chromium from Debian, cut down to the files headless Chrome loads: its own directory without the Vulkan
# validation layers and other locales, the shared libraries it links (not Mesa, LLVM or GTK, which it never
# loads headless), NSS's runtime modules, fontconfig, and Noto for Latin, a few scripts, emoji and CJK.
FROM debian:trixie-slim AS chrome
RUN apt-get update && apt-get install -y --no-install-recommends chromium tini fonts-noto-core fonts-noto-color-emoji fonts-noto-cjk fontconfig
RUN set -e; out=/out; mkdir -p $out/usr/lib $out/etc $out/usr/share/fonts $out/var/cache; \
    cp -a /usr/lib/chromium $out/usr/lib/; \
    cd $out/usr/lib/chromium; rm -f libVk*; \
    find locales -type f ! -name en-US.pak -delete; \
    libs="$(for f in /usr/lib/chromium/chromium /usr/lib/chromium/chrome_crashpad_handler /usr/lib/chromium/lib*.so \
        /usr/lib/*-linux-gnu/libsoftokn3.so /usr/lib/*-linux-gnu/libfreeblpriv3.so /usr/lib/*-linux-gnu/libnssckbi.so; \
        do ldd $f | awk '/=> \//{print $3} /^\t\/lib/{print $1}'; done | sort -u)"; \
    for l in $libs /usr/lib/*-linux-gnu/libsoftokn3.so /usr/lib/*-linux-gnu/libfreeblpriv3.so /usr/lib/*-linux-gnu/libnssckbi.so; do \
        d=$out$(dirname $(readlink -f $l)); mkdir -p $d; cp -L $l $d/$(basename $l); done; \
    mkdir -p $out/usr/share; cp -a /etc/fonts $out/etc/; cp -a /usr/share/fontconfig $out/usr/share/; \
    mkdir -p $out/usr/share/glib-2.0/schemas; cp /usr/share/glib-2.0/schemas/gschemas.compiled $out/usr/share/glib-2.0/schemas/; \
    mkdir -p $out/usr/share/fonts/noto; \
    cp /usr/share/fonts/truetype/noto/NotoSans-*.ttf /usr/share/fonts/truetype/noto/NotoSerif-*.ttf /usr/share/fonts/truetype/noto/NotoSansMono-*.ttf \
       /usr/share/fonts/truetype/noto/NotoSansArabic-*.ttf /usr/share/fonts/truetype/noto/NotoSansHebrew-*.ttf \
       /usr/share/fonts/truetype/noto/NotoSansDevanagari-*.ttf /usr/share/fonts/truetype/noto/NotoSansThai-*.ttf \
       /usr/share/fonts/truetype/noto/NotoColorEmoji.ttf /usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc $out/usr/share/fonts/noto/; \
    cp /usr/bin/tini-static $out/tini; \
    fc-cache -f -y $out

FROM gcr.io/distroless/cc-debian13:nonroot
COPY --from=chrome /out/ /
COPY --from=build /browser /browser
ENV HOME=/tmp CHROME=/usr/lib/chromium/chromium
ENTRYPOINT ["/tini", "--", "/browser"]
