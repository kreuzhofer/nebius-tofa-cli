# tofa — Token Factory launcher prototype

A standalone Go CLI that launches an **already installed Codex CLI** against
Nebius Token Factory's native Responses endpoint. It does not run models locally.
This is an experimental, reviewable prototype on `prototype/direct-launcher`.
There is no published release yet, and **no model/client combination is certified**.
Claude, desktop integrations, protocol proxies and browser OAuth are outside this prototype.

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

`auth login` asks for a hidden API key followed by the project ID. It saves locally;
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

## Install, upgrade and uninstall

The scripts are implemented and tested with local release fixtures. The following
release commands become usable **after a release is published**; no release has
been published by this work. `--version TAG` / `-Version TAG` selects a release;
rerunning upgrades. The default is the latest published release.

macOS/Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/kreuzhofer/nebius-tofa-cli/prototype/direct-launcher/scripts/install.sh | sh
curl -fsSL https://raw.githubusercontent.com/kreuzhofer/nebius-tofa-cli/prototype/direct-launcher/scripts/uninstall.sh | sh
# Also remove saved preferences and credentials:
curl -fsSL https://raw.githubusercontent.com/kreuzhofer/nebius-tofa-cli/prototype/direct-launcher/scripts/uninstall.sh | sh -s -- --purge
```

Windows PowerShell:

```powershell
& ([scriptblock]::Create((Invoke-RestMethod 'https://raw.githubusercontent.com/kreuzhofer/nebius-tofa-cli/prototype/direct-launcher/scripts/install.ps1')))
& ([scriptblock]::Create((Invoke-RestMethod 'https://raw.githubusercontent.com/kreuzhofer/nebius-tofa-cli/prototype/direct-launcher/scripts/uninstall.ps1')))
# Add -Purge to the uninstall invocation to remove saved data.
```

Install location: `~/.local/share/tofa/bin` on Unix and
`%LOCALAPPDATA%\tofa\install\bin` on Windows. No administrator privileges are needed.
Installers verify SHA-256 checksums, update the user's PATH and print the exact
current-shell activation command. `--no-modify-path` / `-NoModifyPath` prints manual
setup instead. Bash, zsh, fish and PowerShell are handled explicitly.

An installed copy supports `tofa uninstall [--purge]`. The standalone scripts
work even when the binary is broken. Default removal retains saved data; purge
removes only tofa-owned files and recorded credentials. Unrelated files and Codex
are preserved. Linux script purge needs `secret-tool` for native-store entries;
if missing/unavailable, it retains references and reports incomplete cleanup.
Windows CLI uninstall starts a helper that waits for the executable to exit and
then reports completion; watch the helper's output for errors.

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
