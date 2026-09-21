# tofa — Token Factory launcher prototype

A standalone Go CLI that launches an **already installed Codex CLI** against
Nebius Token Factory's native Responses endpoint. It does not run models locally.
This is an experimental, reviewable prototype.
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
version=v0.1.0-rc.1 # select an existing published tag
curl -fsSL "https://github.com/kreuzhofer/nebius-tofa-cli/releases/download/$version/install.sh" -o install.sh &&
  sh install.sh --version "$version"
```

Installs into `~/.local/share/tofa/bin`. Follow the printed PATH activation command,
then run `tofa auth login`. Bash, zsh and fish receive shell-specific instructions.

Use the same tag for the script download and `--version`. Prereleases must be
selected explicitly: the installer's default `latest` looks for a stable release
and does not select a prerelease. Add `--no-modify-path` for manual setup instructions.
Rerun the installer to upgrade; saved preferences and credentials are retained.

### Windows PowerShell

```powershell
$Version = 'v0.1.0-rc.1' # select an existing published tag
$Installer = Invoke-RestMethod "https://github.com/kreuzhofer/nebius-tofa-cli/releases/download/$Version/install.ps1" -ErrorAction Stop
& ([scriptblock]::Create($Installer)) -Version $Version
```

Installs into `%LOCALAPPDATA%\tofa\install\bin`. Follow the printed PowerShell PATH
activation command, then run `tofa auth login`.

Use the same tag for the script download and `-Version`; `latest` does not select
prereleases. Add `-NoModifyPath` for manual PATH setup.
Rerun the installer to upgrade; saved preferences and credentials are retained.

### Building a distribution

```sh
sh scripts/build.sh v0.1.0-rc.1
```

An explicit version is required. A successful build replaces the generated
`dist/` directory with one complete distribution; old artifacts are removed.
A failed compilation leaves the previous distribution intact and exits with an
error. The directory contains:

- `tofa_TAG_darwin_amd64`, `tofa_TAG_darwin_arm64`, `tofa_TAG_linux_amd64`,
  `tofa_TAG_linux_arm64`, `tofa_TAG_windows_amd64.exe`, and `tofa_TAG_windows_arm64.exe`.
- Matching `install.sh`, `install.ps1`, `uninstall.sh`, and `uninstall.ps1`.
- `LICENSE` (project MIT license), `THIRD_PARTY_NOTICES.txt`, `LICENSE-GO.txt`,
  `PATENTS-GO.txt`, `LICENSE-CODEX.txt`, `NOTICE-CODEX.txt`, and this `README.md`.
- `SHA256SUMS`, covering every other file in the distribution.

Retain the license and notice files when redistributing standalone binaries.
The local build creates files only; tag-triggered publication is tracked in
[#20](https://github.com/kreuzhofer/nebius-tofa-cli/issues/20). Cross-compilation
does not establish native execution or real-machine qualification on every target.

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
- Fresh logins prefer macOS Keychain, Windows Credential Manager, or Linux Secret
  Service. A read-only lookup of a fresh probe reference checks availability; it
  does not save a test credential. The vault may request access or unlocking.
- Automatic file storage is selected only when the vault facility is confirmed
  absent or unsupported (for example, D-Bus reports no Secret Service, the macOS
  keychain helper is missing, or the keyring library does not support the platform).
  Before requesting the key, the launcher announces the choice, the path to
  `credentials.yml` beside configuration, and that the key is **unencrypted**.
  Private Unix modes / Windows ACLs restrict access; they do not encrypt the key.
- Locked vaults, denied access, uncertain availability, and failed operations are
  errors. A missing or broken Linux session bus does not prove Secret Service is
  absent; restore the bus/store or explicitly use `tofa auth login --storage file`.
  A Linux vault also needs a usable login collection.
- Re-login reuses the saved backend unless `--storage keyring` or `--storage file`
  overrides it. Explicit keyring selection fails if the vault is unavailable and
  never falls back to files. Normal launches use the saved backend without probing
  availability or migrating credentials as the environment changes.
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
python3 scripts/build_test.py
python3 scripts/install_test.py
# Native macOS/Linux lifecycle against the bundle built above:
python3 scripts/lifecycle_test.py dist v0.0.0-prototype -v
# Unix only, against a compiled local binary:
python3 scripts/terminal_test.py ./tofa
# Optional: installed Codex, scratch config, synthetic local responses only:
TOFA_TEST_CODEX="$(command -v codex)" go test ./internal/tofa -run TestInstalledCodexToolAndContinuationThroughAdapter -v
# Unix only, offline validation of the live-test harness:
python3 scripts/live_compat_test.py
```

Python is a **development test tool**, not a runtime or installer dependency.
The build tests require Go and run on macOS/Linux. They inspect all six targets'
embedded versions and build metadata, execute the native binary, verify every
checksum and bundled notice, and install from a controlled local release source.
The native lifecycle checks install the actual candidate using its bundled script
and check fresh interactive zsh (macOS) or bash (Linux) discovery, version/help,
repeated installation, CLI uninstall, reinstall and explicit purge. They also
verify that corrupt downloads and failed cleanup report errors, retain retryable
state, and preserve unrelated files. These checks run in the native CI jobs;
fish startup and other shell modes remain outside this coverage.
Run `./scripts/windows_test.ps1` in native Windows PowerShell for the offline
installer and recovery-script tests. Its `-InstallerOnly` switch checks downloads
and checksum rejection without running the Windows recovery script.
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
On macOS/Linux, if unrelated files keep the install directory nonempty, ordinary
uninstall retains its ownership marker so a later install can reuse that directory.
Explicit purge removes that marker as well. To reinstall after purge, select an
empty install directory or move the unrelated files out of the old one first.

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
version=v0.1.0-rc.1 # use the installed version, shown by tofa --version
curl -fsSL "https://github.com/kreuzhofer/nebius-tofa-cli/releases/download/$version/uninstall.sh" -o uninstall.sh &&
  sh uninstall.sh
```

To also remove saved preferences and credentials, run `sh uninstall.sh --purge`.
Linux native-store cleanup needs `secret-tool`; if it or the service is unavailable,
the script retains recovery references and reports incomplete cleanup.

### Windows PowerShell recovery script

This works even if the installed binary is broken:

```powershell
$Version = 'v0.1.0-rc.1' # use the installed version, shown by tofa --version
$Uninstaller = Invoke-RestMethod "https://github.com/kreuzhofer/nebius-tofa-cli/releases/download/$Version/uninstall.ps1" -ErrorAction Stop
& ([scriptblock]::Create($Uninstaller))
```

Append `-Purge` to also remove saved preferences and credentials. Failed native-store
cleanup is reported rather than treated as successful deletion.
