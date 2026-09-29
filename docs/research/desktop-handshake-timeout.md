# Desktop bridge startup timeout

The user reported that `tofa dev-9214539` opened the desktop after choosing
DeepSeek, then closed it with:

```text
tofa: desktop ownership handshake timed out; no existing process was adopted
```

The failed run's desktop log recorded the launcher bridge being spawned. The
saved bridge matched the source-build binary and its ownership digest. Three
isolated installed-Electron startup checks passed (1.86, 1.55 and 1.67 seconds),
so those checks alone did not reproduce the user's ordinary-profile failure.
Normal account settings, credentials and conversation data were not changed.

## Reproduction and diagnosis

At the existing public `App.Run` executable-fixture boundary, a desktop acquired
its native singleton immediately but waited four seconds before making the
bridge's authenticated launch-context request. This deterministically reproduced
the exact reported error twice:

```sh
go test ./internal/tofa -run '^TestDesktopAllowsBridgeStartupAfterAcquiringNativeProfile$' -count=2 -v
```

Both original runs failed with `desktop ownership handshake timed out; no existing
process was adopted` (3.83 and 3.93 seconds including fixture setup). Removing the
delay uses the established successful startup path. The four-second fixture
models delayed initialization; it is not a measurement of the user's startup.

Ranked explanations were a deadline conflating native ownership and bridge
readiness, incorrect parent PID, or lost bridge authentication. Holding PID and
authentication constant and separating the readiness deadline made both delayed
runs pass (6.05 and 6.12 seconds including setup and normal fixture shutdown).
This confirms that the launcher incorrectly rejected an already-owned desktop
whose bridge initialized after three seconds. It is consistent with the user's
failure; the original source of their initialization delay remains unmeasured.

## Correction and safety boundary

The three-second deadline still stops a contender that has not proven ownership.
At that deadline, only the exact launcher-created PID holding the native profile
singleton receives another 12 seconds for the authenticated bridge request.
The total startup-handshake allowance is 15 seconds. Native ownership is checked
every 100 ms during the additional wait. The original parent-PID and ownership
validation still applies when the authenticated claim arrives.

Ownership loss, cancellation or a stalled bridge stops only the launcher's process
group. The native lock is never removed or rewritten by the launcher. No existing
process is adopted, inference remains unavailable until qualification completes,
and the adapter's authentication contract is unchanged. A stalled owned bridge
now produces a distinct bridge-startup timeout instead of claiming the native
profile was unowned.

Targeted race checks passed for the delayed bridge, no ownership, ownership loss
during the wait, permanently stalled bridge, existing owners, competing launchers,
pre-qualification inference refusal and installed-Electron startup. The unclaimed
case stopped in 3.83 seconds, ownership loss in 4.54, and the stalled bridge in
16.00 including fixture setup. There was no paid inference. These observations
use disposable profiles and synthetic credentials; the user's original ordinary
profile still needs a retry of the rebuilt launcher.

The full Go race suite subsequently passed in 631.492 seconds with the installed
engine and isolated installed-Electron checks enabled. Only the two optional
standalone CLI tests skipped. `go vet ./...` and Linux/Windows amd64 cross-builds
also passed; cross-builds do not establish native runtime coverage. Standards and
spec reviews found no remaining implementation findings. The rebuilt public
launcher also passed app-menu display/cancellation in a disposable terminal.
