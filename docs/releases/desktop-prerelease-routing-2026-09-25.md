# Desktop prerelease routing corrections — 2026-09-25

**The rc.3 live attempt failed qualification.** Its installation/upgrade,
shared-session visibility, streaming, tool execution/result continuation and
another user turn passed. Automatic titles and automatic approval failed locally
before either request reached Token Factory. No rc.3 release was published.
These corrections have offline regression coverage; corrected live qualification
and publication remain part of [#35](https://github.com/kreuzhofer/nebius-tofa-cli/issues/35).

## Failed candidate and retained evidence

The [unmodified attempt snapshot](evidence/desktop-rc3-attempt-2026-09-25.json)
records source `b53679f44255beb093db09a710a13cce073073ca`, version
`v0.1.0-rc.3`, and macOS ARM64 binary SHA-256
`aed90826cf2f9eb6b358d345b530f8ad55880530c48c5907f487c98c75e1575a`.
The report SHA-256 is
`2a18fe57780b9cdea0e5bb472fa43748d6ad124f87a83481e2bbe7132d7a902c`.
It pins ChatGPT 26.917.71314 (10954), bundled engine
0.155.0-alpha.16.4, macOS 26.6.2 (25G83), ARM64, and the launch-owned
Responses adapter to `moonshotai/Kimi-K3`.

The attempt stopped after the automatic-review marker was absent. The report's
`not_observed` title result is retained. A subsequent scoped read of the desktop
and engine diagnostic logs establishes an actual title error at 09:23:32 UTC:
the client selected `gpt-6-luna`, which the adapter rejected as an unsupported
title model. The reviewer trace at 09:35:28 UTC identifies `codex-auto-review`;
the resulting tool error states that review could not complete and the command
was not executed. This was a local routing failure, not a provider response or
a reviewer decision that the action was unsafe. No blocked command was bypassed.

There was also an observer limitation: the desktop created the synthetic
conversation's project under `Documents/Codex`, rather than using the planned
scratch workspace. The installer preservation checks passed before that project
was created. Later live workspace preservation must be attributed to the actual
desktop project. The snapshot does not claim completed shutdown, ordinary-mode
return, relaunch, uninstall, reinstall, or final preservation.

The preceding [cross-platform CI run](https://github.com/kreuzhofer/nebius-tofa-cli/actions/runs/36109946328)
passed artifacts and all three native jobs for `b53679f`; publication was skipped.
That branch run used the CI version, not the exact rc.3 release artifact. It does
not qualify the corrections or the failed live workflows.

## Automatic review

The bundled engine reproduced the live failure through the public desktop launch
and app-server boundaries: zero reviewer requests reached the synthetic provider,
and the gated command did not execute. The merged native catalog advertises
`codex-auto-review`, so the engine selects it when the Kimi descriptor has no
reviewer override. The narrower CLI catalog had allowed the engine to use Kimi.

Only the added Kimi descriptor now sets `auto_review_model_override` to
`moonshotai/Kimi-K3`. Native descriptors remain intact. Startup explicitly announces
the reviewer choice. This does not select the user's approval mode, change managed
policy, translate arbitrary `codex-auto-review` requests, or make approval decisions.
The existing adapter preserves the real reviewer's tools and complete assessment
schema as final-answer guidance; the bundled engine still parses the assessment
and enforces the execution gate.

The new bundled-engine regression passes allow, allow after a real reviewer tool
inspection, deny, invalid assessment, and provider failure. Execution is observed
through the engine's command events. Denials and failures do not execute the gated
command; the main turn's approval policy and reviewer mode remain as requested.
These provider responses are synthetic, not a live Kimi quality or latency claim.

## Automatic titles

Replaying the observed `gpt-6-luna` selection with the actual bundled engine first
reproduced the same unsupported-model error. Adding that model identity alone
then exposed an unsupported-inventory error. A temporary observer at the network
boundary captured the entirely synthetic engine request and was removed after
diagnosis. No production request bodies or credentials were added to fixtures.

The older `gpt-5.6-luna` title request contains three `functions` tools. The newer
selection contains four, adding `request_user_input_async`, plus `clock.sleep`
and the six `collaboration` tools. The adapter now recognizes these two exact,
model-specific inventories. It relocates every tool definition intact and retains
the complete title schema as final-answer guidance, as in #52. All remaining
input, metadata, reasoning and text options remain unchanged. The desktop still
validates the title/description, and responses are neither repaired nor retried.

Both title source markers, the exact strict title schema, the additional-tools
container, and the model-specific inventories are required. Unknown models,
crossed inventories, changed schemas, duplicate/unknown namespaces or actions,
and non-title turns still fail explicitly. CLI requests remain unchanged.
The [reduced Luna 6 fixture](../../internal/tofa/testdata/desktop-luna6-title-request.json)
uses synthetic descriptions, parameters, input and identifiers; it is not a raw
live request. The bundled-engine test independently exercises complete real tool
definitions for both Luna identities.

## Validation and next candidate

Targeted title/preservation/refusal tests, the installed bundled-engine review
tests, CLI isolation tests and `go vet ./...` pass. The full `go test -race ./...
-count=1 -timeout=6m` suite passes with the installed bundled engine and CLI
opt-ins enabled (`internal/tofa`: 316.636 seconds); GUI automation was not enabled.
Independent Standards and Spec reviews against `b53679f` found no issues.
The older live title evidence
for #52 and this failed rc.3 attempt remain separate observations.

A new candidate must repeat live title generation and automatic approval with
execution, then complete the remaining shared-history and installed-lifecycle
checks. Use a new version and retain its exact checksums. The compatibility gate,
mandatory `--allow-unverified`, explicit compaction limitation, and exclusion of
Windows desktop remain unchanged. Neither this document nor the offline tests
promote desktop support or authorize replacing existing release assets.
