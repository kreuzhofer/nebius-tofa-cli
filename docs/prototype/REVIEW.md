# Reviewing this Go prototype

The question is whether a small launcher can offer the agreed Ollama-like flow
while preserving normal Codex use. Review user-visible behavior first, then the
credential and launch boundaries. This is a prototype, not a production release.

## Reading order

1. `cmd/tofa/main.go`: process entry, error/exit-code handling, embedded uninstall.
2. `internal/tofa/app.go`: command parsing and prompts. Compare commands to
   [the accepted contract](https://github.com/kreuzhofer/nebius-tofa-cli/issues/8#issuecomment-5759296520).
3. `internal/tofa/launch.go`: exact Codex arguments/environment and process lifecycle.
   No persistent Codex configuration is written.
4. `internal/tofa/catalog.go`: project-scoped authenticated model discovery. It
   rejects redirects and omits response bodies from errors to avoid leaking keys.
5. `internal/tofa/storage.go`: preferences, credential storage and recovery.
   Login writes a new credential before switching config; failed switching attempts
   cleanup while preserving the original login. Logout uses recovery markers.
6. `permissions_unix.go` / `permissions_windows.go`: real platform file protection.
7. `scripts/`: inspect the shell/PowerShell code independently of Go. The recovery
   uninstall scripts are also embedded in the binary, avoiding a network dependency.

## Go syntax in familiar terms

| Go | Meaning for a Python/Node/C#/Java reviewer |
| --- | --- |
| `type Config struct { ... }` | A typed record with named fields. Backtick tags specify YAML field names. |
| `func (s Store) Login(...) error` | A method on Store, returning an error or `nil` for success. |
| `value, err := operation()` | Declare locals from multiple return values. Error handling is explicit. |
| `*App` | A pointer to an App; the method can set fields on the same instance. |
| `defer cleanup()` | Run cleanup when this function returns, including early error returns. |
| `interface` | A small behavioral contract. The fake Vault replaces only the OS boundary in tests. |
| `[]string` / `map[string]string` | A list of strings / string-keyed dictionary. |
| `go func()` / channel | A goroutine and communication channel, used for signals and terminal cancellation. |
| `_windows.go`, build tags | Compile only the appropriate platform implementation. |
| `//go:embed` | Include a script in the executable at build time; no runtime Go installation needed. |

You can review meaningful behavior without becoming a Go expert: which files can
be removed, which inputs reach a child process, whether errors preserve recovery
information, and whether support claims match evidence. Compiler success alone
does not establish any of those properties; the tests cover selected boundaries.

## Deliberate limits and review questions

- There is no certified model registry yet. All catalog entries remain unverified,
  and experimental launch requires explicit permission. Do not flip a boolean to
  certify a model without a recorded Codex-version/model/platform live test.
- Provider endpoint is fixed to Token Factory. No proxy, OAuth or client installer.
- Tests substitute the native vault; they do not establish actual Keychain,
  Credential Manager or Secret Service availability.
- Per-user files assume a trusted OS account. They do not isolate secrets from
  other processes running as that same account or from a compromised target agent.
- An interrupted operation can leave `.auth-lock`; recovery is explicit. Logout
  and script purge retain references on failures rather than claiming deletion.
- SHA-256 files detect corruption and mismatches; they are not signed provenance.
- Windows asynchronous self-removal and native key-store prompts need hands-on checks.
- User shell files can contain arbitrary code. Installer tests cover ownership
  blocks, damaged markers, spaces/apostrophes and repeated installation.

Hands-on feedback should cover whether prompts are clear, the experimental-model
choice is understandable, errors give enough direction, and normal Codex behavior
is preserved. That feedback is needed before the prototype decision can close.
