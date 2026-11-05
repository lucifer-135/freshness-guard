# syntax=docker/dockerfile:1
# Builds Bifrost from source together with the Freshness Guard plugin. Bifrost
# is built exactly like its official dynamic build (`make build DYNAMIC=1`,
# documented at docs.getbifrost.ai/plugins/building-dynamic-binary), because
# custom Go plugins only load into a dynamically linked Bifrost; the official
# image is statically linked and cannot load them. The plugin is built
# separately with the same toolchain, and a check stops the build when the two
# would compile a shared module at different versions.
#
# The Bifrost source comes from the build context "bifrost-src": a sparse
# checkout of tag transports/v2.4.0 (folders transports/ and ui/). docker compose
# passes the checkout made on the host by deploy/fetch-bifrost.sh, because
# cloning inside the build was slow and failed on large transfers there. A build
# without that context, such as the one Hugging Face Spaces runs, uses the
# bifrost-clone stage below instead.
ARG GO_IMAGE=golang:1.27.3-trixie

# A "bifrost-src" build context replaces the stage of the same name, so local
# builds do not clone.
FROM ${GO_IMAGE} AS bifrost-clone
ARG BIFROST_TAG=transports/v2.4.0
RUN git -c core.autocrlf=false clone --depth 1 --filter=blob:none --sparse --branch "$BIFROST_TAG" \
      https://github.com/maximhq/bifrost.git /src \
 && git -C /src sparse-checkout set transports ui \
 && rm -rf /src/.git

FROM scratch AS bifrost-src
COPY --from=bifrost-clone /src/ /

FROM node:25-alpine AS ui
WORKDIR /app
COPY --from=bifrost-src ui/package*.json ./
RUN npm ci
COPY --from=bifrost-src ui/ ./
RUN npm run build-enterprise

# The Freshness Guard web app, embedded into the plugin with go:embed.
FROM node:24-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json web/.npmrc ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM ${GO_IMAGE} AS builder
ENV CGO_ENABLED=1 GOOS=linux GOWORK=off
WORKDIR /build
COPY --from=bifrost-src transports ./bifrost/transports
COPY --from=ui /app/out ./bifrost/transports/bifrost-http/ui
# Bifrost is built before the plugin sources are copied, so a change to the
# plugin rebuilds only the plugin.
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd bifrost/transports/bifrost-http && go build -ldflags="-w -s -X main.Version=v2.4.0" -a -trimpath -o /out/bifrost-http .
COPY go.mod go.sum ./fg/
COPY deploy/check-shared-versions.sh ./fg/deploy/
COPY plugin ./fg/plugin
COPY internal ./fg/internal
COPY --from=web /web/dist/ ./fg/internal/webui/static/
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    bash fg/deploy/check-shared-versions.sh bifrost/transports/bifrost-http fg/plugin
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    cd fg && go build -trimpath -buildmode=plugin -o /out/freshness-guard.so ./plugin

FROM debian:trixie-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /out/bifrost-http /app/bifrost-http
COPY --from=builder /out/freshness-guard.so /app/plugins/freshness-guard.so
EXPOSE 8080 8090
ENTRYPOINT ["/app/bifrost-http"]
CMD ["-app-dir", "/app/data", "-host", "0.0.0.0", "-port", "8080", "-log-style", "pretty"]

# The image for Hugging Face Spaces, which builds the last stage of a Dockerfile.
# A Space publishes one port, so only the plugin API (8090, protected by the
# judge token) listens on all interfaces; Bifrost keeps to loopback. Spaces run
# the container as user 1000.
FROM runtime AS hosted
COPY deploy/hosted/config.json /app/data/config.json
RUN chown -R 1000:1000 /app/data
USER 1000
EXPOSE 8090
CMD ["-app-dir", "/app/data", "-host", "127.0.0.1", "-port", "8080", "-log-style", "pretty"]
