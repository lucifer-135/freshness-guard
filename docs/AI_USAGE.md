# AI usage

This document states where Freshness Guard uses AI models, how AI tools were used to build it, and what data leaves the machine.

## 1. AI inside the product

Freshness Guard calls language models through Bifrost's OpenAI-compatible endpoint, so the operator chooses the providers and keys. The defaults below are in `internal/config/config.go`, and `deploy/config.json` sets the same models.

| Role | Default model (fallback) | What it does |
|---|---|---|
| Claim extraction | `groq/openai/gpt-oss-20b` (`gemini/gemini-3.5-flash-lite`) | Reads an answer and lists its time-sensitive claims with the entity, attribute, value and category of each. |
| Judge | `groq/openai/gpt-oss-120b` (`gemini/gemini-3.5-flash`) | Reads the claims of one answer together with the search results for them and returns a verdict (`FRESH`, `STALE`, `WRONG` or `UNVERIFIABLE`), the current value, a confidence and a short rationale. |
| Heal | `groq/openai/gpt-oss-120b` (`gemini/gemini-3.5-flash`) | Rewrites only the outdated claims of an answer when the `heal` policy is on. |
| Semantic-cache embeddings | `gemini-embedding-001` | Embeds prompts for Bifrost's own semantic cache. This is a setting of Bifrost's cache plugin, not of Freshness Guard. |

Safeguards:

- Structured output. Extraction and judging ask for JSON that matches a schema. A reply that does not match gets one repair request. A second failure ends the check as an error, and the answer passes through unchanged (`internal/llm/client.go`).
- Judging from evidence. The judge prompt tells the model to decide only from the search results it is given and not to use other knowledge (`internal/verify/prompt.go`). A claim whose evidence is not enough is marked `UNVERIFIABLE`, not guessed.
- A diff guard on rewrites. A rewritten answer is accepted only when at most 10 percent of the characters outside the outdated claims changed. Otherwise the plugin falls back to inline annotations (`internal/heal/guard.go`).
- The plugin does not check its own calls. Its model calls carry `x-bf-freshness: off` and a unique cache key, and they are not stored in the semantic cache (`internal/llm/client.go`).
- Failures do not break the gateway. Every hook recovers from panics and returns the response unchanged on an error, with `X-Freshness-Status: error`.
- The accuracy numbers in the README come from `bench/results/` and describe one small set whose labels Claude Code proposed from official sources and the developer has not yet reviewed (see section 2). They are not a general accuracy claim.

## 2. AI used to build the project

> This section is a draft for the developer to review and correct before submission.

Claude Code, which uses Anthropic's Claude models, helped build this project under the developer's direction. It was used for:

- research on Bifrost's plugin interfaces, Tavily's API and the hackathon requirements;
- the design specification, the implementation plans and the architecture;
- most of the Go and TypeScript code, and the tests written before it;
- the Docker, CI and deployment files, and the documentation, including this file;
- the curated benchmark set in `bench/curated.json`: Claude Code searched the web, using official sources where possible, and proposed each label together with its source link.

The developer chose the problem statement and the free-tier services, set the rules for the work (no paid services, no AI attribution in commits, planning documents kept out of the repository), decided how the project is hosted and submitted, and runs the final steps: pushing, publishing, recording the demo video and entering the submission.

*To be completed by the developer:* which parts they reviewed line by line, which labels in the curated set they spot-checked, and anything they changed.

## 3. What data leaves the machine

- Answer text goes to the model providers configured in Bifrost (in the default setup Groq and Google Gemini) for extraction, judging and rewriting. Providers handle it under their own terms.
- Search queries go to Tavily. A query is built from a claim (for example the entity and attribute), not from the whole answer.
- Nothing else is sent by the plugin. It keeps its data (reports, facts, credit use) in a local SQLite database, `freshness.db`, and it has no telemetry.
- The web app's chat page sends the text that the user types to the model that the user picks, through Bifrost.
