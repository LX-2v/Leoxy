---
name: security-audit
metadata:
  author: Mohamed
description: >
  Read-only security audit of the Leoxy Go reverse proxy (HTTP/WS/SSE, TCP, UDP).
  Runs Go security tooling and Podman-based container scans, reviews each attack
  surface manually, verifies findings, writes a versioned markdown report, and
  compares it with the previous audit (new, fixed, persisting, regressed). Use
  when the user asks for a security audit, vulnerability review, pentest-style
  review, pre-release security check, or a comparison between audit versions.
---

# Security Audit (Leoxy)

Audit as a senior application security engineer. Findings must be verified by
tracing the code path. No generic advice, no padding.

## Rules

- Read-only. Do not modify, rename, or delete existing files.
- Only files you may create: `docs/security-audit/SECURITY_AUDIT-v<version>-<YYYY-MM-DD>.md`
  and `docs/security-audit/LATEST.md` (copy of the newest report).
- Never read or print `.env` files or secrets. Redact secret-looking values.
- Follow `AGENTS.md`. No destructive git or shell commands.
- Use Podman, never Docker. Rootless by default. If a command needs network or
  images that are unavailable, mark the step "Skipped" with the reason.
- Every finding cites `file:line` and a concrete attack scenario. If the exploit
  path cannot be traced, mark it "Unverified" or drop it.
- No emojis. Plain hyphens only.

## Workflow

1. Baseline and previous report (Phase 0)
2. Tooling, including Podman (Phase 1)
3. Manual review per surface (Phase 2)
4. Verify and rate (Phase 3)
5. Write the report (Phase 4)
6. Compare with the previous version (Phase 5)

### Phase 0 - Baseline

- Read `AGENTS.md`, `CONTRIBUTING.md`, `security.md`, `app/internal/core/run.go`,
  `app/internal/file/config.go`.
- Record: current version (`version.json`), commit (`git rev-parse HEAD`), date
  (`date +%F`), Go version, Podman version (`podman --version`).
- List entrypoints, listeners, middleware order, trust boundaries
  (client -> Leoxy -> upstream, config/env -> Leoxy, Leoxy -> Redis/webhook).
- Find the previous report: newest file in `docs/security-audit/` other than the
  one you are about to write. Read its commit, version, finding IDs, and status.
  If none exists, this is the baseline audit and Phase 5 is skipped.
- If a previous report exists, compute what changed:
  `git diff --stat <prev_commit>..HEAD -- app docker script .github`
  and list changed security-relevant files first for Phase 2.

### Phase 1 - Tooling

Run what is installed, skip and note what is not.

Go (from `app/`):
- `govulncheck ./...`
- `go vet ./...`
- `go test -race ./...`
- `gosec ./...` and `staticcheck ./...` if available
- `go list -m -u all` for outdated modules

Podman (repo root):
- Build: `podman build -f docker/stable/Dockerfile -t leoxy:audit .`
  (also `docker/alpine/Dockerfile` and `docker/build/Dockerfile`; record build failures as findings)
- Rootless check: `podman info --format '{{.Host.Security.Rootless}}'`
- Image scan: `podman run --rm -v "$PWD:/src:ro,Z" docker.io/aquasec/trivy:latest fs /src`
  and `podman run --rm -v /run/user/$(id -u)/podman/podman.sock:/var/run/docker.sock docker.io/aquasec/trivy:latest image leoxy:audit`
  (use the socket path from `podman info` if different)
- Dockerfile lint: `podman run --rm -i docker.io/hadolint/hadolint < docker/stable/Dockerfile`
- Secret scan: `podman run --rm -v "$PWD:/repo:ro,Z" docker.io/zricethezav/gitleaks:latest detect --source /repo --no-git=false`
- Runtime hardening check with the stable image: run with
  `--read-only --cap-drop=ALL --security-opt no-new-privileges --user 10001:10001 --tmpfs /tmp`
  and confirm `/health/live`, `/ping`, non-root UID, and that writes to `/app/leoxy` behave as documented.
  Reuse `docker/stable/podman.sh leoxy:audit` for the smoke test.
- Compose: `podman compose -f docker/stable/docker-compose.yml config`
  (fall back to `podman-compose`). Note the repo scripts `script/docker.sh` and
  `docker/stable/docker.sh` call `docker`; flag any pipeline step that assumes Docker.
- Remove audit containers and images afterwards: `podman rm -f` and `podman rmi leoxy:audit`.

Triage tool output into real findings vs false positives. Keep raw summaries short.

### Phase 2 - Manual review

Review changed files from Phase 0 first, then the rest.

1. Auth and access (`security/security.go`, `internal.go`): API key and JWT (alg
   confusion, exp/nbf/iss/aud, timing), check order (CIDR, method, path, auth),
   mTLS enforcement, `path.Clean` vs mux routing (trailing slash, encoded paths,
   double slashes, case) bypassing `allowed_paths` or auth, per-upstream vs
   server policy gaps.
2. Client IP and headers (`common/ip.go`, `proxy/load_balancer.go`):
   X-Forwarded-For, X-Real-IP, CF-Connecting-IP spoofing, trusted-proxy logic,
   X-Forwarded-Host/Proto trust, hop-by-hop and Connection headers, request
   smuggling, header injection, Host abuse, leakage of Authorization, cookies,
   X-API-Key upstream.
3. Internal endpoints (`core/run.go`): `/metrics`, `/health/ready`, `/leoxy`,
   `/debug/pprof/*` - confirm each is behind `InternalOnly`; empty
   `internal_cidrs`; loopback behind another proxy.
4. DoS and resources: rate limiter (bucket cap behavior when full, IPv6 /64
   grouping, global tokens consumed by rejected requests, mutex hold time), body
   limit (chunked, Expect 100-continue, buffering of unknown-length bodies),
   timeouts, slowloris, SSE/WebSocket deadline clearing, MaxHeaderBytes,
   connection limits, goroutine/FD leaks, logger queue backpressure, log growth.
5. Redis client (`security/redis.rate.limit.go`): hand-rolled RESP, key injection,
   reply parsing, pool races, no TLS, password handling, fail-open consequences.
6. TCP/UDP (`http_server/TCP`, `UDP`, `shared`): policy bypass, UDP spoofing and
   amplification, session table limits, per-IP accounting leaks, deadlines,
   default listen address.
7. Config and secrets (`file/*.go`, `config/*.yaml`, examples): insecure
   defaults, file permissions, env override precedence, secrets in YAML or logs,
   validation gaps (unenforced `limit_request`, zero or negative values silently
   disabling protections).
8. Watcher (`internal/watcher`): webhook SSRF (redirects, internal addresses),
   report and heap profile contents, file permissions, path handling.
9. Logging: log injection, PII/secret/query leakage, request ID handling, error
   responses leaking internals.
10. Process management (`app/cli`, `core/process_*`): PID file races, wrong-PID
    kill, symlink/TOCTOU, permissions on `leoxy/keys` and `leoxy/log`.
11. Upstream TLS (`shared/index.go`): verification, MinVersion, redirects,
    upstream URL validation.
12. Containers and deploy (`docker/*`, `script/nsm/*`, compose): root user,
    missing user or tools in final stage, capabilities, read-only FS, secrets in
    env/compose, unpinned base images, systemd/launchd hardening, published
    ports, Podman rootless compatibility (low ports, volume labels `:Z`, UID mapping).
13. CI and supply chain (`.github/workflows`, dependabot): actions pinned by tag
    vs SHA, `@latest` installs, token permissions, secret exposure, release flow
    abuse via `version.json`, fork PRs.
14. `test/` and `script/`: committed webhook URLs, credentials, insecure samples
    users may copy.

### Phase 3 - Verify and rate

- Reproduce each candidate by reading the code path and existing tests. Note if a
  test already covers it.
- Severity: Critical, High, Medium, Low, Info, with reasoning on exploitability,
  preconditions, impact.
- Separate "vulnerable by default" from "unsafe if misconfigured".
- Assign stable IDs: `LX-SEC-001`, `LX-SEC-002`, ... If a previous report exists,
  reuse the previous ID for the same issue (match by location and root cause);
  only new issues get new numbers.

### Phase 4 - Report

Write `docs/security-audit/SECURITY_AUDIT-v<version>-<date>.md`, then copy it to
`docs/security-audit/LATEST.md`. Structure:

1. Summary: scope, version, commit, date, tools run and skipped (including Podman
   steps), counts by severity.
2. Mermaid flow diagram of trust boundaries with finding IDs placed on it.
3. Findings table: ID | Severity | Title | Location | Default-exposed (Y/N) | Status vs previous.
4. Per finding: description and `file:line`, attack scenario (steps), impact,
   root cause, fix (minimal Go snippet allowed), regression test (name and
   assertion), effort (S/M/L).
5. Hardening recommendations (not vulnerabilities).
6. Positive observations.
7. Prioritized remediation roadmap (before next release).
8. False positives rejected, with reasons.
9. Version comparison (Phase 5).

### Phase 5 - Compare with previous version

Skip with a note if no previous report. Otherwise add section 9:

- Header table: previous version/commit/date vs current version/commit/date, and
  the `git diff --stat` summary of security-relevant paths.
- Counts by severity, previous vs current, with delta.
- Status table, one row per finding ID across both reports:

| ID | Title | Previous | Current | Status |
| --- | --- | --- | --- | --- |

  Status values:
  - New: not in previous report
  - Fixed: in previous, no longer reproducible (cite the commit or change, and the regression test if added)
  - Persisting: still present, unchanged
  - Regressed: previously fixed or lower severity, now present or worse
  - Severity changed: same issue, different rating (state why)
  - Not re-verified: previous finding in a file untouched since the last audit and not re-checked (avoid this; re-verify instead)
- Mermaid chart or table showing the trend (counts per severity per version).
- "Previous remediation roadmap" check: for each item, mark Done, Partial, Not done.
- Verdict: release-blocking items (any new or regressed Critical/High).

Mark every "Fixed" by re-reading the code, not by trusting the previous report or commit messages.

## Finish

Print only: finding counts by severity, counts New/Fixed/Persisting/Regressed,
and the report path.
