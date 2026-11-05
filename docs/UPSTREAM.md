# Upstream notes

This document describes how Freshness Guard could be contributed to Bifrost, which of Bifrost's extension points it uses, and which gaps the work revealed. Every statement about Bifrost was checked against the Bifrost source at tag `transports/v2.4.0` or against a spike run during development.

## 1. What an upstream contribution would be

There are two possible contributions, and they can be made one after the other.

1. A community plugin. Freshness Guard is a self-contained Go plugin (`plugin/`, built as `freshness-guard.so`). It can be listed in Bifrost's documentation as a plugin that operators build and load themselves. This needs no change in Bifrost.
2. Reusable pieces in Bifrost itself. Some of the gaps below are small changes to Bifrost that would help every plugin, not only this one. They are listed in section 4 and could be sent as separate, small pull requests.

## 2. Extension points used

| Extension point | What the plugin does with it | Where |
|---|---|---|
| `HTTPTransportPreHook` | Records the request headers that select the mode and policy (`x-bf-freshness`, `x-bf-freshness-policy`) and the virtual key, so that later hooks can read them from the context. | `plugin/main.go`, `internal/hooks/handler.go` |
| `PreLLMHook` | Records the prompt and the model of a chat request, streamed or not. The request passes through unchanged. | `plugin/main.go`, `internal/hooks/handler.go` |
| `PostLLMHook` | Submits the answer for the claim check. In sync mode the `heal` policy may replace the answer and the `block` policy may return an error. For a stream it collects the chunks and submits the answer on the last one. It also reads the cache entry ID of the response. | `plugin/main.go`, `internal/hooks/handler.go` |
| `HTTPTransportResponseHeadersHook` | Adds `X-Freshness-Status`, the score and the report ID to the response headers. | `plugin/main.go` |
| `placement: pre_builtin` | Makes the plugin's hooks run before the built-in semantic cache, so they also see answers that the cache serves. | `deploy/config.json` |
| Semantic cache metadata (`ExtraFields.CacheDebug`) | Reads the `CacheID` of a response, so that an entry can be removed when a fact in it becomes outdated. | `internal/hooks/handler.go` |
| `DELETE /api/cache/clear/{id}` | Removes a cached answer that contains an outdated claim. It is sent with the `X-Bifrost-Setup-Token` header. | `internal/cachepurge/client.go` |
| MCP gateway (`mcp.client_configs`) | Registers the plugin's `check_freshness` tool, so that any MCP client of the gateway can check a text. Bifrost lists the tool as `freshness_guard-check_freshness`. | `deploy/config.json`, `internal/mcp/server.go` |
| OpenAI-compatible endpoint (`/v1/chat/completions`) | Sends the plugin's own model calls (claim extraction, judging, healing) back through Bifrost, so they use its providers, keys and fallbacks. | `internal/llm/client.go` |

The plugin does not implement a stream-chunk hook. `PostLLMHook` runs once per chunk of a stream, and the plugin acts on the last one.

## 3. Compatibility

- Bifrost `transports/v2.4.0`, with `github.com/maximhq/bifrost/core` v1.14.1 (`go.mod`).
- Go 1.27.3, the version Bifrost v2.4.0 builds with (`ARG GO_IMAGE` in `deploy/Dockerfile`).
- Bifrost must be built as a dynamic binary, as described in its documentation for `make build DYNAMIC=1`. `deploy/Dockerfile` does this from source.
- A Go plugin only loads when it is built with the same Go version and the same version of every shared module as the Bifrost binary. `deploy/check-shared-versions.sh` stops the Docker build when a shared module differs.
- Go plugins load only on Linux and macOS, so the plugin is built and run in Docker.

## 4. Gaps found and proposed changes

| Gap | Evidence | Proposed change |
|---|---|---|
| A plugin cannot serve its own HTTP routes. Bifrost answers unknown paths with 404 before any plugin hook runs. | Spike S1: `HTTPTransportPreHook` was never called for `/freshness/*`. | Let a plugin register routes under a reserved prefix. Today the plugin starts a second HTTP server on its own port (default 8090), which an operator must publish and secure separately. |
| The official Bifrost image cannot load plugins. | Spike S2: the image is statically linked, and Go plugins need a dynamic binary. | Publish a dynamic image variant, or document the source build prominently. |
| A plugin cannot remove a semantic cache entry in-process. | `internal/cachepurge/client.go` calls `DELETE /api/cache/clear/{id}` over HTTP, and Bifrost accepts it only with the setup token or admin credentials. | Give plugins an in-process way to invalidate a cache entry, or a credential scoped to that call. |
| `env.` works only as a whole header value in `mcp.client_configs`. | A value such as `Bearer env.TOKEN` is not resolved. The plugin's MCP route therefore also accepts `X-Freshness-Token`. | Resolve `env.` inside a longer value. |
| MCP tools are injected into every chat request, including requests that a plugin makes to Bifrost itself. | Without `tool_manager_config.disable_auto_tool_inject: true`, the tool reaches the plugin's own extraction and judge calls. The e2e test `TestChatRequestsCarryNoMCPTools` checks that the setting keeps it out. | A per-request opt-out, so that a plugin's internal calls do not need a global switch. |

## 5. What a pull request would contain

The following describes the plan. Nothing has been sent to the Bifrost maintainers.

- Code. The plugin as a self-contained module. Its location in the Bifrost repository is a question for the maintainers.
- Config schema. An entry for the plugin's settings in `transports/config.schema.json`. The settings are read in `internal/config/config.go`.
- Documentation. A page for the Bifrost documentation site: what the plugin does, the headers, the configuration keys and the Docker build.
- Tests. Go unit tests, the end-to-end suite against a real Bifrost with fake providers, and the web tests.
- Before opening the pull request, read the contribution guide in the Bifrost repository and follow it.

Readiness checklist:

- [x] Unit tests, end-to-end tests and web tests exist and pass locally (see the README for the last results).
- [x] A continuous integration workflow exists (`.github/workflows/ci.yml`). It has not run yet because the repository has not been pushed with it.
- [x] The code is under the Apache License 2.0, the licence of Bifrost (`LICENSE`, `NOTICE`).
- [x] No breaking change: the plugin adds behaviour only when it is configured, and each hook returns the response unchanged on any error.
- [ ] The contribution guide of the Bifrost repository has been read and followed.
- [ ] The maintainers have agreed to where the code lives.
