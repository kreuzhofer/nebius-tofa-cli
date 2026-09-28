# Desktop explicit main/Guardian selection (#58)

This is offline routing and execution-gate evidence, not paid model qualification.
All five candidates remain experimental and require `--allow-unverified`.

## Reproduction and provenance

- Starting source: `5957ff06216ccf4904199d350506cd8872152add`.
- Tested source: the implementation committed with this report; exact changed Go
  source hashes are in `evidence/desktop-selection-2026-09-28/source-sha256.json`.
- Platform: macOS 26.6.2 arm64; Go 1.27.1.
- Bundle contract: ChatGPT 26.917.71314 (10954), `com.openai.codex`.
- Actual bundled engine: `codex-cli 0.155.0-alpha.16.4`, SHA-256
  `93169e745735930598e867ad837abf3fdc50774a3ad7e7aa89c0d0c51b0189a5`.
- Metadata: unchanged `assets/evaluation-candidates.json`, provider snapshot
  2026-09-23 with its recorded source checksum. These are dated capabilities,
  not fresh live availability or pricing measurements.
- Route: public `App.Run`/`RunContext` → synthetic desktop shell fixture → durable
  bridge → actual bundled `app-server` → authenticated adapter → controlled HTTP
  provider. Homes, history, profiles and credentials are synthetic and temporary.
  No ordinary account, active desktop, or paid inference was used.

## Observations

`TestDesktopExplicitMainAndGuardianMetadata` exercises all five explicit mains
with default `zai-org/GLM-5.3-Flash` Guardian. It checks effective output, context
windows, modalities and reviewer binding. The new selection accepts compatible
shortlist pairs without promoting historical CLI evidence to desktop support.

| Main ID | Default GLM Flash Guardian exercise | Live desktop qualification |
| --- | --- | --- |
| `zai-org/GLM-5.3-Flash` | Same-model routing; provider failure prevents execution | Not attempted |
| `deepseek-ai/DeepSeek-V4.1-Flash` | Allow executes; cancellation and native deadline prevent execution | Not attempted |
| `zai-org/GLM-5.3` | Deny prevents execution | Not attempted |
| `moonshotai/Kimi-K3` | Allow executes with distinct default Guardian | Not attempted |
| `nvidia/Nemotron-3-Ultra-550b-a55b` | Invalid assessment prevents execution | Not attempted |

Additional actual-engine cases cover DeepSeek main with explicit Kimi Guardian
and same-model Kimi review with read-only inspection. Kimi review retains the
existing schema-to-instructions adaptation; GLM receives the native schema.
Tests observe requested/upstream identities and command-execution events, not
merely a successful HTTP status. Native approval policy and parsing stay active.

The native engine makes three attempts for an invalid assessment and four for a
503 stream-establishment failure. These counts are asserted; the adapter adds no
retry or role/model fallback. The native review deadline remains 90 seconds,
measured from provider request receipt with an 85–100 second test tolerance.
Cancelling a turn cancels the provider request and does not execute its action.
Only the test harness deadlines increased to allow this observation.

`TestDesktopBundledEngineRequiresRecordedMainOnRelaunch` creates a GLM Flash
conversation, refuses continuation under a DeepSeek launch (even though GLM Flash
is its Guardian), and resumes the same identity after relaunching with GLM Flash.
`TestDesktopBundledEngineResumeUsesLaunchGuardian` also retains the main and
thread identity while changing Guardian from Kimi to GLM Flash between launches,
observing a real approved command on both turns.
The existing full history/native catalog, ordinary-mode, crash recovery, and
installed lifecycle regressions retain their original coverage.

Kimi-main naming retains the captured Luna contracts and actual-engine replay.
Other mains announce unsupported naming and reject title requests without
silently using Kimi. Changed review contracts are rejected even when addressed to
the selected main; a distinct Guardian is not an ordinary conversation route.
The pinned reviewer supplies no role metadata, so changed-review detection is
bounded to its assessment-field signature or exact three-tool inspection
inventory. Ordinary main structured output with an `outcome` property remains
valid. Broader client shapes require new qualification.

Malformed model fields, unavailable or metadata-incompatible roles, and
unverified pairs fail before the target starts or before inference, as applicable.
New inactive provider entries give model-independent recovery instructions.
Exactly matching legacy entries remain accepted unchanged, including their old
Kimi-specific guidance; users must follow the conversation's recorded main.

## Checks and limits

Final checks passed; the machine-readable summary is
[`checks.json`](evidence/desktop-selection-2026-09-28/checks.json):

- `TOFA_TEST_DESKTOP_ENGINE=/Applications/ChatGPT.app/Contents/Resources/codex go test -race -json ./...`: 398 passing tests/subtests, no failures.
- `go vet ./...` and `git diff --check`.
- Offline installer tests: 2 passed; macOS qualification harness: 18 passed.
- Linux amd64 and Windows amd64 builds.
- Standards review: 0 unresolved findings. Spec review: 0 unresolved findings.

Initial red tests reproduced the Kimi-only restriction and incorrect Guardian
binding. Review regressions reproduced malformed-model admission and changed
same-model review admission; those now pass with rejection before upstream IO.
The first full run found a bridge-copy fixture that needed synthetic login/catalog
preflight after validation moved earlier; its corrected run and the final complete
suite pass. No production fallback was added to accommodate that fixture.

The installed Electron UI check and installed standalone CLI opt-in were not run.
Actual bundled-engine checks were enabled. Cross-builds do not establish native
Linux/Windows desktop coverage. This ticket does not refresh paid availability,
prices, latency or coding-quality evidence, or qualify any model pair for support.
