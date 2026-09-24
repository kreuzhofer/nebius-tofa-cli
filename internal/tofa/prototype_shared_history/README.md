# Throwaway prototype: shared desktop history (#34)

**Verdict: the combined prototype passes the human history/relaunch walkthrough.**
Ordinary mode displays tofa conversation history without the logo loop. Sending
there fails explicitly because no tofa launch credential exists. Relaunching
through tofa continues the same conversation using fresh routing. Native sessions
also continued during the earlier tofa-launch walkthrough.

The prototype combines three validated decisions:

1. Supply provider overrides at the `app-server` argument boundary. Its own config
   arguments otherwise displace root CLI overrides and cause a missing-provider
   resume loop even during an active tofa launch.
2. Give the executable referenced by desktop tool settings a durable lifetime.
   The bridge reads launch-owned routing only while the launch environment points
   to its private temporary manifest. Outside a launch it delegates to the bundled
   engine; an expired manifest fails explicitly instead of falling back.
3. Keep non-secret, inactive provider metadata in the shared scratch config. This
   permits ordinary-mode history loading without a running adapter or credential.
   The inactive definition uses loopback port zero and a deliberately absent
   credential variable. Active tofa launch overrides replace it with live routing.

This is **prototype evidence, not production completion of #34**. No ordinary user
profile was used, and no production launcher code changed. Keep this directory on
`prototype/34-shared-desktop-profile`, out of main.

## Try it

Double-click [index.html](index.html) for the in-memory logic walkthrough. It makes
no network requests or desktop changes. The page models intended behavior; the
runnable probes and recorded observations are the implementation evidence.

For the human desktop trial, run from the repository root:

```sh
python3 internal/tofa/prototype_shared_history/live_history.py
```

It creates a marked scratch profile and seeds ordinary and tofa conversations with
successful synthetic replies. It opens the same profile in ordinary → tofa →
ordinary → tofa modes. Each window stays open until Enter in the runner terminal.
Ctrl-C closes only the trial's owned desktop and routes. No sign-in is needed.
Do not enter real credentials. Scratch files are retained for inspection.

In ordinary mode, open **PROTOTYPE tofa history** and check that its messages are
readable. A send attempt should report the absent launch credential. In tofa mode,
keep Kimi selected and send a continuation; expect a synthetic Token Factory reply.
The ordinary conversation uses the native synthetic fixture.

For the automated durable-reference experiment:

```sh
python3 internal/tofa/prototype_shared_history/durable_route_probe.py
```

This exercises four real desktop startups and public engine APIs, normal cleanup,
fresh routing, credential absence from shared engine files, and explicit rejection
of expired launch manifests. It does not inspect desktop rendering. Add
`--temporary` to reproduce the original dangling executable (expected failure).

For the isolated ordinary-mode history reproduction:

```sh
python3 internal/tofa/prototype_shared_history/offline_provider_probe.py
```

Without inactive metadata, resume fails with the missing-provider error. With it,
resume/read succeed and inference still fails explicitly without a credential.
No inference service or desktop is started by this probe.

## Recorded observations

| Boundary or human action | Observed result |
| --- | --- |
| Both known session IDs across launch modes | Each appears once through the public engine list API; names, turns and workspace retained |
| Ordinary session during tofa launch | Human opened it and received native fixture continuation |
| Tofa session in ordinary mode with inactive definition | Human confirmed earlier messages and latest reply visible, with no logo loop |
| Send in ordinary mode | Explicit `Missing environment variable: TOFA_OFFLINE_PROBE_MISSING_KEY` error |
| Select Astra in that tofa session, then send | Same missing-key error; model selection did not switch the provider to OpenAI |
| Relaunch through tofa, select Kimi again, send | Same conversation returned a Token Factory synthetic reply |
| Saved desktop tool executable after cleanup | Durable bridge still callable, with unchanged bytes |
| Expired launch manifest | Explicit exit 78; no native fallback |
| Fresh tofa relaunch | New endpoint and bearer; old listeners closed |
| Account credentials | Synthetic auth file stayed unchanged |
| Cleanup after final human confirmation | Owned desktop and fixture listeners closed; temporary launch files removed; durable bridge retained |

The model picker is not a provider migration mechanism. This agrees with the
accepted limitation: relaunch through tofa to continue tofa conversations. Switching
the model to Astra is not sufficient; reselect Kimi before continuing through tofa.

## Evidence and scope

- [live-history-evidence.json](live-history-evidence.json): completed human combined
  walkthrough, including readable ordinary history, the Astra experiment and final
  tofa recovery. Automated snapshots precede each interaction; human observations
  are recorded separately. The generic runner's `ui_verified: false` fields are
  not human observations.
- [durable-reference-evidence.json](durable-reference-evidence.json): successful
  four-launch durable bridge experiment on engine `0.155.0-alpha.16.3`.
- [offline-provider-probe-evidence.json](offline-provider-probe-evidence.json) and
  [offline-provider-evidence.json](offline-provider-evidence.json): engine-only
  inactive-provider checks, followed by the human UI verification above.
- [logo-loop-evidence.json](logo-loop-evidence.json): earlier failure reproduction,
  corrected argument placement, and original ordinary-mode loop before the inactive
  definition was added.
- [temporary-reference-evidence.json](temporary-reference-evidence.json) and
  [desktop-evidence.json](desktop-evidence.json): original temporary executable
  failures. [durable-executable-evidence.json](durable-executable-evidence.json)
  records the initial non-desktop bridge checks while execution approval review
  was timing out; the later desktop run supersedes that pending status.
- [evidence.json](evidence.json): initial engine-only history experiment on
  `0.155.0-alpha.9.2`. `run.py` retains that version gate; later experiments target
  alpha.16.3. The installed engine updated between these trials.

All inference responses were local synthetic fixtures. Desktop startup can contact
vendor services for its normal metadata/plugin behavior; this is not a network
isolation experiment. Reports omit credential values, local ports, session IDs and
personal paths. Scratch files contain only synthetic credentials. The desktop
maintains its own shared tool settings; byte-identical config is not claimed.
The bridge does not restore or rewrite those settings at shutdown.

No new automated test suite was added, per the prototype skill. The executable
probes, human checks, syntax and diff checks are the validation. The HTML file's
browser automation was blocked by URL policy; it has not received a visual check.

## Next implementation work

Implement the validated shared-history, durable-executable and inactive-provider
behavior at the already-approved launcher executable/HTTP and engine-RPC test
boundaries. Qualify installation, upgrade, uninstall, concurrent ownership,
unexpected process death, enforced policy and existing-account onboarding.
Preserve native live-account model metadata; this prototype merges a shipped
catalog snapshot. Desktop auxiliary native-model requests under the tofa provider
also need qualification. No real Token Factory inference was performed here.

The interactive trial can be repeated with `live_history.py`. Earlier
`continue_desktop.py` and `resume_probe.py` remain as the source of the original
argument-placement diagnosis; their incomplete historical checks should not be
mistaken for the final combined walkthrough.
