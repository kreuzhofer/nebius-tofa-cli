# tofa — Token Factory launcher prototype

A standalone Go CLI that launches an **already installed Codex CLI** against
Nebius Token Factory's native Responses endpoint. It does not run models locally.
This is an experimental, reviewable prototype on `prototype/direct-launcher`.
There is no published release yet. Kimi-K3 with Codex 0.155.1 on macOS ARM64 passes
the recorded live qualification; all launches still require `--allow-unverified`.
Claude, desktop integrations, broader protocol translation and browser OAuth are
outside this prototype.

**Compatibility adapter:** a user test with Codex 0.154.0 and Kimi-K3 answered once,
then failed on conversation history validation. Launch now starts a private,
per-launch loopback adapter that supplies missing assistant-message `status` and
output-text `annotations`, preserving existing values. An offline test with Codex
0.155.1 exercises a tool call and continued conversation using synthetic responses.
The maintainer also reports a successful live browser-game build and follow-up
feature change using the suggested launcher flow. Kimi launches now receive a
temporary model catalog using Nebius's advertised context limit and capabilities,
removing the missing-metadata warning in the installed-client check. Broader
compatibility and platform checks are still tracked separately.
Three automated live sessions now also pass streaming, tool execution, checked file
changes and same-session follow-up, with normal settings/auth files preserved.
See [the investigation](docs/research/codex-kimi-followup.md) and
[current evidence](docs/prototype/VALIDATION.md#request-adapter-validation).

## Installation

**No release is published yet.** The scripts below are ready for a published release,
but cannot currently download one. For hands-on review now, use the locally built
artifact or follow [Try the local build](#try-the-local-build).

Once a release is available, installation is per user and needs no administrator
privileges. The installer verifies SHA-256 checksums, updates your PATH and prints
the exact command to activate tofa in your current terminal.

### macOS and Linux

```sh
curl -fsSL https://raw.githubusercontent.com/kreuzhofer/nebius-tofa-cli/prototype/direct-launcher/scripts/install.sh | sh
```

Installs into `~/.local/share/tofa/bin`. Follow the printed PATH activation command,
then run `tofa auth login`. Bash, zsh and fish receive shell-specific instructions.

To select a release, append `-s -- --version TAG` after `sh`. Use
`--no-modify-path` to receive manual setup instructions instead of automatic edits.
Rerun the installer to upgrade; saved preferences and credentials are retained.

### Windows PowerShell

```powershell
& ([scriptblock]::Create((Invoke-RestMethod 'https://raw.githubusercontent.com/kreuzhofer/nebius-tofa-cli/prototype/direct-launcher/scripts/install.ps1')))
```

Installs into `%LOCALAPPDATA%\tofa\install\bin`. Follow the printed PowerShell PATH
activation command, then run `tofa auth login`.

Append `-Version TAG` to select a release, or `-NoModifyPath` for manual PATH setup.
Rerun the installer to upgrade; saved preferences and credentials are retained.

## Try the local build

Developers need Go 1.26 or newer. Users of a compiled artifact need no Go, Python,
Node or other language runtime **for tofa itself**. Codex retains its own requirements.

```sh
go build -o tofa ./cmd/tofa
./tofa --help
./tofa doctor
./tofa auth login
./tofa models
./tofa launch codex --model '<catalog-model-id>' --allow-unverified
```

`auth login` asks for an API key (displayed as `*` while typing or pasting), followed
by the project ID. It saves locally;
it does not claim that the service accepted those credentials. `models` checks the
remote catalog with the selected project. Launch performs catalog discovery before
starting Codex. Launching Codex can incur inference charges.

Use `./tofa` for interactive selection, `--project-id ID` for a one-session override,
and `--` to pass Codex arguments. Routing flags such as `--config`, `--profile` and
`--model` cannot override tofa's provider via passthrough. Examples:

```sh
./tofa launch codex --model '<id>' --allow-unverified --project-id '<project>' -- --no-alt-screen
./tofa launch codex --model '<id>' --allow-unverified --direct
./tofa auth logout
```

The default route is announced before launch. Each launch binds its own
`127.0.0.1` port and gives Codex a random local bearer token through `TOFA_API_KEY`.
The saved Nebius key stays in the launcher, which forwards only to the fixed Token
Factory endpoint and selected project. The adapter accepts only `POST /responses`,
limits bodies to 16 MiB, streams responses, propagates cancellation and does not
retry requests or follow redirects. Codex request/stream retries are also disabled.
Unsupported routes and oversized or encoded requests fail explicitly.

`--direct` explicitly bypasses the adapter for diagnosis; this route passes the
Nebius key in the child environment and leaves history unchanged. Routes never
switch automatically. The child and its tools can read their environment; the
local token permits requests during that launch and is not an isolation boundary
against other software running under the same OS account.

The launcher supplies settings through child-process arguments and does not edit
Codex's configuration or login files. On exit it cancels active upstream requests
and closes its endpoint. Interrupts and adapter failures cancel the child; Unix
allows two seconds before killing an unresponsive child, while Windows terminates
the direct child immediately. Detached descendants are not guaranteed to terminate.
Existing Codex
skills, hooks and policy still apply. Web search is disabled for this unverified
provider. `doctor` is local only and does not read a key or trigger inference.

For `moonshotai/Kimi-K3`, a launch-scoped model catalog supplies Nebius's advertised
1,024,000-token context limit and text/image modalities, with Codex's pinned default
coding instructions preserved. Optional reasoning-effort, reasoning-summary and
verbosity controls are not advertised because their Kimi Responses behavior is
not yet established. This does not disable Kimi's own reasoning. The catalog is
removed on normal exit/failure; abrupt process termination can leave a nonsecret
`tofa-model-catalog-*.json` file in the OS temporary directory. Other model IDs keep
their existing metadata behavior. All models still require `--allow-unverified`.
See [the metadata research](https://github.com/kreuzhofer/nebius-tofa-cli/blob/03d47a502c09debc36a2072c3aa3a929beb6c38d/docs/research/kimi-provider-metadata.md)
for the provider snapshot and remaining gaps.

## Credentials and preferences

- macOS/Linux: `$XDG_CONFIG_HOME/tofa/config.yml`, default `~/.config/tofa/config.yml`.
  An XDG override must be absolute. macOS deliberately uses the same CLI convention.
- Windows: `%LOCALAPPDATA%\tofa\config.yml`.
- Default secret storage: macOS Keychain, Windows Credential Manager, Linux Secret Service.
  Linux needs a working session bus and usable login collection.
- Explicit plaintext fallback: `tofa auth login --storage file`, using a separate
  `credentials.yml`. This has private Unix modes / Windows ACLs, **not encryption**.
  Store failures never silently select this backend.
- `auth logout` removes saved local keys and retains preferences. It does not revoke
  your Nebius API key. Failed native-store cleanup retains recovery references.

Configuration is strict, versioned YAML. Example after logout:

```yaml
version: 1
project_id: your-project-id
model: optional-preferred-model-id
```

`keyring-refs/` contains nonsecret recovery identifiers so an interrupted login can
be cleaned up. Do not delete it before logout/purge. Concurrent auth changes are
rejected using `.auth-lock`. If a process was forcibly terminated, ensure no other
auth operation is running, then remove that **empty** lock directory and retry.

## Review and validation

Read [the Go walkthrough](docs/prototype/REVIEW.md) and
[the evidence and limitations](docs/prototype/VALIDATION.md).

```sh
go test -race ./...
go vet ./...
sh scripts/build.sh v0.0.0-prototype
python3 scripts/install_test.py
# Unix only, against a compiled local binary:
python3 scripts/terminal_test.py ./tofa
# Optional: installed Codex, scratch config, synthetic local responses only:
TOFA_TEST_CODEX="$(command -v codex)" go test ./internal/tofa -run TestInstalledCodexToolAndContinuationThroughAdapter -v
# Unix only, offline validation of the live-test harness:
python3 scripts/live_compat_test.py
```

Python is a **development test tool**, not a runtime or installer dependency.
The tests use synthetic credentials, temporary directories and local HTTP servers.
They never contact Token Factory or access native credential stores. The workflow
also tests on native runners; test results must be checked before claiming coverage.

For opt-in real inference using saved credentials, see the
[live compatibility harness](docs/prototype/LIVE-COMPATIBILITY.md). It runs three
isolated Kimi sessions and records streaming, tools, checked file changes and
continuation evidence. Live runs are separate from the offline commands above.

Design decisions: [Wayfinder map](https://github.com/kreuzhofer/nebius-tofa-cli/issues/1).
Dependencies and reuse: [third-party notices](docs/prototype/THIRD_PARTY.md).

## Uninstallation

Uninstall preserves saved preferences and credentials by default. It removes only
tofa's installation and owned PATH changes; Codex and unrelated files are retained.

### Using the CLI

For an installed copy:

```sh
tofa uninstall
```

To also remove saved preferences and credentials, use `tofa uninstall --purge`
instead. Windows starts a helper that waits for tofa to exit; watch its output for
the completion result or cleanup errors.

### macOS and Linux recovery script

This works even if the installed binary is broken:

```sh
curl -fsSL https://raw.githubusercontent.com/kreuzhofer/nebius-tofa-cli/prototype/direct-launcher/scripts/uninstall.sh | sh
```

To also remove saved preferences and credentials, append `-s -- --purge` after `sh`.
Linux native-store cleanup needs `secret-tool`; if it or the service is unavailable,
the script retains recovery references and reports incomplete cleanup.

### Windows PowerShell recovery script

This works even if the installed binary is broken:

```powershell
& ([scriptblock]::Create((Invoke-RestMethod 'https://raw.githubusercontent.com/kreuzhofer/nebius-tofa-cli/prototype/direct-launcher/scripts/uninstall.ps1')))
```

Append `-Purge` to also remove saved preferences and credentials. Failed native-store
cleanup is reported rather than treated as successful deletion.
