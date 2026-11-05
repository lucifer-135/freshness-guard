---
title: Freshness Guard
sdk: docker
app_port: 8090
license: apache-2.0
short_description: Checks LLM answers in Bifrost for outdated claims
---

Freshness Guard is a plugin for the open-source Bifrost LLM gateway. It finds claims in an answer that may be outdated, checks them against live web sources with Tavily, and scores each claim.

This Space runs Bifrost with the plugin and serves the web app. Sign in with the judge token from the submission form. The source code, setup instructions and architecture overview are at https://github.com/lucifer-135/freshness-guard.
