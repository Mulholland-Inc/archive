FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod ./
COPY browser ./browser
RUN CGO_ENABLED=0 go build -trimpath -ldflags=-s -o /browser ./browser

FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends chromium tini ca-certificates fonts-noto-core fonts-noto-color-emoji fonts-noto-cjk \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /browser /usr/local/bin/browser
USER nobody
ENV HOME=/tmp
ENTRYPOINT ["tini", "--", "browser"]
