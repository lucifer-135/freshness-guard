# syntax=docker/dockerfile:1
# Builds Bifrost from source together with the Freshness Guard plugin. Both are
# compiled in one Go workspace with the same toolchain and flags, because Go
# plugins only load when every shared package matches exactly. The official
# Bifrost image cannot be used: it is statically linked and cannot load plugins.
#
# The Bifrost source comes from the extra build context "bifrost-src": a sparse
# checkout of tag transports/v2.4.0 (folders transports/ and ui/) made on the
# host by deploy/fetch-bifrost.sh. Cloning inside the build was too slow and
# failed on large transfers.
ARG GO_IMAGE=golang:1.27.3-trixie

FROM node:25-alpine AS ui
WORKDIR /app
COPY --from=bifrost-src ui/package*.json ./
RUN npm ci
COPY --from=bifrost-src ui/ ./
RUN npm run build-enterprise

FROM ${GO_IMAGE} AS builder
ENV CGO_ENABLED=1 GOOS=linux
WORKDIR /build
COPY --from=bifrost-src transports ./bifrost/transports
COPY --from=ui /app/out ./bifrost/transports/bifrost-http/ui
COPY go.mod go.sum ./fg/
COPY plugin ./fg/plugin
COPY internal ./fg/internal
RUN go work init ./bifrost/transports ./fg
RUN cd bifrost/transports && go build -trimpath -tags sqlite_static -o /out/bifrost-http ./bifrost-http
RUN cd fg && go build -trimpath -tags sqlite_static -buildmode=plugin -o /out/freshness-guard.so ./plugin

FROM debian:trixie-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /out/bifrost-http /app/bifrost-http
COPY --from=builder /out/freshness-guard.so /app/plugins/freshness-guard.so
EXPOSE 8080 8090
ENTRYPOINT ["/app/bifrost-http"]
CMD ["-app-dir", "/app/data", "-host", "0.0.0.0", "-port", "8080", "-log-style", "pretty"]
