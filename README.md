# tofa — Token Factory launcher prototype

A standalone Go CLI that launches an **already installed Codex CLI** against
Nebius Token Factory's native Responses endpoint. It does not run models locally.
This is an experimental, reviewable prototype on `prototype/direct-launcher`.
There is no published release yet, and **no model/client combination is certified**.
Claude, desktop integrations, protocol proxies and browser OAuth are outside this prototype.

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
./tofa auth logout
```

The launcher supplies settings through child-process arguments and `TOFA_API_KEY`
in the child environment. It does not edit Codex's configuration or login files.
The child and tools it starts can access its environment; the OS credential store
protects persistence, not secrets while the agent is using them. Existing Codex
skills, hooks and policy still apply. Web search is disabled for this unverified
provider. `doctor` is local only and does not read a key or trigger inference.

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
```

Python is a **development test tool**, not a runtime or installer dependency.
The tests use synthetic credentials, temporary directories and local HTTP servers.
They never contact Token Factory or access native credential stores. The workflow
also tests on native runners; test results must be checked before claiming coverage.

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
