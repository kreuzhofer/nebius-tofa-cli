# Automatic titles in the shared desktop profile (#52)

Qualification for [#52](https://github.com/kreuzhofer/nebius-tofa-cli/issues/52),
following the unchanged [#50 result](../releases/desktop-shared-history-final-2026-09-24.md).
The initial #50 `not_observed` result remains historical provenance; it did not
establish a cause. This investigation reproduced two concrete failures on the
current shared-profile path. The maintainer then authorized explicit title-only
routing to Kimi. Independent model selection remains
[#41](https://github.com/kreuzhofer/nebius-tofa-cli/issues/41).

## Current client and trigger

The inspected combination is ChatGPT **26.917.71314 (10954)**, bundle
`com.openai.codex`, engine **0.155.0-alpha.16.4**, macOS **26.6.2 (25G83) ARM64**.
The source baseline is `1da2989dfbc97a7eeff64b6fb3dda474dede028c`. The accompanying
[evidence](evidence/desktop-shared-title-2026-09-24.json) pins changed production
files, binaries, installed components, effective catalog and each live attempt.

Installed archive members inspected read-only:

| Member | SHA-256 |
| --- | --- |
| `.vite/build/main-C-Mhak1n.js` | `457c79be69620d4489e94c14ac665f81731d869635606dcf175b7b4cc2e8b467` |
| `.vite/build/src-DldfpmrL.js` | `88ec69722b5d87a7081edf2e2d6a2e21c3f25300cee587363d74b8b87969a412` |
| `webview/assets/app-initial-51da50e6c6e3.js` | `ea4b3893669a56e67f6baa7d2791ed64e954b081a549f62b647101def36ef6d1` |

**Source inspection:** the renderer's `sDn` / `lDn` title path requires nonempty
text and an untitled conversation or an expected provisional title. It installs
a first-message provisional title, calls `threadMetadataGeneration.generateTitle`,
and persists a valid result through `thread/name/set`, conditional on the expected
title still matching. Null/error results persist the first-message fallback.
Existing manual titles are excluded by these guards; a visible provisional title
alone does not prove generation or persistence.

The main service `I5.generateTitle` obtains `ephemeralGenerationModel` through
`Ln` from the desktop's `codex-app-model-routing` dynamic service configuration.
Its default is `gpt-5.6-luna`. This differs from #33's older fixed-model helper.
`Wce -> X9 -> L8 -> TN` starts an ephemeral read-only thread with
`modelProvider: null`, `allowProviderModelFallback: true`, source `thread_title`,
effort `low`, and a turn with trigger `thread_title` and the title/description
schema. `TN` has a 30-second structured-result deadline; `EN` parses/validates the
answer and `Jne` normalizes the title. These are version-specific implementation
findings, not public configuration promises.

The [official configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference)
was rechecked; no documented conversation-title model or disablement setting was
found. The remote service's routing field is not a user-facing `config.toml`
setting. The launcher neither invents such a setting nor patches the client.

## Reproduction and correction

1. A fresh human-created synthetic conversation emitted a real `thread_title`
   request for **gpt-5.6-luna** to the launcher's **nebius-tofa** adapter. The
   effective catalog retained all nine native descriptors plus Kimi, including
   Luna. The actual bundled-engine regression confirms that the title helper's
   `allowProviderModelFallback: true` retains Luna when it is present, with
   provider `nebius-tofa`. The adapter rejected that unsupported model locally.
   The desktop title service logged the same failure. No provider title request
   occurred. This explains the new reproduction; #50 did not retain enough
   boundary evidence to prove its earlier observation had exactly this cause.
2. Routing that recognized request to Kimi alone exposed a second failure:
   Token Factory returned HTTP 400, `Unsupported Responses API input item type:
   'additional_tools' None`. The native model's engine serialization puts its
   code-mode tool definitions in an `additional_tools` input item, rather than
   the top-level `tools` field. An absent top-level field does **not** mean the
   request is tool-free. This failed live attempt is retained in the evidence.
3. The corrected desktop-only policy recognizes both exact title markers,
   requested model `gpt-5.6-luna`, the captured strict `codex_output_schema`,
   automatic tool choice, absent top-level tools/instructions, and one leading
   developer `additional_tools` item. Its sole `functions` namespace contains
   `exec` (custom), `wait` and `request_user_input` (functions). Unknown shapes,
   extra tool items, inventories, models and unrelated sources fail explicitly.
4. For that contract only, the launcher routes upstream to `moonshotai/Kimi-K3`,
   relocates the **complete unchanged tool definitions** to `tools`, removes only
   their former input container, and supplies the **complete title schema** as
   final-answer instructions. Remaining input, metadata, reasoning, streaming and
   other settings remain intact. As in #33, this replaces constrained decoding
   with schema guidance plus the desktop's existing validation, because Kimi's
   tools-plus-schema combination is rejected. Startup announces both routing and
   adaptation. Responses are never repaired, fabricated or retried by this policy.

The main conversation's recorded model/provider and every native catalog descriptor
are preserved. The temporary title thread still records the client's Luna choice;
the announced upstream route is Kimi. This distinction is deliberate and captured.
Guardian, main turns, summaries, compaction and other native auxiliary models do
not gain this route. CLI launches remain unchanged. The older
[#33 adaptation](desktop-title-generation.md) retains its separate captured Kimi
contract. Adding app-backed title lookups or changing the current inventory still
requires separate evidence and qualification.

The [reduced HTTP fixture](../../internal/tofa/testdata/desktop-native-title-request.json)
preserves captured routing/schema/code-mode structure with synthetic input, IDs and
tool descriptions. It is not presented as a complete raw live request. The actual
bundled engine additionally generates the real serialized request in its public
protocol regression.

## Live qualification and limitations

The human operator used fresh synthetic conversations without manually naming
them. Agent UI automation was unavailable: the computer-use tool refused this
application. Provider observations came from a temporary diagnostic entry point
calling the production `App` and **ordinary shared-profile desktop launch path**,
with an observing listener and bounded HTTP transport. This was not #33's isolated
Codex/Electron profile. Limits per diagnostic launch were six upstream requests,
512 KiB/request, 1,024 output tokens/request, a 90-second transport deadline and
a ten-minute launch deadline; the desktop title helper retained its own deadline.
Normal credentials stayed inside the launcher.

The first attempt used a 128 KiB cap, which rejected a 311 KiB main request and
caused the user-visible 502. No main inference was sent in that attempt. The cap
was explicitly raised; the next reproduction's main answer completed, independently
of its rejected title. The routing-only attempt's main answer also completed,
while its title received the provider 400 described above.

With the complete adaptation, Kimi returned the valid title **Explain dictionary
word counting** and the required description. The title is distinct from the
first-message fallback, relevant to the synthetic request, and within the client's
36-character schema. The operator confirmed that exact displayed title. A read-only
query restricted to this synthetic conversation confirmed its persisted `name`,
recorded Kimi model and `nebius-tofa` provider. A subsequent real `printf 'tofa-title-52-ok'` tool call completed; the operator confirmed the output and
unchanged title. SSE output deltas and completed events were captured.

The operator then quit the diagnostic launch, opened the ordinary desktop and
confirmed the generated title, history and an existing manual title were intact.
After Cmd-Q, the standalone production binary resumed the same conversation and
answered the no-tools recall question with `tofa-title-52-ok`. The operator again
confirmed the unchanged title, then quit with Cmd-Q; the production launcher exited
with status 0. Its final launch also replaced the scratch observer's durable bridge
with the production executable.

The successful observed run completed four requests: 44,304 input tokens and 259
output tokens, including 33,472 cached input tokens. The title itself used 5,387
input and 136 output tokens (102 reasoning). The standalone recall's engine usage
reported 16,101 input and 18 output tokens; provider request IDs and wire event
counts were not captured for that final standalone run. These are usage counts,
not a measured price or a broad model-quality claim.

Credential-file stat metadata was unchanged. The first reproduction changed the
ordinary configuration digest, but its digest-only baseline could not attribute
that change. In the later qualification interval, only `trust_level` entry
fingerprints changed; other configuration-entry fingerprints remained unchanged.
No configuration snapshot or credentials were restored. The UI used a workspace
other than the requested scratch launch directory; retained prompts and observations
are synthetic, and private workspace paths are omitted. No app-backed title lookup
or actual code-mode title tool invocation was exercised. Native account behavior,
manual-title preservation and review gates additionally retain regression coverage.

The live desktop-to-engine requested model was not directly intercepted. The
current service default and public bundled-engine replay corroborate the observed
resolved Luna request at the engine-to-adapter boundary. This distinction is retained
rather than presenting source inference as a runtime capture.

## Regression checks

The bundled-engine test first failed on the model-routing error, then failed on
the provider's `additional_tools` rejection before each corresponding correction.
HTTP regressions check intact tool definitions, complete schema guidance, unchanged
remaining request fields and responses, malformed/missing/empty/wrong-type/long
titles, failed streams, HTTP 400/429/503, changed inventories, extra tool items,
source/trigger mismatches, unsupported models, and CLI isolation. The old #33
regressions continue to cover their captured contract.

The first full race run overlapped the live desktop. Tests encountered the
existing native-owner refusal instead of their intended fixtures; that failed run
is retained and does not qualify the change. The clean final run is recorded in
the evidence with opt-in engine, installed-desktop and Guardian checks enabled.
All 310 tests/subtests passed without test-case skips (280.548 seconds for the
launcher package). `go vet ./...`, formatting and diff checks, and Linux/Windows
amd64 builds passed. Cross-builds do not establish native Linux/Windows execution.
Independent standards review found an overbroad redaction in the reduced test
fixture; the schema objects were restored, independently rechecked, and the affected
contract tests passed again. No production code changed after the full race suite.
Spec review found no material implementation mismatch; the requested-model capture
and initial settings-preservation evidence limitations above remain.

No independent naming-model choice, broader platform support, release publication,
merge or removal of `--allow-unverified` is implied.
