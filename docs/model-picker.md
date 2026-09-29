# Universal main-model picker contract

Codex CLI and Codex desktop share this launch interaction. New target integrations
must provide their own validation and evidence; implementing Claude remains in
[#30](https://github.com/kreuzhofer/nebius-tofa-cli/issues/30) and
[#31](https://github.com/kreuzhofer/nebius-tofa-cli/issues/31).

1. An explicit `--model ID` bypasses the picker and is validated normally, including
   an explicitly empty or invalid ID. An omitted interactive main requires
   confirmation every launch, regardless of saved preferences. An omitted
   noninteractive main fails with a target-specific `--model ID` command.
2. Resolve the project and current catalog, route, and effective Guardian before
   presenting choices. Guardian uses the target's default or explicit override,
   with no second picker. Unavailable or incompatible Guardians fail explicitly.
   The diagnostic CLI direct route retains native review and rejects an override.
3. Normal choices require evidence for the exact target, route, main and Guardian.
   Availability, metadata and another target's qualification do not grant support.
   An empty supported list explains `--allow-unverified`. This explicit opt-in
   exposes the entire current project catalog; entries lacking compatible metadata
   are disabled with reasons. An empty catalog or catalog error never broadens the
   list or silently substitutes a role.
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

The shared implementation is `mainModelChoices`, `pickMainModel` and `pickerView`.
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
