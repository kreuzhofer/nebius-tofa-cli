# Desktop rc.5 qualification preparation — 2026-09-29

**Status: awaiting the maintainer's real-account desktop UI run.** This resumes
[#35](https://github.com/kreuzhofer/tofa-launcher/issues/35) after
[#41](https://github.com/kreuzhofer/tofa-launcher/issues/41). There is no published
rc.5 release and #35 is not complete. The existing rc.3/rc.4 attempts and model
campaign evidence remain unchanged.

## Immutable candidate

The final local candidate is built from
`4f3b1430b5f3ab7517d76fcd7e79da539388a447`, using Go 1.27.1 and the existing
`scripts/build.sh v0.1.0-rc.5`. The unused tag `v0.1.0-rc.5` is reserved locally at
that commit and has **not** been pushed. The tagged checkout supplies the embedded
Go module version and VCS identity; the build uses a clean git archive staged
under that checkout, CGO disabled, trimpath and the version ldflag.

- macOS ARM64 binary SHA-256:
  `e0b9b02ee3f6ecf0bcc71137e1e5dc64b7ad9e51dad274d3a0a9d17d9d799a46`.
- Desktop: ChatGPT 26.917.71314 (10954), `com.openai.codex`.
- Engine: 0.155.0-alpha.16.4, SHA-256
  `93169e745735930598e867ad837abf3fdc50774a3ad7e7aa89c0d0c51b0189a5`.
- Platform: macOS 26.6.2 ARM64.
- Main: `deepseek-ai/DeepSeek-V4.1-Flash`.
- Guardian: `zai-org/GLM-5.3-Flash`.
- Connection: installed launcher, owned desktop, launch-scoped Responses adapter.

All 18 asset hashes and checks are recorded in the accompanying
[preflight evidence](evidence/desktop-rc5-preflight-2026-09-29.json).
The final local capsule is `.qualification/desktop-rc5-2026-09-29-v2/`.
An earlier preparatory build remains in a separate capsule: its Go metadata used a
development module version because the local release tag did not yet exist. It
was not used for live inference or installed-account qualification. The final
capsule was built after reserving the local tag; no earlier bytes were overwritten.

## Coverage and remaining work

The full source race suite passed with installed engine and desktop opt-ins:
423 passing tests/subtests, no failures, `internal/tofa` 622.161 seconds. The two
optional standalone CLI inference tests were not enabled. These are synthetic
services and temporary profiles, including actual-engine approval allow/deny,
native deadlines, main/Guardian selection, history, title routing, cancellation,
startup ownership, cleanup and installed lifecycle checks. They do not establish
real-account UI continuity or paid model performance for this candidate.

The final packaging/static/offline results are retained in the evidence JSON.
The earlier PR #70 checks passed, but the first merge-commit CI attempt
[36603469532](https://github.com/kreuzhofer/tofa-launcher/actions/runs/36603469532)
exceeded the native macOS job's ten-minute limit during its final offline
qualification step. Artifacts, Linux and Windows passed. One rerun was requested;
the original cancellation is not relabelled as success. This CI job budget is
separate from native Guardian/title deadlines, which were not changed.

Computer Use explicitly refused access to `com.openai.codex`. Consequently the
visible UI observations must come from the maintainer. A one-run wizard and
preservation recorder are retained inside the local final capsule. The wizard
library is unchanged; bash syntax and Python compilation were checked. Synthetic
recorder checks verify that missing observations or configuration drift leave the
attempt incomplete, and that a complete synthetic observation set can pass without
exporting credential values. They do not exercise the real wizard or UI.

## Manual procedure

From the repository root, run:

```sh
python3 .qualification/desktop-rc5-2026-09-29-v2/observe.py run
```

The procedure assumes the ordinary installed launcher and its saved login. It
refuses inherited alternate-home, installation or live-routing overrides. The
launcher itself reuses saved credentials; the wizard performs no login or purge.
It installs the checksum-verified local candidate with `--no-modify-path` and
checks the installed binary and app/engine hashes before each launch.

Seven stages collect only fixed pass/fail observations:

1. Ordinary account/history/model-picker baseline and a short synthetic native reply.
2. Upgrade, shared-history continuity, real streaming/tool continuation, a second
   turn and the announced unsupported automatic-title behavior for DeepSeek.
3. Actual automatic approval followed by execution of a harmless explicit review
   request, then active-turn cancellation and clean exit. An unobserved review
   cannot pass merely because a command ran without a prompt.
4. Ordinary-mode return/native continuation, explicit inactive-provider refusal,
   and same-conversation recovery through the launcher.
5. Normal uninstall without purge and ordinary account/history/workspace checks.
6. Reinstall the same bytes and resume the same conversation using saved login.
7. Final ordinary-mode preservation and recorder completion.

Use only the supplied synthetic prompts and scratch workspace. The three owned
launcher sessions each have a 15-minute outer limit; native review deadlines
remain intact. No currency cap is enforced. Stop on the first failure; a retry is
a separate retained attempt. The recorder's settings/credential-file comparisons
remain in memory and export only equality booleans. It reads no native Keychain
values. Reported saved-login reuse complements file preservation; it is not a
claim to have independently compared vault contents. Configuration changes remain
visible and keep the attempt incomplete; no snapshot is restored to hide drift.

Each attempt gets a new private `ui-*/report.json` and fixed observations. Launcher
logs remain private and must be reviewed for sanitization before any publication.
Do not paste keys, account identifiers, private titles or raw conversation data.
A successful wizard report is an input to final review, not automatic permission
for the helper to publish a release. The helper has no publication operation.

## Publication gate and support boundary

After the real-account report passes, review it together with exact-source CI and
packaging evidence. Push the reserved tag only after qualification is complete;
the existing tag workflow must build and test its candidate, upload a draft,
download and compare every asset, then publish the prerelease. Compare the final
published binary against the qualified checksum and retain the tag workflow's
checksums and source identity. Any mismatch or required source change needs
investigation and a new candidate; never replace existing release assets.

The release notes and installation instructions must identify the tested exact
macOS pair, shared-session behavior, launch-only inference lifetime, and remaining
limits. Automatic title generation for the selected DeepSeek main is explicitly
unsupported; no Lightning route exists. Compaction and unqualified auxiliary
requests fail explicitly. Windows desktop is unqualified. Existing model support
records are unchanged; GLM Flash main and Kimi main remain experimental. This work
does not repeat the five-model campaign or complete the separate published-candidate
real-machine acceptance tracked by #23.
