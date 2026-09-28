# Desktop conversation recovery (#59)

Controlled-provider recovery checks extend the explicit-selection work in
[#58](desktop-selection-2026-09-28.md). These checks do not qualify live model
behavior. All five mains remain experimental.

## Provenance and reproduction

- Starting source: `4aabef1f8fbe8aa9976376128bada2653cb839cd`.
- Tested Go source hashes: [source-sha256.json](evidence/desktop-recovery-2026-09-28/source-sha256.json).
- Platform: macOS 26.6.2 arm64; Go 1.27.1.
- Client contract: ChatGPT 26.917.71314 (10954), `com.openai.codex`.
- Actual bundled engine: `codex-cli 0.155.0-alpha.16.4`, SHA-256
  `93169e745735930598e867ad837abf3fdc50774a3ad7e7aa89c0d0c51b0189a5`.
- Route: public launcher → synthetic desktop shell → durable bridge → actual
  bundled app-server → authenticated adapter → controlled HTTP provider.
- Homes, credentials, settings, conversations and workspaces are temporary test
  data. Tests never use the ordinary account or live Token Factory inference.

```sh
TOFA_TEST_DESKTOP_ENGINE=/Applications/ChatGPT.app/Contents/Resources/codex \
  go test -race -timeout 20m -json ./...
go vet ./...
python3 scripts/install_test.py
python3 scripts/qualify_macos_test.py
```

## Recovery observations

`TestDesktopHistoryRoundTripRequiresFreshLaunch` exercises all five mains:
`zai-org/GLM-5.3-Flash`, `deepseek-ai/DeepSeek-V4.1-Flash`, `zai-org/GLM-5.3`,
`moonshotai/Kimi-K3`, and `nvidia/Nemotron-3-Ultra-550b-a55b`.
Each completes an initial tool/result exchange, launcher cancellation, ordinary
history hydration with sending refused, and two matching-main relaunches. Each
resumed turn executes a fresh tool and sends its result back to the provider.
Assertions retain ordered messages, tool results, thread ID, main, provider, title,
workspace association, native conversation routing, settings and credentials.

The earlier provider fixture answered from any historical tool result, so it
could not demonstrate a fresh exchange after resume. A failing ordered-history
assertion reproduced that gap; the fixture now responds only to the current
turn's tool result.

`TestDesktopWrongMainExplainsRecovery` first failed against the generic old
message, then passed with both model identities and explicit quit/relaunch/reopen
instructions. `TestDesktopBundledEngineRequiresRecordedMainOnRelaunch` verifies
that those instructions reach the engine's failed-turn error, with no provider
request. It covers GLM Flash recorded as main while also being the launch Guardian,
and GLM recorded as main while absent from the DeepSeek launch catalog. Relaunching
with the original main continues the same thread and preserves its metadata.

`TestDesktopBundledEngineResumeUsesLaunchGuardian` retains a DeepSeek main across
launches with Kimi and then GLM Flash Guardian. Both turns execute an approved
command, the provider sees the current Guardian, and launch output displays it.
Existing native catalog and native conversation tests retain their descriptor and
routing assertions.

The shared-history failure cases cover:

| Failure/lifecycle | Main models |
| --- | --- |
| Ordinary exit | Kimi, GLM Flash |
| Engine loss | Kimi, Nemotron |
| Adapter failure | Kimi, GLM |
| Startup failure | Kimi, DeepSeek |
| Abrupt launcher death | Kimi, DeepSeek |
| Installed uninstall and purge | Kimi, DeepSeek |

The actual installed upgrade/standalone-purge fixture now launches with DeepSeek.
It checks replacement of owned bridges, compatibility refusal, preserved settings
and credentials, and refusal while another process owns the desktop. This uses a
synthetic engine; real-engine history and removal are checked separately above.
Existing ownership tests cover surviving desktops, unrelated processes, expired
routes, symlinks and artifacts with unproven ownership.

Concrete [matching-main and wrong-main recovery examples](../codex-desktop.md#matching-main-example)
are in the user guide. Deliberate conversation model changes remain in #55.

## Additional checks and review

- Full race-enabled Go suite: 413 tests/subtests passed, no failures. The actual
  bundled-engine checks were enabled; the installed standalone CLI's two opt-in
  tests and installed Electron UI check were skipped. Exact command, package
  timings and skip names are in [checks.json](evidence/desktop-recovery-2026-09-28/checks.json).
- Targeted actual-engine matching-main, wrong-main, Guardian-resume and
  failure/lifecycle checks passed.
- `go vet ./...` and `git diff --check` passed.
- Offline installer tests: 2 passed. macOS qualification harness: 18 passed.
- Linux amd64 and Windows amd64 cross-builds passed.
- Standards review: 0 findings. No documented-standard violations or material
  new code smells; the change remains localized and uses the confirmed seams.
- Spec review: 0 findings. Required recovery, routing, persistence, examples and
  coverage limits are represented; no scope creep or ADR conflict.

Initial sandboxed checks could not exercise local listeners/process fixtures;
the same checks passed outside the sandbox. No production behavior was changed
to accommodate those restrictions.

## Coverage limits

The installed Electron UI and standalone installed CLI opt-in checks are outside
this run. Actual bundled-engine checks are enabled. Native Windows/Linux desktop
behavior and other macOS/client versions are unqualified. Controlled responses
prove routing, persistence and execution behavior; they do not measure live model
quality, availability, latency, pricing or title generation. Failure coverage is
the matrix above, not every possible main/Guardian/failure combination.
