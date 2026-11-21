---
name: benchmark-reader
metadata:
  author: Mohamed
description: >
  Read and interpret output of the Leoxy benchmark client (test/benchmark/cmd).
  Validates the run, compares direct vs via-Leoxy and with vs without database
  runs, computes proxy overhead, detects saturation with Little's law, classifies
  the bottleneck, and returns a short verdict with next actions. Use when the user
  pastes benchmark output ("Leoxy Network", "Throughput", "Latency p50/p95/p99"),
  asks what the numbers mean, asks whether Leoxy or the backend is the bottleneck,
  or wants a before/after comparison of two runs.
---

# Benchmark Reader (Leoxy)

Read-only analysis. Do not modify code. Follow `AGENTS.md`. Plain hyphens, no emojis.
Every claim must be backed by a number from the pasted output. If a number is missing, say so.

## Input

One or more runs, each labelled by the user or inferable from the target URL and port.
Per run extract: target URL, duration, concurrency, total, accepted, rejected,
throughput (req/s), response throughput (MB/s), status codes, p50, p95, p99, max.
Ignore the "Client Memory" block for proxy conclusions: it describes the benchmark
process, not Leoxy.

If labels are unclear, infer: port 80 = via Leoxy, port 9000 (or the upstream port) = direct.
Ask one question only if the pairing is still ambiguous.

## Workflow

1. Validate each run (step below). Mark invalid runs and exclude them from comparisons.
2. Normalize: convert latencies to ms, compute achieved req/s = total / duration.
3. Saturation check per run (Little's law).
4. Pair runs into comparisons and compute deltas.
5. Classify the bottleneck.
6. Report.

## 1. Validate a run

- Rejected > 0 or non-2xx/3xx codes: report error rate and which codes; 429 means a rate limit
  was hit (Leoxy), 502/504 means upstream failure, `error` bucket means client or transport failure.
- Stop condition differs: run ended by duration vs by `-request` limit. Note it; rates stay comparable,
  totals do not.
- Different concurrency, method, body, headers, or endpoint between paired runs: comparison is invalid.
- Client, Leoxy, and upstream on the same host: results are CPU-contended. State it as a caveat.
- Duration under 30s: treat tail percentiles (p99, max) as unreliable.
- `max` far above p99 (for example > 3x): treat as a single outlier (GC pause, connection setup,
  Atlas hiccup), not a trend.

## 2. Saturation check

The client is closed-loop: each worker sends the next request only after the previous one returns.

- Expected throughput ~= concurrency / mean latency (use p50 as an approximation).
- If observed throughput is within ~20 percent of that value, the system is saturated:
  adding concurrency only raises latency, it does not raise throughput.
- A tight p50-to-p99 spread (p99 / p50 < ~1.5) under saturation points to a fixed ceiling
  (pool size, rate limit, DB or plan throttle) rather than random noise.
- A wide spread (p99 / p50 > ~3) points to queueing, GC, lock contention, or noisy neighbours.

Because of closed-loop coordinated omission, tail latency is under-reported when the system stalls.
Say this when p99 looks good but throughput is low.

## 3. Pair and compare

Useful pairs:

| Pair | Question it answers |
| --- | --- |
| via Leoxy vs direct, same endpoint | Cost of the proxy |
| DB endpoint vs no-DB endpoint, same path (direct) | Cost of the database |
| DB endpoint via Leoxy vs direct | Does the proxy matter once the DB dominates |

Formulas:

- throughput delta % = (via - direct) / direct * 100
- latency delta (ms) per percentile = via - direct
- latency delta % = delta / direct * 100
- retention % = via throughput / direct throughput * 100
- DB share of latency = (DB p50 - no-DB p50) / DB p50 * 100

Interpretation defaults (adjust if the user gives a budget):

- Delta inside +/- 2 percent on throughput and p50: no measurable proxy cost.
- Retention below ~85 percent or p50 added above a few ms on a same-host run: investigate the proxy
  hot path (locks, logging, per-request allocations, client IP parsing, extra hops).
- Under saturation, added latency includes queueing from reduced throughput. It is not the proxy's
  pure service time. To measure pure overhead, recommend a fixed-rate (open-loop) run at 30-50 percent
  of the direct maximum and compare again.

## 4. Classify the bottleneck

| Signal | Verdict |
| --- | --- |
| DB run: latency in hundreds of ms, throughput equal direct and via Leoxy | Backend or database bound, proxy irrelevant |
| No-DB run: via Leoxy throughput clearly lower than direct | Proxy or shared-host CPU bound |
| Direct no-DB throughput low and p50 > ~20 ms at concurrency 100 | Upstream (for example single-threaded Node) bound |
| 429 or 503 present | Rate limiter or limiter store (Redis) |
| 502 or 504 present | Upstream health, timeout, or load-balancer failover |
| Throughput flat while concurrency grows | Hard ceiling downstream: pool, plan limit, lock |

When the DB dominates, name what to check next instead of guessing: query plan (`explain("executionStats")`),
index on filter plus sort fields, driver pool size and wait queue, region and RTT to the cluster,
cluster tier throttling, response caching for hot read endpoints.

## 5. Report format

Keep it short. No raw restatement of the pasted output.

1. One-line verdict per question asked (proxy cost, bottleneck).
2. Comparison table: metric | direct | via Leoxy | delta | delta %.
3. Saturation line: observed vs expected throughput and what it implies.
4. Caveats that apply (same host, different stop condition, coordinated omission, outlier max).
5. Next actions, at most 5, ordered by expected impact, each with the measurement that proves it worked.

If the user supplies a previous report or baseline, add a before/after table and state whether each
metric improved, regressed, or stayed within noise (use +/- 5 percent as the default noise band
unless repeated runs show otherwise). Recommend at least 3 repeated runs before claiming an improvement
under 10 percent.

## Reference baseline (2026-11-20, v1.15.1, Windows, client + Leoxy + upstream on one host, concurrency 100)

| Run | req/s | p50 | p95 | p99 | max |
| --- | ---: | ---: | ---: | ---: | ---: |
| /api/user/all via Leoxy (MongoDB Atlas) | 142.82 | 844.9 ms | 1.014 s | 1.143 s | 4.513 s |
| /api/user/all direct (MongoDB Atlas) | 142.79 | 851.6 ms | 1.014 s | 1.138 s | 4.023 s |
| /api/health via Leoxy (no DB) | 3107.81 | 30.0 ms | 38.8 ms | 95.2 ms | 264.1 ms |
| /api/health direct (no DB) | 4365.73 | 21.0 ms | 29.0 ms | 68.1 ms | 183.0 ms |

Reading of the baseline: the DB path is backend bound (proxy delta is noise). The no-DB path shows
about -29 percent throughput and +9 ms p50 through the proxy on a contended single host, which is the
number to improve and re-measure.

## Rules

- Do not claim the proxy is "fast" or "slow" without a paired direct run.
- Do not use the Client Memory block to judge Leoxy memory. Point to `/metrics` (`leoxy_memory_bytes`)
  or a heap profile via `-pprof` instead.
- Do not recommend code changes unless asked; recommend what to measure and where to look.
