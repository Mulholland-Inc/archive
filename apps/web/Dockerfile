FROM --platform=$BUILDPLATFORM node:26-alpine AS build
WORKDIR /w/apps/web
COPY apps/web/package.json apps/web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY apps/kit/ /w/apps/kit/
COPY apps/site/static/art/ /w/apps/site/static/art/
COPY apps/web/ ./
RUN npm run build

FROM nginx:1.29-alpine-slim
COPY apps/web/nginx.conf /etc/nginx/conf.d/default.conf
COPY apps/web/api.sh /docker-entrypoint.d/40-archive-api.sh
COPY --from=build /w/apps/web/dist/ /usr/share/nginx/html/
ENV ARCHIVE_API_URL=http://localhost:8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=5s --retries=3 CMD ["wget", "-q", "-O", "/dev/null", "http://127.0.0.1:8080/"]
