# Explicit main/Guardian routing — 2026-09-23

Issue [#38](https://github.com/kreuzhofer/nebius-tofa-cli/issues/38) extends the
[#36 specification](https://github.com/kreuzhofer/nebius-tofa-cli/issues/36).
The prerequisite evaluator implementation is commit `96f368c` (#37).

## Scoped invocation

```sh
go build -o /tmp/tofa-evaluation ./cmd/tofa
python3 scripts/model_evaluation.py \
  --launcher /tmp/tofa-evaluation \
  --codex /absolute/path/to/codex \
  --model 'moonshotai/Kimi-K3' \
  --guardian-model 'zai-org/GLM-5.3-Flash' \
  --output /tmp/kimi-main-glm-flash-guardian.json
```

This live invocation uses existing launcher authentication. Select the same ID
for both flags for an explicit same-model run. Omitting `--guardian-model`
explicitly selects the main model for Guardian. The example is a routing example,
not a recommendation of this pair's live compatibility.

Both IDs require bundled metadata and current project-catalog availability.
Each launch gets a temporary catalog containing each distinct role model once,
with `auto_review_model_override` on the main entry. The launcher removes it
on success and client failure. The evaluation-only launcher flag rejects direct
connections and unresolved metadata; ordinary launcher calls retain their existing
behavior, including unknown-model handling and `--allow-unverified`.

The observer accepts the main identity for coding and synthetic proposals, and
the reviewer identity for Guardian assessments in approval cases. Other identities,
swapped roles, and unrelated structured-output requests fail before upstream
inference. No request model ID is rewritten. Existing adapter restrictions,
read-only approval sandbox, policy, deadlines, request caps and repeat counts
remain unchanged. The run has no currency cap.

## Source findings versus runtime findings

The [configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference)
documents `model_catalog_json`; this implementation additionally depends on the
pinned **Codex CLI 0.155.1** source, commit
`be2951ea34f0d295ed0becf97079f92fa5f6950e`:

- [Guardian selection](https://github.com/openai/codex/blob/be2951ea34f0d295ed0becf97079f92fa5f6950e/codex-rs/ext/guardian-reviewer/src/model.rs):
  the main model's `auto_review_model_override` selects an exact reviewer preset.
  Catalog order is not the selection mechanism. A missing preset can inherit
  parent reasoning settings, which is why both roles must have metadata here.
- [Reviewer configuration](https://github.com/openai/codex/blob/be2951ea34f0d295ed0becf97079f92fa5f6950e/codex-rs/core/src/guardian/reviewer_config.rs):
  it clones the parent configuration, retaining the custom provider. The root
  `review_model` setting concerns code review, not Guardian.
- [Preset conversion](https://github.com/openai/codex/blob/be2951ea34f0d295ed0becf97079f92fa5f6950e/codex-rs/protocol/src/openai_models.rs#L869):
  an omitted default reasoning level becomes the preset effort `none`.
  Runtime requests confirm main effort is omitted and Guardian sends
  `reasoning.effort = "none"`. The reviewer uses its own preset, not the main
  model's reasoning configuration. Summaries and verbosity remain omitted.

The last finding corrects an earlier assumption that every optional effort field
would be omitted. It is a client behavior, **not verified provider support** for
that control. The final report records it explicitly. Live campaign rejections
must remain failed evidence; do not change effort or route to obtain a pass.

## Installed-client evidence

[Sanitized runtime evidence](evidence/paired-routing-2026-09-23.json) records the
real launcher (`tofa.App`), real request adapter, installed CLI 0.155.1 on macOS
ARM64, and one loopback-only synthetic provider. The fixture executable changes
only the endpoint and synthetic credential store; it does not replace launch or
review orchestration. All task and review responses in this fixture are synthetic.
No paid model inference occurred, and no model compatibility claim follows.

The recorded distinct pair is Kimi-K3 main / GLM-5.3-Flash Guardian. The explicit
same-model pair uses GLM-5.3-Flash for both roles. Each successful run checks three
two-turn coding sessions (correctness 2/2, protocol 6/6 each), three allow/deny
pairs, exact provider request identities, real client decision parsing, actual
marker execution/nonexecution, ordinary configuration preservation and temporary
catalog cleanup. Provider requests also verify the role-specific reasoning fields.

The negative fixture returns invalid assessments to the distinct review model.
Codex performs three native attempts per approval case, does not execute the
marker, and retains the main lane's pass. Assessment timing spans the first
review request through the final attempt, including native retry waits; request
and whole-turn times remain separate. Missing timing and usage remain null or
incomplete. Costs in these synthetic fixtures validate arithmetic only: for
100 input and 20 output tokens per request, 12 Kimi coding requests estimate
USD 0.0072 and six GLM Flash review requests estimate USD 0.00015. These are
hypothetical dated-price calculations, not incurred charges.

Reproduce the runtime gate with the pinned installed client (a skip is not a pass):

```sh
TOFA_TEST_CODEX=/absolute/path/to/codex python3 scripts/evaluation_client_test.py -v
```

Optionally set `TOFA_TEST_EVIDENCE_DIR` to an existing empty directory to retain
sanitized reports, including unsuccessful attempts. It refuses to overwrite them.
Reports label synthetic provider inference explicitly outside the evaluator's
upstream-eligibility accounting. Earlier development attempts are listed separately
in the retained evidence; they do not qualify the frozen configuration.

## Desktop auxiliary findings

The separately inspected desktop is **ChatGPT 26.915.31945, build 9922**, Codex
mode, with bundled engine **0.155.0-alpha.9.2**. Its `app.asar` SHA-256 is
`1f7939c1c781887c167043c4d1d307af3400d324685cfc315dfe2f80e634f483`.
The existing [desktop investigation](../research/codex-desktop-feasibility.md)
records that environment and the earlier runtime failure.

Read-only bundle inspection traces `ThreadMetadataGenerationService.generateTitle`
in `.vite/build/main-DUHZj4_w.js` to `ece` → `X9` → `z8` in
`.vite/build/src-C3YaUE83.js`. `X9` requests the fixed auxiliary model `WN`,
`gpt-5.6-luna`; the fresh-thread path uses `modelProvider: null` and
`allowProviderModelFallback: true`. No dedicated naming-model setting was
established by that inspection. This source result does not qualify title routing.
Earlier live Kimi title generation failed the existing tools-plus-schema adapter
gate before reaching Token Factory. The approval workaround remains unchanged.
CLI results establish neither desktop Guardian nor title runtime qualification.
`nvidia/Nemotron-3_5-Lightning` stays naming-only, outside this implementation.

## Frozen campaign configuration

The [freeze manifest](evidence/paired-routing-freeze.json) pins evaluator, observer,
scorer, launcher metadata/adapter sources, candidate snapshot and installed-client
fixture by SHA-256. Every live report additionally records executable/client and
harness hashes. Use this final explicit role configuration for all five standalone
runs and subsequent selected pairs, including a fresh Kimi baseline. Keep the
dated 2026-09-23 prices unless a separately documented campaign decision updates
them before measurement. Historical results are not comparable campaign reruns.

Do not change this configuration between candidates. A later defect correction
requires a new manifest and separately identified reruns of affected measurements;
preserve original attempts. Reviewer ranking and paid live campaign results are
subsequent work. Synthetic routing success does not select a reviewer, qualify a
real model, or change supported-model policy.
