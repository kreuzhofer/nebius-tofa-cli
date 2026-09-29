# Desktop qualification campaign

Issue #61 extends `scripts/model_evaluation.py`, its request observer and reporting
module to the desktop launcher and pinned bundled engine. The Electron interaction
is supplied by a headless driver in a disposable bundle/profile. The real launcher
owns discovery, metadata, routing, profile ownership, request adaptation and the
engine bridge. The real engine owns tool execution, automatic review, assessment
parsing, retries, cancellation and its native 90-second review deadline. This is
engine/routing qualification, not evidence about the Electron UI or model quality
when responses come from fixtures.

The [baseline manifest](desktop-baseline.json) fixes all five exact main candidates,
the GLM Flash Guardian, three two-turn sessions (relaunching and resuming the same
conversation between turns), and three allow/deny pairs. DeepSeek is first; Kimi
uses the identical workload as the contemporaneous comparison. Arithmetic results
are checked independently against `live_compat.EXPECTED`; the evaluator also
requires successful tool events, result continuation, streaming, preserved input,
and exact resumed main/provider identity. Approval reports compare the native
review decision with actual execution/nonexecution of a harmless marker command.
The main proposals in approval cases are synthetic, including during live runs;
they isolate Guardian behavior and do not establish natural main-model behavior.

No spending cap, request quota or spending ledger applies to this desktop campaign.
Input, output, response, SSE event and wall-clock limits remain finite. Every native
attempt is recorded, including unsuccessful attempts, native retries and rejected
auxiliary contracts. Guardian inspection tool calls remain intermediate requests;
only the final assessment supplies an allow/deny decision. The report records
whether that decision came from completed output or streamed text. The observer adds no retries. Failure stops its affected lane;
other independent work remains measurable. Remaining cases stay `unattempted`;
preflight failures are `blocked` with a cause. Missing rates/usage mean unknown
cost, never a free request or an execution block. Synthetic usage estimates are
labelled explicitly and do not represent paid inference.

## Deterministic checks

Use the installed pinned engine with synthetic credentials and loopback providers:

```sh
TOFA_TEST_DESKTOP_ENGINE=/Applications/ChatGPT.app/Contents/Resources/codex \
  TOFA_TEST_EVIDENCE_DIR=/path/to/controlled-reports \
  python3 scripts/desktop_evaluation_test.py -v
python3 scripts/evaluation_proxy_test.py -v
python3 scripts/model_evaluation_test.py -v
```

A missing pinned platform/engine is a skip, not successful qualification. Tests
exercise the public evaluation command through the real `tofa.App`, desktop route,
bridge and engine. They cover failures, invalid review, provider errors, local
limits, native cancellation, restart reporting, unknown cost and sanitized output.
Normal settings and credentials are checked without copying native login state
into the synthetic home. Ordinary desktop processes are never interrupted.

## Run and recover evidence

For live inference, first freeze the final source and retain a matching successful
controlled-provider report for the selected main. Supply it as
`--qualification-evidence`; source, manifest and engine fingerprints must agree.
Completing #61 requires no paid inference. Live campaign authorization and results
belong to the subsequent campaign tickets.

On macOS, a disposable HOME hides the launcher's saved Keychain login. The
evaluator links the existing `~/Library/Keychains` directory into each disposable
home so the launcher can read that login. It does not copy or change credentials.
The engine still uses file authentication in its disposable `.codex` directory;
normal desktop settings and history are not used for these sessions. Removing
the disposable home removes the link, not the Keychain directory.

```sh
python3 scripts/model_evaluation.py --desktop \
  --launcher /absolute/path/to/tofa \
  --codex /Applications/ChatGPT.app/Contents/Resources/codex \
  --model deepseek-ai/DeepSeek-V4.1-Flash \
  --qualification-evidence /path/to/matching-controlled-report.json \
  --campaign /path/to/campaign --output /path/to/new-report.json
```

Use an **absolute `--campaign` path**. The child driver runs from its disposable
workspace; relative observation/result paths do not resolve there. Such a run
fails locally and is not model-compatibility evidence. Keep the failed run and
use a new output path when correcting the invocation.

Live preflight refreshes authenticated project availability through `tofa models`
and exact public model metadata/prices from the provider catalog. Changed or
missing required metadata blocks the affected combination until deliberately
requalified. DeepSeek's separate pinned tool-capability model card is rechecked
because the public catalog omits that flag. No other candidate supplies missing
capabilities. `--metadata-snapshot` is the controlled-provider fixture input and
must not be used to imply refreshed live evidence.

Every invocation creates a new private run directory. Observations are atomically
checkpointed before and during requests; case outcomes are checkpointed after each
turn. A process crash can leave an `incomplete` request or case; it cannot turn it
into a pass. Rebuild a report without executing or charging for another request:

```sh
python3 scripts/model_evaluation.py \
  --desktop-report /path/to/campaign/RUN_ID --output /path/to/recovered.json
python3 scripts/model_evaluation.py \
  --desktop-report /path/to/campaign --output /path/to/matrix.json
```

The matrix retains every invocation and all five candidates, including unattempted
ones. It never replaces failed runs with a successful rerun. A candidate with
multiple qualification conditions is `multiple_conditions`, with separate group
outcomes. Differing outcomes within one condition group are `mixed`. Compare only identical manifest, source,
engine, platform and metadata fingerprints. Different evidence kinds and workload
versions must remain separate. Totals read each run once; report exports are not
inputs to accounting. Keep the campaign directory intact for recovery.

Additional investigations use `--diagnostic short-purpose-id`. This repeats the
same workload in a separately identified run and retains the baseline. The
`--cancel-review` diagnostic interrupts the first in-flight native assessment;
it requires a diagnostic ID and produces incomplete evidence. Other workload or
implementation changes require a new manifest/source fingerprint and separate
reports, rather than pooling unlike observations.

## Report interpretation

Reports record exact roles, source file hashes, launcher/engine hashes and engine
version, manifest, platform, metadata fingerprint/check time, request IDs,
request start/header/first delta/completion timing, whole-turn duration, native
review duration where available, usage, refreshed rates and known cost subtotals.
Observer review duration spans upstream native attempts; native review event time
and whole-turn time remain distinct. Provider compute time is unknown. Reported
provider wait also includes adapter/transport overhead.

Titles are not a qualification gate. The native provisional first-message title
is distinct from a generated title; generated naming is unmeasured by this
baseline. Only the existing recognized Kimi naming route is supported. Other-main
naming remains explicitly unsupported, with no hidden Kimi fallback. The baseline
does not request generated titles or claim title-quality evidence. Any auxiliary
request that reaches the observer is counted and an unsupported contract fails
explicitly. Normal native-provider traffic is not part of this isolated workload.

Only fixed categories, identities, checks, counts and numeric measurements enter
reports. No prompts, tool output, files, thread IDs, credentials or reviewer
rationales are retained there. Synthetic client state is removed after each case;
sanitized observations survive in the campaign directory. Passing this narrow
workload never changes the launcher's supported-model policy.
