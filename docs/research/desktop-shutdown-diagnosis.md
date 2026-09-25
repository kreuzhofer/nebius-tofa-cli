# Engine-first desktop shutdown refusal — 2026-09-24

During the [#50 continuation](../releases/desktop-live-attempt-2026-09-24.md#continuation--14331437-utc),
the operator used Cmd-Q after successful Kimi recovery and native continuation.
Candidate `v0.0.0-qualification.50.fd22182` exited 1 with the captured category
`engine_exited_before_app`, mapped to
`owned desktop app-server exited; launch cancelled`. A separate short launch/quit
had exited 0. The initial failed walkthrough had no retained terminal diagnostic,
so its cause cannot be assigned retrospectively from this later event alone.

## Feedback loop

The approved `App.Run` executable-fixture seam reproduced the captured error:

```sh
go test ./internal/tofa -run '^TestDesktopAllowsEngineFirstGracefulShutdown$' -count=2
```

Both runs failed, in 2.86 and 2.62 seconds, with the exact engine-exit error.
The fixture starts its owned engine, lets it exit cleanly, and finishes the app's
own shutdown 800 ms later. Before the correction, the launcher interrupted that
legitimate sequence after its 200 ms allowance. No network inference, account
credential or human UI was needed to reproduce this ordering defect. The 800 ms
fixture interval models a valid delayed shutdown; it is not a measurement of
the operator's desktop shutdown duration.

The ranked hypotheses were: the shutdown allowance is too short; the app itself
exits unsuccessfully; or the engine dies while the app remains open. The clean
delayed-exit fixture isolates the first case. Its nonzero-app-exit counterpart
and the existing engine-loss fixture distinguish the other cases.

## Correction and boundaries

After observing a previously running engine disappear, tofa now gives its owned
app up to two seconds to finish. This is a bounded grace period, not a success
fallback: the app must actually exit, and its real exit error is preserved.
If it stays alive, the launcher reports engine loss and performs its existing
owned-process-group cleanup. A replacement engine is not adopted and cannot
reset the deadline.

The grace timer runs within the lifecycle loop, keeping cancellation and native
profile-ownership checks active. The ownership-loss allowance is unchanged, and
only the launcher-created process group can be signaled. Credentials, provider
routing, catalog freshness and compatibility gates are unchanged.

The clean delayed-exit regression passes and verifies the app reached its own
shutdown completion marker. A delayed app exit with status 23 retains that
failure. The real-engine-loss fixture still fails explicitly and stops its owned
worker. Existing cancellation and adapter-failure cases also pass. The targeted
run passed in 20.629 seconds; `go vet ./...` passed.

The first full race run failed two existing startup checks (native catalog fresh
cache and modified bridge ownership). Their fake desktop could exit after a
fixed 400 ms measured from worker spawn, before the bridge actually executed the
engine. A temporary 600 ms pre-exec delay reproduced the same startup error in
the bridge-ownership case. Adding a fixture worker-readiness acknowledgement
made that delayed case pass; a delay after engine exec already passed without
the acknowledgement. This distinguishes fixture startup scheduling from the
production shutdown defect. The fixture now waits for worker readiness before
starting its existing lifetime; a failed worker or five-second readiness timeout
still fails explicitly. The temporary injected delay was removed. No production
startup rule was relaxed. The affected catalog, ownership and shutdown checks
then passed under the race detector in 44.425 seconds.

A subsequent suite exposed a dependent fixture-ordering issue: the crash helper
treated the published worker PID as readiness, so it could kill the launcher
before the new acknowledgement. The synthetic desktop then timed out and exited,
invalidating that test's surviving-desktop premise. The stalled case was ended
by stopping only its verified synthetic process group. Publishing the fixture
PID after worker readiness restored the intended boundary; the isolated crash
recovery regression passed under the race detector in 5.476 seconds. These
fixture corrections do not alter launcher behavior. The final full
`go test -race ./... -count=1 -timeout=6m` run passed (`internal/tofa`:
247.720 seconds), with both installed-desktop and installed-engine opt-ins
enabled. The crash-recovery case also passed in that full run. Only the two
installed CLI opt-ins, `TestInstalledCodexToolAndContinuationThroughAdapter` and
`TestInstalledCodexApprovalReviewThroughAdapter`, skipped because
`TOFA_TEST_CODEX` was unset. `go vet ./...` passed after the final fixture edit.
Standards and Spec reviews found no actionable issues in the final correction.

This corrects a demonstrated lifecycle defect consistent with the captured
Cmd-Q failure. The subsequent
[final qualification run](../releases/desktop-shared-history-final-2026-09-24.md)
confirmed live Kimi/native continuation followed by launcher exit 0 and all four
ordinary-mode preservation observations on candidate `b772004`. Earlier evidence
and failures remain pinned to their original binaries; no historical result was
rewritten.
