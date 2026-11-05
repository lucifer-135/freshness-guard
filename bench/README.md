# Curated benchmark set

`curated.json` holds 51 labelled claims. Freshness Guard checks each one, and `freshbench` compares the verdict with the label. The set measures how often the detector flags an outdated claim and how often it wrongly flags a current one. It is small, so the numbers it produces are an indication, not a proof.

## Format

`curated.json` is a JSON array. Each item has these fields:

| Field | Meaning |
|---|---|
| `id` | A unique name, such as `pricing-netflix-standard`. |
| `category` | `pricing`, `version`, `leadership`, `law`, `statistic`, `event` or `feature`. The `static` category is refused, because a fact that never changes cannot be outdated. |
| `text` | One time-sensitive claim, written the way an assistant states it. It contains no date. |
| `label` | `outdated` if the claimed value held in the past and has since changed, `current` if it holds on `checked_on`. |
| `current_values` | Accepted spellings of today's value, for example `["$19.99"]`. |
| `changed_on` | The date the value last changed, as `YYYY-MM-DD`. It is required for outdated items and may be empty for current items. |
| `source` | An `https` link to an official page that shows today's value. |
| `checked_on` | The date the label was checked. |
| `note` | Optional. Used where a date or a source needs a caveat. |

The loader (`internal/bench/dataset.go`) rejects unknown fields, duplicate ids, empty texts and invalid dates. `TestCuratedDatasetIsValid` also requires 45 to 60 items, at least 20 of each label, at least 8 items each in `pricing`, `version` and `leadership`, `checked_on` on or after 2026-11-01, and no repeated text.

## How the labels were made

- Each item is about a different fact. The set has 27 outdated and 24 current items: 14 pricing, 14 version, 16 leadership, 6 law and 1 feature.
- Outdated items are claims that were true in 2025 or 2026 and changed after that. Current items are values that held on `checked_on`.
- The labels were checked on 6 November 2026 with ordinary web searches, restricted to official domains where possible (vendor pricing pages, release pages, government sites, company announcements). Tavily was not used, so the system under test does not grade its own sources.
- An item whose current value could not be confirmed was left out.
- Claude Code ran the searches and wrote the set. The developer has not yet checked the labels. See `docs/AI_USAGE.md`.
- The facts were chosen to be unlikely to change before 15 November 2026. Two exceptions are noted in the items: PostgreSQL 19 is planned for 19 November 2026, and the label of the PostgreSQL 17 claim holds either way.
- Several `changed_on` dates are the date of an announcement, not the day the change reached every customer. The `note` field says so.

## Running the benchmark

The benchmark runs on its own stack with a fresh database, so cached verdicts and the daily credit budget of a development stack do not change the result. From the repository root:

```bash
docker compose -f deploy/docker-compose.yml --profile bench up -d --build bifrost-bench
docker compose -f deploy/docker-compose.yml --profile bench run --rm freshbench curated -dataset bench/curated.json -out bench/results -name curated -concurrency 2 -pace 30s
```

`deploy/.env` must hold `FG_API_TOKEN` and the keys of the models the stack uses. `-pace 30s` starts one check every 30 seconds. It is needed on Groq's free tier, which allows 8000 tokens per minute for the default models: without it, a run returns rate-limit errors for most items, as the first run in `bench/results/` (`curated-2026-11-05`) did. A run takes about 25 minutes. A full run spends Tavily credits, so check the remaining budget first. A result is written to `bench/results/curated-DATE.json` and `.md` and is never overwritten; a second run on the same day gets `-2`.

## Correcting a label and re-scoring

If a label turns out to be wrong, fix it in `curated.json`, then re-score the stored reports. This spends no credits and sends no requests:

```bash
docker compose -f deploy/docker-compose.yml --profile bench run --rm freshbench score -results bench/results/curated-DATE.json -dataset bench/curated.json -out bench/results
```

The result is written next to the original as `curated-rescored-DATE.json` and `.md`. The original run is not changed, so both numbers stay on record.

## How a text is scored

A text counts as predicted outdated when at least one of its checked claims is `STALE` or `WRONG`; otherwise it counts as predicted current. Positive means outdated. Precision, recall and F1 follow from the confusion matrix. A check that ends in an error is listed and left out of the metrics. A rate whose denominator is zero is shown as `n/a`.

Targets from the design specification are written as targets: precision of at least 0.8 on this set, and async mode adding less than 2 ms at the 99th percentile. The result files show what a run actually measured.
