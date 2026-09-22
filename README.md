# tofa — Token Factory launcher

A standalone CLI that launches an **already installed Codex CLI** against
Nebius Token Factory. It does not run models locally or install Codex.

The current release is [v0.1.0-rc.2](https://github.com/kreuzhofer/nebius-tofa-cli/releases/tag/v0.1.0-rc.2),
an experimental prerelease. The repository and release downloads are public;
no GitHub account is needed to install.

- Kimi-K3 with Codex 0.155.1 on macOS ARM64 passed recorded live streaming,
  tool execution and continued-conversation checks. All models still require
  `--allow-unverified`.
- rc.2 addresses the observed automatic-review request-format rejection while
  preserving Codex's approval decisions. Completed live automatic review remains
  unverified, and Codex 0.155.1's fixed 90-second review deadline can still expire
  during slow Token Factory responses.
- Release CI passed native checks on macOS ARM64, Linux amd64 and Windows amd64.
  Full real-account macOS and Windows release qualification remains pending.

See [rc.2 evidence and qualification status](docs/releases/v0.1.0-rc.2.md),
[conversation validation](docs/prototype/VALIDATION.md#request-adapter-validation)
and [the adapter investigation](docs/research/codex-kimi-followup.md).
Claude, desktop integrations, broader protocol translation and browser OAuth
remain outside the current scope.

## Installation

Use the version-pinned commands below to install **v0.1.0-rc.2** directly from
GitHub. The installer downloads only the binary matching your operating system
and CPU.

Installation is per user and needs no administrator privileges. The installer
verifies the binary's SHA-256 checksum, updates your PATH and prints the exact
command to activate tofa in your current terminal. An already installed Codex CLI
and a Token Factory API key and project ID are required to launch Codex.

### macOS and Linux

```sh
version=v0.1.0-rc.2
curl -fsSL "https://github.com/kreuzhofer/nebius-tofa-cli/releases/download/$version/install.sh" -o install.sh &&
  sh install.sh --version "$version"
```

Installs into `~/.local/share/tofa/bin`. Follow the printed PATH activation command
or open a new terminal. Bash, zsh and fish receive shell-specific instructions.

Use the same tag for the script download and `--version`. Prereleases must be
selected explicitly: the installer's default `latest` looks for a stable release
and does not select a prerelease. Add `--no-modify-path` for manual setup instructions.
Rerun the installer to upgrade; saved preferences and credentials are retained.

### Windows PowerShell

```powershell
$Version = 'v0.1.0-rc.2'
$Installer = Invoke-RestMethod "https://github.com/kreuzhofer/nebius-tofa-cli/releases/download/$Version/install.ps1" -ErrorAction Stop
& ([scriptblock]::Create($Installer)) -Version $Version
```

Installs into `%LOCALAPPDATA%\tofa\install\bin`. Follow the printed PowerShell PATH
activation command or open a new terminal.

Use the same tag for the script download and `-Version`; `latest` does not select
prereleases. Add `-NoModifyPath` for manual PATH setup.
Rerun the installer to upgrade; saved preferences and credentials are retained.

### First launch and upgrades

After installing and activating PATH:

```sh
tofa --version
tofa auth login
tofa launch codex --model 'moonshotai/Kimi-K3' --allow-unverified
```

`tofa --version` should print `tofa v0.1.0-rc.2`. Login is needed for first setup;
when upgrading, rerun the installer and reuse your saved login.

To exercise automatic approval review, select **Approve for me** in Codex's
`/permissions` menu. rc.2 preserves the approval gate; it does not select a review
mode for you. See the [automatic-review limitations](#automatic-approval-review)
below before interpreting a timeout as a request-format failure.

<a id="download-from-the-private-repository"></a>

### Optional GitHub CLI downloads

If you prefer `gh`, the following commands download and verify release assets
before installation or standalone use. They require a configured GitHub CLI
(`gh auth login`); the direct installers above do not. GitHub authentication is
separate from `tofa auth login` for Token Factory. Keep the release pinned;
`latest` does not select a prerelease.

On macOS/Linux, download the complete bundle, verify it, then use the matching
installer with the downloaded assets:

```sh
version=v0.1.0-rc.2
release_dir=$(mktemp -d)
gh release download "$version" --repo kreuzhofer/nebius-tofa-cli --dir "$release_dir" &&
  (cd "$release_dir" && shasum -a 256 -c SHA256SUMS) &&
  TOFA_RELEASE_BASE_URL="file://$release_dir" sh "$release_dir/install.sh" --version "$version"
```

On Windows amd64, download and verify the standalone executable, then launch it
directly. This path needs no installer or PATH changes. Keep `$Download` until you
are finished using this executable; replace `amd64` with `arm64` for Windows ARM64
(cross-build evidence only).

```powershell
$Version = 'v0.1.0-rc.2'
$Asset = "tofa_${Version}_windows_amd64.exe"
$Download = Join-Path $env:TEMP ([guid]::NewGuid().ToString('N'))
gh release download $Version --repo kreuzhofer/nebius-tofa-cli --dir $Download --pattern $Asset --pattern SHA256SUMS
if ($LASTEXITCODE -ne 0) { throw 'Release download failed' }
$Lines = @(Select-String -Path (Join-Path $Download 'SHA256SUMS') -Pattern ('^[0-9a-f]{64}  ' + [regex]::Escape($Asset) + '$'))
if ($Lines.Count -ne 1) { throw 'Missing or ambiguous checksum' }
$Binary = Join-Path $Download $Asset
if ((Get-FileHash -LiteralPath $Binary -Algorithm SHA256).Hash.ToLowerInvariant() -ne $Lines[0].Line.Split(' ')[0]) { throw 'Checksum mismatch' }
& $Binary --version
& $Binary auth login
& $Binary launch codex --model 'moonshotai/Kimi-K3' --allow-unverified
```

Both paths require an already installed Codex CLI. Release downloads and checksums
are verified separately from live Token Factory qualification.

### Building a distribution

```sh
sh scripts/build.sh v0.1.0-rc.2
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
The local build creates files only. Cross-compilation does not establish native
execution or real-machine qualification on every target.

### Publishing a prerelease

Push a new explicit prerelease tag such as `v0.1.0-rc.2` to the intended source
commit. Tags must use `vMAJOR.MINOR.PATCH-PRERELEASE`; stable tags and malformed
versions fail validation. Ordinary branch pushes and manual CI runs never publish.

The workflow builds one candidate bundle, then runs native race tests, vet and
actual-binary installation lifecycle checks on macOS, Linux and Windows. Every
required job must pass. Native jobs and publication download the same bundle;
publication never rebuilds the tested executables. Release CI uses synthetic state
and needs no real inference credential.

Publication uploads a private draft, downloads and compares every asset with the
checked bundle, verifies the tag still names the checked commit, then publishes it
explicitly as a prerelease. An existing release (including a partial draft) makes a
retry fail without overwriting assets or repointing the tag. Use a new candidate
version for changed release files; investigate an interrupted draft before taking
any manual recovery action. The publication job alone has `contents: write`.

All six OS/CPU artifacts have cross-build evidence. Native lifecycle execution
covers macOS ARM64, Linux amd64 and Windows amd64. Maintainer qualification with
real credential vaults and live Codex is recorded separately; native CI is not
real-machine acceptance. Apple signing/notarization and Windows publisher signing
are deferred, so record any platform prompts or blocks during qualification.

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
retry requests or follow redirects. The launcher sets provider request/stream
retry limits to zero; Codex automatic review can still retry failed review
sessions and requests independently.
Unsupported routes and oversized or encoded requests fail explicitly.
The adapter supplies missing assistant-message `status` and output-text
`annotations` in conversation history, preserving existing values so Codex can
continue a conversation through Token Factory.

### Automatic approval review

For Kimi-K3, the adapter also handles the exact non-strict automatic approval
review format observed in Codex 0.155.1. Nebius rejects tools combined with
constrained JSON generation. When the request has the recognized decision schema,
`strict: false`, `tool_choice: auto`, and exactly the reviewer function tools
`exec_command`, `write_stdin`, and `view_image`, the adapter appends the complete
schema as a final-answer instruction and removes `text.format`. It preserves
existing instructions, approval policy, context, tool definitions/results, other
text options, and response bytes. This adaptation is announced once when used.
Other Kimi tools-plus-schema shapes fail locally, including strict schemas;
other models and requests without schema/tool conflicts retain their options,
including requests that explicitly disable tool calls with `tool_choice: none`.

Codex still parses the assessment and controls execution. Installed-client tests
show that valid allow decisions execute a harmless action, while deny, malformed
JSON, missing outcomes, invalid enums, upstream failures, and cancellation leave
it blocked. Optional assessment fields can be absent, as Codex's schema permits.
The adapter adds no retries or fallback decisions and does not change Codex policy.

Automatic review is still **not live-qualified**. Codex 0.155.1 has a fixed
90-second total review deadline. In the live diagnostic, the adapted request
reached HTTP 200 only after about 242 seconds, after Codex had already stopped
waiting; a complete assessment was not observed. The format adjustment does not
extend that deadline or resolve slow provider responses. See
[the rc.2 evidence](docs/releases/v0.1.0-rc.2.md).

### Connection and client settings

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
python3 scripts/release_test.py -v
python3 scripts/install_test.py
# Native macOS/Linux lifecycle against the bundle built above:
python3 scripts/lifecycle_test.py dist v0.0.0-prototype -v
# Unix only, against a compiled local binary:
python3 scripts/terminal_test.py ./tofa
# Optional: installed Codex, scratch config, synthetic local responses only:
TOFA_TEST_CODEX="$(command -v codex)" go test ./internal/tofa -run TestInstalledCodex -v
# Unix only, offline validation of the live-test harness:
python3 scripts/live_compat_test.py
# Unix only, offline validation of the request-tracing harness:
python3 scripts/trace_codex_test.py -v
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
Windows CI also builds the versioned bundle and runs
`./scripts/windows_lifecycle_test.ps1 -Dist dist -Version v0.0.0-ci` against the
actual executable through its matching installer. It checks persistent user PATH
and fresh-process discovery, repeated install, real asynchronous CLI uninstall,
retained-state reinstall and purge. It waits for helper completion with a deadline
and checks cleanup results; launcher exit alone is not success. Failed purge,
helper cancellation, timeout and corrupt downloads must remain observable.
This check is restricted to disposable GitHub Actions Windows runners because it
temporarily changes user PATH. It restores PATH and removes its synthetic state
in `finally`, stops outstanding helpers, and never changes execution policy.
The tests use synthetic credentials, temporary directories and local HTTP servers.
They never contact Token Factory or access native credential stores. The workflow
also tests on native runners; test results must be checked before claiming coverage.

For the complete pinned macOS and Windows prerelease lifecycle, including interactive login,
saved-login reuse and sanitized local reports, see the
[local qualification runners for macOS and Windows](docs/releases/qualification.md). Their offline tests
use synthetic releases and clients. Windows qualification tests use uniquely named
synthetic vault entries only on disposable CI runners; maintainer results remain separate.

For opt-in real inference using saved credentials, see the
[live compatibility harness](docs/prototype/LIVE-COMPATIBILITY.md). It runs three
isolated Kimi sessions and records streaming, tools, checked file changes and
continuation evidence. Live runs are separate from the offline commands above.

For opt-in request diagnosis, see [Codex request tracing](docs/codex-tracing.md).
The standalone harness records sanitized request metadata, status and timing;
the released executable has no `--debug` flag. Traces must be enabled for a new
run and cannot reconstruct earlier conversations.

Design decisions: [Wayfinder map](https://github.com/kreuzhofer/nebius-tofa-cli/issues/1).
Dependencies and reuse: [third-party notices](docs/prototype/THIRD_PARTY.md).

## Uninstallation

Uninstall preserves saved preferences and credentials by default. It removes only
tofa's installation and owned PATH changes; Codex and unrelated files are retained.
If unrelated files keep the install directory nonempty, ordinary
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
version=v0.1.0-rc.2 # use the installed version, shown by tofa --version
curl -fsSL "https://github.com/kreuzhofer/nebius-tofa-cli/releases/download/$version/uninstall.sh" -o uninstall.sh &&
  sh uninstall.sh
```

To also remove saved preferences and credentials, run `sh uninstall.sh --purge`.
Linux native-store cleanup needs `secret-tool`; if it or the service is unavailable,
the script retains recovery references and reports incomplete cleanup.

### Windows PowerShell recovery script

This works even if the installed binary is broken:

```powershell
$Version = 'v0.1.0-rc.2' # use the installed version, shown by tofa --version
$Uninstaller = Invoke-RestMethod "https://github.com/kreuzhofer/nebius-tofa-cli/releases/download/$Version/uninstall.ps1" -ErrorAction Stop
& ([scriptblock]::Create($Uninstaller))
```

Append `-Purge` to also remove saved preferences and credentials. Failed native-store
cleanup is reported rather than treated as successful deletion.
