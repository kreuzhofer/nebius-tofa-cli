# Throwaway prototype: shared desktop history (#34)

Question: can one desktop profile retain ordinary and Token Factory conversations,
while a launch-owned executable injects temporary routing without changing shared
account or routing settings?

**Verdict: shared engine history works; the desktop executable-wrapper candidate
is not ready for the ordinary profile.** The actual desktop writes the temporary
`CODEX_CLI_PATH` into `mcp_servers.node_repl.env.CODEX_CLI_PATH` in shared
`config.toml`. Removing the launch directory leaves that tool setting pointing at
a missing executable. This was reproduced in a synthetic profile during the first
ordinary-mode desktop phase, before trying tofa mode. The accepted limitation
(relaunch through tofa to continue a tofa conversation) is not the problem.

## Open or run

Double-click [index.html](index.html) for an in-memory walkthrough. It has free-play
actions and three guided scenarios. Its lifecycle behavior is a proposal, not a
claim that the installed desktop implements it. No server or dependencies needed.

From the repository root on the qualified Mac:

```sh
python3 internal/tofa/prototype_shared_history/run.py
```

That invokes the installed engine and exercises ordinary → tofa → ordinary →
tofa against one marked scratch profile. All inference responses are synthetic
and served on loopback. It uses no real credentials and does not access the
ordinary profile. It retains scratch files for inspection and closes its routes.

Reproduce the desktop result:

```sh
python3 internal/tofa/prototype_shared_history/run.py --desktop-smoke --report internal/tofa/prototype_shared_history/desktop-evidence.json
```

This opens a separate desktop with shared **scratch** Electron/engine state,
observes its engine handshake, and stops only its owned process group. It exits
nonzero when shared config changes, after writing sanitized failure evidence.
The failure is the captured answer, not a passing desktop qualification.
`--desktop` instead pauses for human inspection at each phase; pressing Enter
does not certify visual checks. Do not sign in or enter real credentials in it.

Requires `/Applications/ChatGPT.app`, bundled engine
`codex-cli 0.155.0-alpha.9.2`, and Python 3.9+. Tested with desktop
26.915.31945 (9922), macOS 26.6.2 arm64. An engine version gate stops unqualified
versions. Scratch directories use short `/private/tmp/PROTOTYPE-tofa34-*` names:
the original longer default temp path caused an IPC socket `EINVAL`.

## Evidence and limits

| Observation | Result |
| --- | --- |
| Bundled engine `thread/list`, `thread/read`, `thread/resume` | Both known IDs listed once; titles and workspace retained; completed turn counts survive mode changes |
| Native continuation while tofa active | Original native provider/model; request reached native fixture |
| Tofa continuation in ordinary mode | Explicit missing-provider error; history retained |
| Tofa relaunch | Same session/model/provider continued through fresh endpoint and bearer |
| Native model catalog | Merged catalog retained all shipped native descriptors |
| Engine-only shared config/auth | Byte-identical; local bearer absent from shared engine files |
| Actual desktop startup | Selected temporary executable and completed engine handshake |
| Actual desktop shared config | Changed: desktop/plugin defaults plus persisted temporary executable path |
| Actual desktop auth and native model/provider | Unchanged in failing scratch run |
| Session list and full history in desktop UI | Not visually verified |

[evidence.json](evidence.json) captures the successful engine-only run.
[desktop-evidence.json](desktop-evidence.json) captures the desktop failure.
Reports omit bearer values, local ports, session IDs and personal paths. The runner
prints its marked scratch directory; retained scratch files contain only synthetic
credentials. Desktop startup may contact vendor services for its normal metadata
and plugin behavior; this is not an offline network-isolation experiment.

The runner reuses the existing public-RPC driver in
`scripts/desktop_history_test.py`; it adds no tests. Python/inline JavaScript syntax
checks passed. Browser automation refused the local HTML URL under its security
policy, so the walkthrough has not received a visual interaction check.

Not qualified: existing account/onboarding, normal packaged-Mac instance behavior,
concurrent launches, enforced policy, upgrades, crash recovery, native live account
catalog refresh, uninstall, and actual Token Factory inference. A merged catalog
also leaves a source-identified question: desktop title helpers request a native
model under the active provider. The fixture rejects mismatched models rather than
silently remapping them; no title-helper request was observed in this run.

## Decision to carry into #34

Keep one history owner and retain recorded provider/model routing. Use a merged
native-plus-tofa catalog when qualifying a shared engine; a Kimi-only override
cannot preserve native metadata. Do not transplant this temporary-executable
wrapper into the ordinary desktop profile yet.

Next bounded experiment: find a source-supported way to keep the executable
reference durable or prevent launch-specific environment propagation into shared
tool settings, without rewriting user credentials/settings on exit. Then rerun
the four desktop phases and verify session-list parity and full history visually.
Only after that should #34 move into production launcher implementation and its
already-approved executable/HTTP and engine-RPC test boundaries.

This directory belongs on `prototype/34-shared-desktop-profile`, out of main.
Production launcher code is unchanged. The validated decisions and this evidence
are recorded on #34; the simulation's reducer is not a verified desktop contract
and should not be copied into production.
