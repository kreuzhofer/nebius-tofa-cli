# Credential storage and native installer conventions

Research for [Establish credential storage and native installer conventions](https://github.com/kreuzhofer/nebius-tofa-cli/issues/12), checked 2026-09-21. Scope: the standalone Go launcher using direct Token Factory connections. This report recommends choices for maintainer review; it does not settle the command design. No credentials, operating-system settings, or installed software were accessed or changed.

## Recommendation for discussion

Use native credential storage by default, with the API key separate from editable configuration. If storage is unavailable, explain the failure and offer an **explicit** plaintext-file option; never silently downgrade. Keep endpoint, project ID, model selection, credential backend and a credential reference in `config.yml`. The reference is an identifier, not the key. A configuration backup consequently cannot restore authentication by itself.

An owner-restricted plaintext backend is a reasonable convenience tradeoff for headless environments when the user knowingly selects it. It should use a separate credentials file so that routine configuration sharing, debugging and backups do not automatically include the key. Native storage adds availability and integration work; plaintext alone is simpler but exposes the key to anything able to read that file. Neither choice protects a key while an authorized agent uses it.

## Established credential-store behavior

| Platform | Native option | Operational limits |
| --- | --- | --- |
| macOS | Keychain Services stores small secrets in an encrypted database. | Access and unlocking must succeed in the actual launch session. Apple distinguishes file-based and Data Protection keychains; the `security` tool primarily targets file-based keychains. Do not assume iOS protection-class guarantees apply to that tool. |
| Windows | Generic Credential Manager entries belong to the user's credential set. | `CredWriteW` uses the current token's logon session; network logons can lack a credential set. `CRED_PERSIST_LOCAL_MACHINE` survives subsequent logons for the same user on that computer. |
| Linux | Secret Service exposes secrets through a service in the user's login session over D-Bus. | A service, reachable session bus and usable collection are prerequisites. Unlocking can require a prompt, and availability cannot be presumed on headless machines. |

Sources: [Apple Keychain Services](https://developer.apple.com/documentation/Security/keychain-services), [Apple's Mac keychain implementations](https://developer.apple.com/documentation/technotes/tn3137-on-mac-keychains), [CredWriteW](https://learn.microsoft.com/en-us/windows/win32/api/wincred/nf-wincred-credwritew), [Windows credential persistence](https://learn.microsoft.com/en-us/windows/win32/api/wincred/ns-wincred-credentialw), [Secret Service introduction](https://specifications.freedesktop.org/secret-service/latest/ch01.html) and [locking behavior](https://specifications.freedesktop.org/secret-service/latest/unlocking.html). The latter specification permits services to expose unlocked items to other clients; lookup attributes are not secret. Storage is usually encrypted, rather than encryption being an unconditional API guarantee.

Windows DPAPI is another implementation option: protect a blob in an application-owned file, normally bound to the same user and computer. It adds application responsibility for file lifecycle and recovery. Avoid `CRYPTPROTECT_LOCAL_MACHINE` for a personal API key: Microsoft says any user on that computer can decrypt such data. This flag is distinct from Credential Manager's similarly named persistence setting above. [DPAPI documentation](https://learn.microsoft.com/en-us/windows/win32/api/dpapi/nf-dpapi-cryptprotectdata).

**Go feasibility:** [go-keyring's own README](https://github.com/zalando/go-keyring) describes a statically linked implementation without C bindings, supporting Windows, macOS and D-Bus systems. It depends on `/usr/bin/security` on macOS and a Secret Service implementation with a default `login` collection on Linux. It is a candidate, not a selected dependency or evidence of universal availability. Before selection, pin a revision and inspect key transport, subprocess arguments, error handling and deletion semantics; never copy examples that put a real key in command history. Native-API alternatives may introduce cgo/build requirements and require separate evaluation.

## Paths, format and file permissions

The [XDG specification](https://specifications.freedesktop.org/basedir/latest/) places configuration under an absolute `$XDG_CONFIG_HOME`, defaulting to `~/.config`; relative values are invalid. Go's [`os.UserConfigDir`](https://pkg.go.dev/os#UserConfigDir) follows that convention on Unix, but returns `~/Library/Application Support` on macOS and `%AppData%` on Windows. Thus the proposed Mac `~/.config/tofa/config.yml` is a deliberate CLI convention, not Go's native default.

Two coherent choices remain:

- Follow native locations: Linux XDG, macOS Application Support, Windows AppData.
- Follow XDG on both Unix platforms, including macOS, and use a Windows native location. This best matches the maintainer's proposed Unix path; document the deliberate macOS policy and honor an absolute XDG override.

For this locally installed launcher, recommend `%LOCALAPPDATA%\tofa\config.yml` on Windows if config includes machine-specific paths and local credential references. `%APPDATA%` is the alternative for intentionally roaming preferences and matches Go's default. Do not assume a roaming config carries a usable local secret. Microsoft distinguishes [LocalAppData and RoamingAppData known folders](https://learn.microsoft.com/en-us/windows/win32/shell/knownfolderid); resolve the chosen location instead of hard-coding a username.

YAML is acceptable for this small editable configuration. YAML, JSON and TOML are serialization choices, not encryption; changing extension does not protect an API key. Recommend a versioned, strict schema with string-valued project/model IDs and actionable unknown-field errors. Keep implementation-owned secrets out of this document regardless of format.

For an explicitly chosen plaintext backend, recommend a private directory (`0700`) and file (`0600`) on Unix, with secure creation and checks for unsafe existing permissions or symlinks. These modes restrict other users; they do not encrypt bytes or exclude processes running as the owner, privileged access or copied backups. **On Windows, Go's `Chmod(0600)` does not establish an owner-only ACL**: only its owner-write bit controls the read-only attribute. Use Windows security descriptors/ACLs and verify the result. Default ACLs inherit from the parent directory. Sources: [Go Chmod](https://pkg.go.dev/os#Chmod), [Windows file security](https://learn.microsoft.com/en-us/windows/win32/fileio/file-security-and-access-rights).

## Proposed login and logout behavior

`tofa auth login` should read the pasted key without terminal echo and prompt separately for project ID. Avoid key-valued command-line flags, logs, diagnostics and config output. Show which backend will persist it. Treat missing service, locked store, denied access and absent credential as different outcomes. Complete storage before claiming success, and preserve an existing working login if replacement fails. Credential validation against Token Factory, if included, needs separately defined request and failure semantics; successful local persistence alone is not remote validation.

Recommend `tofa auth logout` remove tofa's local saved key and corresponding reference, with idempotent behavior when already absent. This proposed meaning does not revoke the remote API key, erase copies in backups or remove credentials independently saved by coding agents. Never report complete deletion when the selected store could not be accessed. Whether logout retains project/model preferences remains a maintainer choice.

## PATH activation and uninstall

A downloaded installer executed in a child shell cannot change its parent's environment. Persistent PATH changes and immediate activation are separate operations. Bash documents [separate command environments](https://www.gnu.org/software/bash/manual/html_node/Command-Execution-Environment.html); PowerShell distinguishes inherited process scope from [persistent user scope](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_environment_variables?view=powershell-7.5).

Recommend printing an exact, correctly quoted activation command for the selected install directory: `export PATH="…:$PATH"` for Bash/zsh, `fish_add_path …` for fish, and a `$env:Path` prepend for PowerShell. Persistent shell edits should be owned and idempotent. zsh distinguishes login `.zprofile` from interactive `.zshrc` and honors `ZDOTDIR`; [fish_add_path](https://fishshell.com/docs/current/cmds/fish_add_path.html) supports persistent universal paths and avoids duplicates. See [zsh startup files](https://zsh.sourceforge.io/Doc/Release/Files.html). Do not require reloading an entire arbitrary startup file or promise a new Windows tab always inherits updated PATH.

Provide standalone `uninstall.sh` and `uninstall.ps1` that work without a healthy tofa binary; optional CLI uninstall should follow the same policy. Recommend removing only the owned executable and owned PATH changes, never a shared bin directory or installed coding agents. Default retention of config/credentials plus explicit purge is a proposal for human approval. Purge must report residual native-store entries if required services/tools are unavailable; filesystem removal alone cannot prove those entries were deleted. Shell startup edits also cannot remove a PATH entry from an already running parent shell.

Outstanding implementation checks: native credential access and deletion on all three operating systems, headless Linux failure handling, Windows ACLs, masked-input cancellation, and installer activation/uninstallation in each supported shell. This documentary investigation performed none of those runtime checks.
