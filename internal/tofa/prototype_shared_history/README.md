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

The subsequent human walkthrough confirmed ordinary-session continuation during a
tofa launch, tofa-session continuation, and recovery of that same tofa session
after ordinary → tofa relaunch. It also found that ordinary mode cannot display
the tofa conversation's messages: selecting its list item loops on the logo while
resume repeatedly reports the missing provider. Stored history survives; readable
history in ordinary mode was **not** demonstrated.

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

The installed engine updated to `0.155.0-alpha.16.3` before the subsequent human
walkthrough. Keep the initial evidence above scoped to its recorded version.
`run.py` deliberately retains its original version gate. The current-version
resume probe creates its own fresh synthetic profile:

```sh
python3 internal/tofa/prototype_shared_history/resume_probe.py
```

`continue_desktop.py /private/tmp/PROTOTYPE-tofa34-...` continues a previously
created, marked scratch profile. It preserves its synthetic native endpoint and
credentials and pauses for human observation. It records known config mutation
without claiming config preservation. It can restart phase 2 using the existing
sessions rather than duplicating them. Pressing Enter only advances the runner;
user observations are captured separately in the evidence below.

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
| Ordinary session through tofa launch (human follow-up) | Opened and continued through native fixture |
| Tofa session through tofa launch (human follow-up) | Opened and continued after correcting argument placement |
| Tofa session selected in ordinary mode (human follow-up) | List item selectable, but history view loops on missing-provider resume |
| Same tofa session after relaunch (human follow-up) | Opened and continued through a fresh Token Factory fixture |

[evidence.json](evidence.json) captures the successful engine-only run.
[desktop-evidence.json](desktop-evidence.json) captures the desktop failure.
[logo-loop-evidence.json](logo-loop-evidence.json) captures the later human
observations and the argument-placement reproduction/fix on engine alpha.16.3.
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

## Logo-loop diagnosis during the human walkthrough

The initial tofa launch's desktop process received provider overrides before
`app-server`, plus the desktop's plugin override after it. App-server overrides
displaced those root CLI settings, producing `Model provider nebius-tofa not
found` even with the launcher active. The renderer retried resume indefinitely.

`resume_probe.py` reproduced that exact RPC failure in under a second without the
renderer. Removing the app-server override passed; replacing the plugin setting
with an unrelated `web_search` setting still failed. Moving launch overrides to
the app-server arguments made the original probe pass. The human then confirmed
the same tofa session opened and continued in the actual desktop. The wrapper
keeps its original argument placement for other commands such as catalog export.

This fixes the **prototype's active-tofa launch**. The ordinary-mode loop remains:
there the provider is intentionally absent. The separate stale executable setting
also remains unresolved. No production launcher code was changed and no automated
test suite was added; the throwaway reproduction and human checks are the evidence.

## Decision to carry into #34

Keep one history owner and retain recorded provider/model routing. Use a merged
native-plus-tofa catalog when qualifying a shared engine; a Kimi-only override
cannot preserve native metadata. Do not transplant this temporary-executable
wrapper into the ordinary desktop profile yet.

Next bounded experiment: find a source-supported way to keep the executable
reference durable or prevent launch-specific environment propagation into shared
tool settings, without rewriting user credentials/settings on exit. Preserve the
validated app-server argument placement. Treat ordinary-mode readable history as
an unmet part of the current issue wording, distinct from the accepted requirement
to relaunch through tofa for continuation; do not call the observed logo loop an
explicit user-facing unavailable-provider error.
Only after that should #34 move into production launcher implementation and its
already-approved executable/HTTP and engine-RPC test boundaries.

This directory belongs on `prototype/34-shared-desktop-profile`, out of main.
Production launcher code is unchanged. The validated decisions and this evidence
are recorded on #34; the simulation's reducer is not a verified desktop contract
and should not be copied into production.


## Durable bridge and inactive-provider follow-up (2026-09-24)

The durable bridge candidate is in `durable_route_probe.py`. Run with
`--temporary` to reproduce the dangling saved executable; run without that flag
for the proposed four-launch desktop check. The baseline failed as expected on
alpha.16.3 (`temporary-reference-evidence.json`). The candidate's non-desktop
checks passed (`durable-executable-evidence.json`): native delegation without
launch settings, valid launch-file handling, explicit rejection of expired files,
unchanged bridge bytes and no embedded credential.

The four-launch candidate check has **not run**. Automatic approval review timed
out before execution, including the attempts after the maintainer explicitly
approved it. That is a tooling blockage, not a desktop result or a safety finding.
Production code remains unchanged. Durable bridge installation, upgrade,
uninstall, concurrent ownership and unexpected process death remain unqualified.
The bridge leaves the desktop's shared settings alone on shutdown.

There is a separate candidate for the ordinary-mode logo loop:

```sh
python3 internal/tofa/prototype_shared_history/offline_provider_probe.py
```

Without a provider definition, the saved synthetic thread fails resume with the
same missing-provider error observed in the desktop. Adding inactive provider
metadata to the scratch config allows ordinary engine startup to resume and read
that history without any launch overrides, running adapter or credential. Sending
a turn still fails explicitly on the absent credential. The inactive definition
uses loopback port zero; it cannot direct a request to a real inference service.
It preserves the recorded provider rather than silently changing to OpenAI.

`offline-provider-probe-evidence.json` records this reproducible engine-boundary
result. A prior probe also successfully resumed the saved human-trial conversation
using only command-line inactive metadata (`offline-provider-evidence.json`).
Neither result proves that the desktop renderer stops looping. Desktop verification
and compatibility with active launch overrides remain outstanding. This proposes
persisting non-secret, inactive provider metadata; it does not persist a live
adapter endpoint or bearer. No ordinary user profile was changed.
