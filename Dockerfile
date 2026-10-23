FROM node:26-alpine AS web
WORKDIR /web
COPY apps/web/package.json apps/web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY apps/web/ ./
RUN npx vite build --outDir /dist --emptyOutDir

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY src/go.mod src/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY src/ ./
COPY --from=web /dist ./web/dist
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /archive .

FROM alpine:3.24
RUN apk add --no-cache postgresql18 postgresql-pgvector && install -d -o postgres -g postgres -m 700 /data
COPY --from=build /archive /usr/local/bin/archive

ENV ARCHIVE_DATA=/data \
    ARCHIVE_LLM_URL=http://host.docker.internal:11434/v1 \
    ARCHIVE_LOCALE=

USER postgres
VOLUME /data
EXPOSE 8080
STOPSIGNAL SIGTERM
HEALTHCHECK --interval=10s --timeout=5s --start-period=60s --retries=3 \
    CMD ["wget", "-q", "-O", "/dev/null", "http://127.0.0.1:8080/healthz"]

ENTRYPOINT ["archive"]
CMD ["serve", "-addr", ":8080"]
