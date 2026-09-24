# Desktop profile ownership on the updated macOS client

Qualification for [#47](https://github.com/kreuzhofer/nebius-tofa-cli/issues/47),
2026-09-24. Five installed-app diagnostics passed against disposable synthetic
profiles. The operator's ordinary desktop, credentials, and conversation store
were not used. This qualifies specific native ownership behavior. Additional production-launcher
and history checks are recorded below; real-account/UI continuity is not claimed.
[Sanitized evidence](evidence/desktop-profile-ownership-2026-09-24.json).

## Accepted behavior and implementation boundary

Tofa refuses an existing ordinary desktop and asks the user to quit it manually.
Opening the ordinary app while tofa owns the same profile may reuse that desktop;
it does not change the running process's provider mode.

The updated Owl shell acquires a native Chromium ProcessSingleton before
JavaScript startup. The JavaScript bootstrap's skipped Electron lock on ordinary
macOS launches therefore does not imply missing native coordination. A successful
launcher launch needs both the native profile owner PID matching the spawned
desktop and an engine-wrapper acknowledgement bound to that process. Serialize
launcher attempts separately and withhold integration writes until both facts
are established.

Native races can produce missing ownership evidence even when an engine starts.
The correct launcher outcome is refusal, with cleanup limited to its own child
processes. A process snapshot, successful spawn, or engine startup alone cannot
establish ownership. Existing-owner notification is also not permission to adopt
or reconfigure that desktop.

## Installed provenance

Final ownership qualification uses a frozen copy of ChatGPT
**26.917.71314 (10954)** with bundled engine **0.155.0-alpha.16.4**, stored at
`/tmp/tofa47-qualified/ChatGPT.app`. The script checks the exact version/build,
engine version, and three bundle hashes before and after the suite. These
minified function names apply to the artifacts below, not stable public APIs.

| Artifact | SHA-256 |
| --- | --- |
| `Contents/Resources/app.asar` | `03108a728bdb1616958ab89587c5495cab0cf4cd1bbe109bdfb186df0a113804` |
| `Contents/Resources/codex` | `93169e745735930598e867ad837abf3fdc50774a3ad7e7aa89c0d0c51b0189a5` |
| `.vite/build/bootstrap-C4dRql4x.js` | `0757af0981f4552ca79ed1a364eafa6a73e71a92e65ea4fce747c7b6c715ed4a` |
| `.vite/build/main-C-Mhak1n.js` | `457c79be69620d4489e94c14ac665f81731d869635606dcf175b7b4cc2e8b467` |
| `.vite/build/src-DldfpmrL.js` | `88ec69722b5d87a7081edf2e2d6a2e21c3f25300cee587363d74b8b87969a412` |
| Native `Codex Framework`, version `153.0.8010.53` | `fac56b9423fe81e6c206a8ca4e755d5dfbc698bef7bbbb3a4fc19cfa6c3e6ca2` |

Initial source inspection and manual probes used **26.917.62051 (10789)** /
**alpha.16.3**, with archive SHA-256
`c41157d36d701d3228c82e56852f49da2381941a7e5ccfac2d58b7e10e31aefd`
and native framework SHA-256
`dff8143b1c2a1788bb59e27d294cd6ee12843af2faaf97b0545fc06e799979f1`.
The installed bundle changed during the investigation; causation was not
established, and prior scripted reruns did not individually pin their versions.
Those runs are historical context, not final exact-build qualification. The
three inspected JavaScript members are byte-identical in the new snapshot;
native/engine hashes changed and their runtime behavior was rechecked.

## Reproducible installed-app diagnostics

Run the [opt-in script](../../scripts/desktop_ownership_test.py):

```sh
TOFA_TEST_DESKTOP_APP=/tmp/tofa47-qualified/ChatGPT.app \
  python3 scripts/desktop_ownership_test.py -v
```

An optional `TOFA_TEST_DESKTOP_OWNERSHIP_EVIDENCE` path receives sanitized
observations, including unsuccessful qualification. Without the app opt-in, all
five tests skip. The engine observer uses `CODEX_CLI_PATH`, records only
app-server PID/parent PID, and executes the actual bundled engine. The harness
also verifies the engine executable through the OS process table. Test processes
set `CODEX_SPARKLE_ENABLED=false`; each owner startup must report both
`enableUpdater=false` and `enableSparkle=false`. Operator updater preferences
are not edited, and the production app bundle is not the test target.

| Boundary | Observed result |
| --- | --- |
| Ordinary owner, then ordinary contender | Contender exits zero, starts no engine, reports existing browser session; owner's native lock and engine remain. |
| Explicit override selecting the same ordinary profile, then ordinary contender | Same reuse behavior. |
| Ordinary owner, then explicit override selecting that same profile | Same reuse behavior. |
| Simultaneous direct launches | At most one engine starts. Both stable ownership and missing native lock were observed; the latter requires launcher refusal. |
| Native termination of the verified owned PID, then relaunch | Termination accepted, desktop exits zero; stale singleton symlinks remain and native relaunch reclaims them successfully. |

The stable owner has `SingletonLock` pointing to its hostname/PID, plus
`SingletonSocket` and `SingletonCookie` symlinks. The engine parent PID matches
that desktop. An exact simultaneous race also produced contender exit 21 with a
native singleton-creation error, followed by a live winning engine but no
`SingletonLock`; the socket/cookie remained. Other runs produced ordinary
exit-zero reuse with a matching owner. The pinned alpha.16.4 rerun reproduced
the missing-lock outcome. The concurrent diagnostic records either
possibilities; its passing result **does not qualify missing ownership**.

Each collision attempt has a five-second deadline. The tests never exercise
native takeover of a hung owner. Cleanup signals only test-created process
groups. No native singleton artifact is manually removed while its disposable
profile is in use. The eventual temporary-directory cleanup happens after owned
processes stop.

## Shutdown and stale-state recovery

Both SIGTERM and the AppKit `NSRunningApplication.terminate` call left all three
native symlinks behind. The latter targeted the verified test-created owner PID,
was accepted, and produced exit zero. Its retained lock still named the exited
PID. A subsequent direct launch reclaimed the same synthetic profile and
established a fresh native owner and engine.

One concurrent diagnostic run accepted the native termination request but did not
exit within eight seconds. Its cause remains unproven. Nine standalone graceful
checks passed before the harness change. The test now waits up to ten seconds for
the installed app's primary-window readiness log before requesting quit; the
actual eight-second shutdown deadline is unchanged and the request is not retried.
That updated case passed. A successfully delivered termination request is not
itself proof of process exit; production cleanup observes exit and bounds its own
SIGTERM/SIGKILL handling.

Thus refusing every existing singleton artifact would also refuse normal
relaunches. The launcher permits native-managed recovery only for a
well-formed same-host lock whose owner PID is demonstrably absent, while refusing
live, malformed, foreign-host, or otherwise uncertain ownership. PID reuse must
result in refusal, not assuming an old PID is still the former owner. Repeated production-launcher fixture tests exercise this recovery policy.

Do not delete native lock/socket/cookie state merely because a launcher failed.
Conditional manual cleanup would need captured symlink identities and targets,
confirmed owned-process termination, and protection against a concurrent native
owner replacing them. Separate comparisons and unlink operations are not an
atomic ownership protocol. Native recovery avoids that additional launcher
mutation and is the mechanism exercised here.

## Source findings and limits

Bootstrap `Tk` requests `appData/Codex` for the unsuffixed production flavor.
Electron's normal macOS app-data base is `~/Library/Application Support`.
However, the probe observed Owl rejecting a late JavaScript user-data path
change in favor of the earlier canonical native selection. Qualification must
check the native directory, not infer it from `app.setPath` alone.
[Electron paths](https://www.electronjs.org/docs/latest/api/app#appgetpathname).

Bootstrap `Sk` skips the explicit `requestSingleInstanceLock` call for packaged
macOS without `CODEX_ELECTRON_USER_DATA_PATH`. The installed native framework
nevertheless contains ProcessSingleton implementation paths, the three native
artifact names, and the observed profile-corruption prevention error. Generic
Electron documentation about direct executable launches bypassing Finder
coordination does not override these exact-build observations.
[Electron singleton documentation](https://www.electronjs.org/docs/latest/api/app#apprequestsingleinstancelockadditionaldata).

Current upstream Chromium uses a profile-specific symlink/socket/cookie
protocol; its macOS `flock` path only handles a legacy lock-file format.
Its notification path can terminate an unresponsive owner, with a default
20-second timeout and a SIGKILL implementation. The installed binary contains
matching hung-owner/termination telemetry strings, but this is not exact Owl
source or proof of its timeout. Refuse detected incumbents and bound contenders;
do not regard native collision handling as universally harmless.
[Chromium singleton source](https://raw.githubusercontent.com/chromium/chromium/main/chrome/browser/process_singleton_posix.cc),
[lock parser](https://raw.githubusercontent.com/chromium/chromium/main/chrome/common/process_singleton_lock_posix.cc).

LaunchServices is not a substitute for ownership evidence. Apple's default
`createsNewApplicationInstance=false` reuses a running app; its returned
`NSRunningApplication` does not separately attest that this invocation created
the process. An ordinary launch can win between a process scan and the launch
call. No document URL or deep link should be sent before ownership is known.
[Instance option](https://developer.apple.com/documentation/appkit/nsworkspace/openconfiguration/createsnewapplicationinstance),
[launch API](https://developer.apple.com/documentation/appkit/nsworkspace/openapplication(at:configuration:completionhandler:)).

Main `EDe`/`DDe` hydrates an interactive shell environment before loading local
state on macOS. It preserves the original `CODEX_HOME` across that hydration
only when the Electron profile override was explicit. Shared `xM`/`SM` then
resolves `CODEX_HOME` or `os.homedir()/.codex`. The launcher's inherited home
therefore need not equal the ordinary desktop's resolved home. Local data
initialization/migrations occur before the proposed engine-wrapper handshake;
that handshake gates launcher integration and engine work, not earlier desktop
writes. [Installed main/shared source, hashes above.]

Account access in shared source follows `getAuthenticatedPrincipal` →
`getAuthToken` → `requestAuthStatus` → app-server `getAuthStatus`;
`getAccount` calls `account/read`. Bootstrap settings store `ED`/`OD` reads
the `desktop` configuration table and has an onboarding migration key.
Path equality alone does not establish preserved account/onboarding behavior.
[Installed source, hashes above.]

## Synthetic-profile isolation

The harness verifies Foundation's application-support directory before each
launch and uses short disposable paths for macOS Unix sockets. Its allowlisted
environment supplies synthetic `HOME`, `CFFIXED_USER_HOME`, `ZDOTDIR`,
`CODEX_HOME`, and dummy file-backed engine authentication. Shared `Uq`/`eJ`
selects the account's shell before consulting `SHELL` and runs it with
`-ilc`; changing `SHELL` alone would not isolate startup-file locations.

Synthetic home and engine file-store settings do not isolate native cookie
encryption keys. The test-only literal `--use-mock-keychain` addresses that
identified native path and exists in the installed framework. Electron's own
test change documents why spawned tests need it.
[Electron change #53790](https://releases.electronjs.org/pr/53790).
Do not apply this synthetic test setting to production profiles.

Production bootstrap ignores `CODEX_ELECTRON_CHROMIUM_SWITCHES` outside its
development flavor and explicitly rejects `CODEX_DESKTOP_NETWORK_POLICY`.
Neither is an available packaged-build isolation shortcut. The JavaScript
account path and identified native cookie-key path were investigated; these
diagnostics are not an exhaustive audit of every opaque native framework call.

Updater suppression is source-backed: bootstrap `Rt` tests whether
`CODEX_SPARKLE_ENABLED` is exactly `false`, and `zt` excludes updater support
when it is. `shouldIncludeSparkle`/`shouldIncludeUpdater` supply the startup
configuration. This is a per-process diagnostic setting; no user updater policy
or preference was changed. [Installed bootstrap, hash above.]

Validation: five installed-app diagnostics passed, Python compilation passed,
and the no-opt-in invocation skipped all five tests. Native owner races and
stale recovery are recorded as scoped observations. Production-launcher executable/HTTP checks also cover refusal, lease contention,
ownership loss, version-checked edits and cleanup. The actual-engine round trip
retains ordered messages and command/results in paginated history, stable IDs,
titles and workspaces across repeated launches. The installed launcher startup
check passes against the frozen app with synthetic account state. Real-account
onboarding and UI continuity remain #50 acceptance work.
