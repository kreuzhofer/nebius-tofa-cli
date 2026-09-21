# Prototype evidence — 2026-09-21

Status: implementation ready for hands-on review; the Wayfinder prototype decision
remains open. No claim of working live Codex inference or universal native-store
availability is made.

## Local evidence

Development host: macOS ARM64. Temporary official Go 1.27.1 toolchain, downloaded
with its SHA-256 verified. The user's system Go/PATH was not changed.
Installed client observed through `--version`/`--help`: Codex CLI 0.154.0.

| Check | Result |
| --- | --- |
| `go test -race ./...` | Passed; synthetic vault, loopback catalog and real fake-client process |
| `go vet ./...` | Passed |
| Native compiled CLI `--help` / `--version` | Passed |
| Asterisk feedback on typing/paste, both backspace encodings, plaintext opt-in login/logout | Passed in a pseudo-terminal using synthetic key and temporary HOME; plaintext key never echoed |
| Ctrl-C during key input restores terminal echo | Failed initially; fixed; passed |
| Failed native-store replacement retains prior login | Passed with fake vault |
| Config commit failure rolls back replacement | Passed with simulated filesystem failure |
| Explicit fallback, failed logout, permission/symlink rejection | Passed with scratch files/fake vault |
| Redirect rejection; project/auth catalog request; unverified gate | Passed against loopback HTTP |
| Child key in environment, not argv; parent/config preservation; exit 23 propagation | Passed with an actual fake executable |
| Unix install/rerun, PATH activation/deduplication, checksum failure, uninstall/purge | Passed with local fixtures, paths containing spaces/apostrophes |
| Damaged shell marker preserves unrelated startup text | Failed initially; fixed; passed |
| Shell script syntax | Passed |

These tests intentionally never access native key stores or real API credentials.
A fake vault establishes the launcher's behavior around failure, not OS integration.
No Token Factory request or paid inference was performed.

## Build and native-execution matrix

| Target | Cross-build | Native execution evidence |
| --- | --- | --- |
| macOS ARM64 | Passed, CGO disabled | Local and CI CLI/PTY/Unix lifecycle tests |
| macOS x64 | Passed, CGO disabled | Not run locally |
| Linux ARM64 | Passed, CGO disabled | Not run locally |
| Linux x64 | Passed, CGO disabled | CI tests/vet, Unix lifecycle and terminal tests passed |
| Windows ARM64 | Passed, CGO disabled | Not run locally |
| Windows x64 | Passed, CGO disabled | CI tests/vet, real filesystem ACL checks, PowerShell parsing and offline installer lifecycle passed |

[Native CI run](https://github.com/kreuzhofer/nebius-tofa-cli/actions/runs/35593768301)
for implementation/test commit `24aaf3c` passed its macOS ARM64, Linux x64 and
Windows x64 jobs. Runner architectures were read from the actual Go version logs.
The subsequent README/evidence edits do not change executable behavior.

The first Windows run exposed an overly strict textual ACL comparison. The fix
checks the protected DACL's actual entry type and owner SID. Windows tests now cover
real private file creation/readback and rejection of an Everyone-accessible key file.
The native OS credential store remains substituted with a fake vault.

The Windows offline lifecycle test substitutes downloads, uses a temporary
LOCALAPPDATA and `-NoModifyPath`, and covers install/rerun, failed checksum retention,
default preservation and purge without touching Credential Manager. Windows user
PATH updates, asynchronous self-removal and fish startup execution remain unverified.
Native CLI self-uninstall also passed locally in a temporary macOS installation.

## Still needed before compatibility/support claims

1. Maintainer feedback on the CLI and code walkthrough.
2. Native credential read/write/delete on each OS, including headless/locked/denied
   stores. Windows file ACL enforcement has CI coverage; actual credential-store
   checks require an isolated test account
   or explicitly authorized test store; this session did not modify user credentials.
3. Windows user PATH update and asynchronous self-removal; fish execution. The
   Windows offline install/update/uninstall lifecycle now has passing CI coverage.
4. A separately authorized paid-inference budget and test credentials, followed by
   a small candidate model set exercising real Codex streaming, tool calls and a
   continued conversation. Record exact model IDs, Codex versions and OS/CPU.
5. Confirm ordinary Codex login/provider behavior after a real launched session.
   File/environment isolation passes locally with a fake client; that is not proof
   of every Codex runtime behavior.
6. Before publishing installable releases: dependency license bundle, release assets
   and checksums. Public curl installation is not yet demonstrated because no release
   was published. Production signing/hardening remains outside this prototype.

Model compatibility registry: **empty**. Every catalog entry is marked unverified;
`--allow-unverified` is required. Browser authorization and local protocol adaptation
remain deferred research/product work, not hidden fallbacks.
