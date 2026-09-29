# Universal main-model picker contract

Codex CLI and Codex desktop share this launch interaction. New target integrations
must provide their own validation and evidence; implementing Claude remains in
[#30](https://github.com/kreuzhofer/tofa-launcher/issues/30) and
[#31](https://github.com/kreuzhofer/tofa-launcher/issues/31).

1. Interactive bare launches first select a target client (Codex CLI or Codex
   desktop) through the same terminal UI, before credentials/catalog discovery.
   Explicit `launch TARGET` skips this step. Desktop is disabled outside macOS
   ARM64; detailed installation/version checks still run at launch. Noninteractive
   bare calls retain their existing CLI behavior; scripts should name the target.
   An explicit `--model ID` bypasses the model picker and is validated normally, including
   an explicitly empty or invalid ID. An omitted interactive main requires
   confirmation every launch, regardless of saved preferences. An omitted
   noninteractive main fails with a target-specific `--model ID` command.
2. Resolve the project and current catalog, route, and effective Guardian before
   presenting choices. Guardian uses the target's default or explicit override,
   with no second picker. Unavailable or incompatible Guardians fail explicitly.
   The diagnostic CLI direct route retains native review and rejects an override.
3. Show supported and experimental choices together, labelled for the exact
   target, route, main and Guardian. Availability, metadata and another target's
   qualification do not grant support. Entries lacking compatible metadata are
   disabled with reasons. Selecting an experimental entry requires a separate
   confirmation: Y launches once; N/Enter returns to the list; Esc cancels.
   `--allow-unverified` supplies this consent in advance. Explicit and scripted
   experimental `--model ID` launches still require that flag. An empty catalog
   or catalog error never broadens the list or silently substitutes a role.
4. Display the target and route. Up/Down selects ready models; Enter confirms.
   Filtering, paging, unavailable-model inspection and details use the shared UI.
   Disabled rows cannot launch. Escape/Ctrl-C cancels. Restore the previous screen,
   cursor and terminal settings on confirmation, cancellation and errors.
5. Revalidate the confirmed model and combination before launching. Start client
   processes and adapters only after confirmation. Target integrations retain
   their ownership, credential, history, routing and approval-policy protections.
   Print the effective main, Guardian, route and support status. Selection does
   not save a preference or alter credentials. Desktop conversations retain their
   recorded main/provider, as required by ADR 0001.

Current desktop support records promote DeepSeek V4.1 Flash and GLM 5.3 mains
with GLM 5.3 Flash Guardian on the pinned adapted macOS desktop configuration.
CLI and other combinations remain experimental. See the
[five-model comparison and launch guidance](evaluation/desktop-comparison-2026-09-29.md).
Naming remains unsupported for both promoted mains.

The shared implementation is `mainModelChoices`, `pickTargetClient`, `pickMainModel`,
and the common terminal runner with `pickerView`.
Callers supply an evidence target and a display name; they own target-specific
startup and policy checks. No new target is supported merely by supplying a name.

Test at the public launcher/PTY boundary with real keys, and through the target's
installed engine/provider boundary where available. Assert selected upstream
identity and policy enforcement, cancellation without startup, explicit errors,
terminal restoration and preservation of native state. Synthetic support records
must remain test-only. Offline routing tests do not constitute model qualification.

Desktop PTY checks run in `TestDesktopPicker` on macOS 26.6.2 arm64. The optional
`TOFA_TEST_DESKTOP_ENGINE` enables actual bundled-engine approval tests. CLI PTY
checks are in `scripts/picker_test.py`; native Windows console/ConPTY coverage
remains outstanding. Cross-builds establish compilation only.
