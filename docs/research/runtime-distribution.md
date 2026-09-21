# Runtime and distribution choices for a reviewable standalone launcher

Research date: 2026-09-21. Ticket: [Compare standalone runtimes and cross-platform distribution for a reviewable launcher](https://github.com/kreuzhofer/nebius-tofa-cli/issues/5).

## Recommendation to discuss

Shortlist **Go** and **C# Native AOT**. Go offers a straightforward native CLI distribution path and lets the project reuse relevant Go implementation patterns from Ollama. C# offers immediate maintainer reviewability and also satisfies “users install no language runtime.” Neither is mandatory. Choosing Go solely because an agent says its implementation works would undervalue maintainability.

My recommendation is conditional: choose Go if the maintainer is comfortable learning enough to own a small, deliberately plain codebase; choose C# if reviewing familiar source from the first change matters more than simpler cross-compilation. A short review exercise on one launch/configuration function would answer that human question better than another language comparison. This is a proposed next decision, not a language selection.

Current scope is a **direct Token Factory launcher**, not a protocol proxy. Its responsibilities are executable discovery, model metadata/selection, configuration, credentials, child-process lifecycle, and installation. The target agents remain separately installed applications with their own requirements. This report does not build or benchmark an implementation; all performance and compatibility claims below are bounded to the cited documentation.

## What “standalone” does and does not mean

The useful promise is: **the user does not first install Go, Python, Node, Java, or .NET to run our downloaded launcher**. A binary can bundle a runtime and satisfy that promise. This is distinct from having no runtime internally, no OS libraries, no certificate store, no shell, or one file that runs on every OS/CPU.

Proposed release targets, pending support decisions: macOS ARM64 and x64; Windows ARM64 and x64; Linux ARM64 and x64. Linux libc/minimum-distribution constraints need explicit handling. A six-target build is not evidence that six targets work. Build availability, clean-machine execution, and agent-integration testing are separate gates.

## Comparison

| Approach | User-installed language runtime | Build/distribution tradeoff | Reviewability for this maintainer | Shortlist assessment |
| --- | --- | --- | --- | --- |
| Go, primarily standard library, no cgo | None | Per-OS/CPU native executable; convenient cross-target builds; inspect actual native linkage | New syntax and conventions, manageable learning scope | Strong candidate |
| C# Native AOT | None | Per-target native executable; native toolchain and OS-specific CI; AOT-compatible dependencies | Familiar C# | Strong candidate |
| C# self-contained single-file | None | Bundles runtime; native libraries may require extraction; per-target output | Familiar C# | Practical fallback if AOT restrictions impede prototype |
| Node SEA | None | Embeds Node and bundled script; version-sensitive packaging, native dependencies need care | Familiar JS/TS | Viable, less attractive if packaging simplicity is central |
| Python with PyInstaller | None | Bundled interpreter; build on each target OS; one-file extraction behavior | Familiar Python | Viable, but more packaging machinery for this small CLI |
| Java jpackage / GraalVM Native Image | None with runtime image or native output | Per-platform packaging or native-image toolchain/reachability work | Familiar Java | Possible; little reason to prefer without a Java-specific benefit |

Evidence for the packaging claims follows by technology. Rankings are engineering judgment for this project, not benchmark results.

### Go

`go build` emits an executable; users run that output rather than `go run`. [Go compile/install tutorial](https://go.dev/doc/tutorial/compile-install). `GOOS` and `GOARCH` select targets. `-trimpath` removes build-machine paths, and build metadata can be embedded/inspected. Pin the toolchain, module versions, and build flags for release reproducibility; do not claim byte-for-byte reproduction until independently compared. [Go command reference](https://pkg.go.dev/cmd/go).

Aim for `CGO_ENABLED=0` only when the chosen dependencies support it. cgo is disabled by default during cross-compilation; enabling it requires a C cross-compiler. Dependencies on native credential stores or other platform APIs can alter the build story. A no-cgo build should still be inspected and run on supported OS versions; “Go” is not a blanket guarantee of zero dynamic system dependencies. [cgo documentation](https://pkg.go.dev/cmd/cgo).

Illustrative CI build, not executed here:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o dist/nebius-tofa ./cmd/nebius-tofa
```

### C#

Native AOT compiles at publish time and does not require .NET installed on the destination. It supports Windows, Linux, and macOS targets, including x64/ARM64. Limitations include runtime code generation, dynamic assembly loading, trimming, and some reflection-dependent libraries; use AOT analyzers and resolve warnings, not blanket suppression. Linux output inherits minimum-library/platform constraints from its build environment. [Native AOT overview](https://learn.microsoft.com/en-us/dotnet/core/deploying/native-aot/).

Native AOT officially **does not support cross-OS compilation**. Cross-architecture builds are possible with the necessary target tools/libraries. Plan separate macOS, Windows, and Linux build jobs. [Native AOT cross-compilation](https://learn.microsoft.com/en-us/dotnet/core/deploying/native-aot/cross-compile).

Self-contained single-file publishing is a different option: it bundles the runtime rather than AOT-compiling everything. `IncludeNativeLibrariesForSelfExtract` embeds native runtime libraries for extraction; “single file” does not imply extraction-free execution. [Single-file deployment](https://learn.microsoft.com/en-us/dotnet/core/deploying/single-file/overview). Microsoft lists OS libraries needed by self-contained Linux apps, including libc/OpenSSL/ICU-related dependencies depending on platform/configuration. Verify the selected build's real requirements rather than distributing a generic promise. [Ubuntu runtime dependencies](https://learn.microsoft.com/en-us/dotnet/core/install/linux-ubuntu-decision#dependencies).

### Node, Python, Java

Node SEA distributes an embedded Node executable without requiring Node installation. Current **26.9.0** docs mark SEA active development; built-in `--build-sea` arrived in **25.5.0**. CommonJS and ESM are documented. The generating and target Node versions must match; cross-platform builds must disable code cache and snapshots. A production choice should pin a supported release line and use its versioned docs, not assume current-release SEA features exist in older LTS. [Node SEA](https://nodejs.org/api/single-executable-applications.html).

PyInstaller bundles the interpreter and dependencies and supports macOS/Linux/Windows, but is not a cross-compiler. [PyInstaller manual, 6.22.3](https://pyinstaller.org/en/stable/index.html). One-file mode extracts support files into a temporary directory before execution; noexec temporary mounts and abrupt termination need consideration. OS libraries are not all bundled. This is genuine no-installed-Python distribution, with operational behavior to validate. [PyInstaller operating modes](https://pyinstaller.org/en/stable/operating-mode.html).

Java `jpackage` creates platform-specific packages with a runtime image, so users need no separate JRE; packages must be produced on their target platform. Its output is usually an application image/installer rather than one portable binary. [JDK 25 packaging guide](https://docs.oracle.com/en/java/javase/25/jpackage/packaging-overview.html). GraalVM Native Image instead compiles reachable application/runtime code into a native executable; dynamic features can require reachability metadata. It is an alternative worth considering only if Java ownership outweighs that extra build discipline. [GraalVM Native Image](https://www.graalvm.org/latest/reference-manual/native-image/).

No comparative binary sizes, startup timings, or memory figures were measured. This report does not use presumed performance differences to decide the language.

### Reuse versus reimplementation

Ollama's repository license is MIT: it permits use/modification/distribution subject to preserving the copyright and permission notice in copies or substantial portions. [Ollama LICENSE](https://github.com/ollama/ollama/blob/main/LICENSE). Go may make selected source reuse easier, but copying a launcher that depends on Ollama's server/model state could import unwanted architecture. In either language, extract the launch/configuration patterns needed for direct connections; record original commit/file provenance for adapted code, keep required notices, and inspect dependency licenses. This is a project recommendation, not a reason to import Ollama wholesale.

## Launching interactive agents is an OS problem in every language

For the first terminal prototype, inherit the existing terminal's input/output/error and launch the agent directly with an argument list. Avoid introducing a PTY unless a real feature requires terminal mediation. This is a design recommendation that still needs interactive testing.

Go `os/exec` does not automatically invoke a shell. `Cmd` supports arguments, working directory, environment, inherited files, and exit status. Windows command-line parsing differs for `cmd.exe`/batch files; an npm-installed `.cmd` shim needs deliberate handling. `CommandContext` defaults to killing the process when canceled, which is not automatically a graceful process-tree shutdown policy. [Go subprocess reference](https://pkg.go.dev/os/exec). Signals differ across systems; Windows console events are not Unix process-group semantics. [Go signal reference](https://pkg.go.dev/os/signal).

C# provides `ProcessStartInfo.ArgumentList` to handle argument escaping. It does not make shell parsing or arbitrary interpolated commands safe. [Microsoft ArgumentList documentation](https://learn.microsoft.com/en-us/dotnet/api/system.diagnostics.processstartinfo.argumentlist). Node `spawn` supports inherited stdio and environment; its docs separately explain Windows `.bat`/`.cmd` handling. [Node child processes](https://nodejs.org/api/child_process.html).

A custom embedded terminal would require additional terminal-state, resize, and lifecycle behavior. Windows ConPTY has explicit creation, communication, resizing, and teardown APIs; it is not equivalent to piping stdout. [Microsoft pseudoconsole guide](https://learn.microsoft.com/en-us/windows/console/creating-a-pseudoconsole-session). Keep this outside the minimal prototype unless testing establishes it is needed.

Desktop integration has a different lifetime: launching a GUI may activate an existing process, and restarting it may be necessary before changed configuration applies. A successful process-start return cannot prove the app adopted the endpoint. The companion client investigation records those vendor-specific contracts; no runtime choice removes them.

## Installer and release design

The repository can host `install.sh`, which downloads prebuilt release artifacts. It should not compile from source or require Python/Node/Go. Provide a separate `install.ps1` for native Windows. A Unix shell script is not a native PowerShell installer, even where `curl.exe` exists.

Proposed Unix one-liner shape, **not a currently published installer**:

```sh
curl -fsSL https://raw.githubusercontent.com/kreuzhofer/nebius-tofa-cli/<release-tag>/install.sh | sh
```

Use `sh` only if the script is actually POSIX-compatible; otherwise name the needed shell. `curl -f` makes HTTP errors failures; `-L` follows redirects and `-sS` retains error messages without progress noise. [curl manual](https://curl.se/docs/manpage.html). A release-pinned bootstrap is reviewable/repeatable; a convenience “latest” URL can be offered separately. A downloaded script still starts the trust chain: verifying its downloaded binary does not independently authenticate the bootstrap itself.

PowerShell can download the Windows installer with `Invoke-WebRequest`; specify failure handling, temporary-file cleanup, and architecture selection in the script. Test supported Windows PowerShell and PowerShell versions explicitly. Avoid requiring PowerShell 7 merely because development used it. [Invoke-WebRequest](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.utility/invoke-webrequest).

Recommended installer behavior, a project proposal:

1. Detect OS/CPU, choose a versioned artifact, reject unsupported combinations clearly.
2. Download to a temporary location; verify its digest before installation; unpack only expected paths.
3. Install into an owned per-user location without requiring administrator access. Explain PATH changes and make repeated installation idempotent.
4. Preserve a prior working version until replacement is verified. Handle Windows running-executable replacement deliberately.
5. Remove only launcher-owned files on uninstall; retain user credentials/preferences unless explicit removal is requested. Restoring client configuration is a separate, conflict-aware operation.

GitHub Releases attach binary assets to versioned releases. [GitHub Releases](https://docs.github.com/en/repositories/releasing-projects-on-github/about-releases). Publish checksums and source/build identity alongside archives. Checksums detect mismatches; stronger publisher/build authentication should use signatures or provenance. GitHub artifact attestations connect artifacts to workflows and can be verified, but merely generating an attestation does not make an installer verify it. [GitHub attestations](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations).

For public distribution, plan macOS Developer ID signing and notarization and test the downloaded artifact's real Gatekeeper behavior. [Apple Developer ID](https://developer.apple.com/developer-id/). Windows SignTool supports signing, timestamping, and signature verification; signing does not by itself prove the application is correct. [Microsoft SignTool](https://learn.microsoft.com/en-us/windows/win32/seccrypto/signtool). Signing identities and release authorization are operational prerequisites, not language features.

## Can the maintainer meaningfully review Go?

**Yes, partially reviewing straightforward Go is a realistic path for an experienced C#/Java/Node/Python developer.** That is a judgment about the learning task, not a guarantee of proficiency after a fixed number of hours. Initially the maintainer can evaluate product behavior, data flow, credential destinations, CLI contracts, and destructive operations while asking for help with Go-specific semantics.

Proposed review preparation:

1. Read the Tour sections on functions, structs, slices/maps, interfaces, errors, pointers, and `defer`. Relate these to a small function from this launcher. [A Tour of Go](https://go.dev/tour/welcome/1).
2. Read the relevant Effective Go sections on errors, interfaces, and resource cleanup. Review value versus pointer behavior and how interface values can contain typed nil pointers. Do not begin with advanced generics or concurrency abstractions. [Effective Go](https://go.dev/doc/effective_go).
3. Walk through one concrete path: parse arguments → locate agent → construct child-only environment → start agent → return its exit code → clean up owned temporary state.
4. Require each change to explain its observable behavior and show relevant test evidence. Keep small modules, explicit errors, few dependencies, and concurrency only where needed. These are proposed project conventions.

Review questions tied to this product:

- Is the credential sent only to the selected endpoint and intended child process? Can logs, command arguments, or persisted configuration reveal it?
- Are arguments passed as data, including paths with spaces and Windows shims?
- Do configuration writes preserve unknown fields and concurrent user edits? Does restoration avoid overwriting newer changes?
- Are failed downloads/authentication/model discovery understandable, and do they leave existing installation/configuration intact?
- Does cancellation clean up only owned processes/files, preserve the terminal, and propagate the agent's exit status?
- Are errors handled at every filesystem/process/network boundary? Does `defer` actually execute on the chosen return/exit path?

Go's race detector can find races exercised by a run; it does not prove unexecuted paths race-free. [Race detector](https://go.dev/doc/articles/race_detector). `govulncheck` checks known vulnerable dependencies and relevant call paths; it supplements source review. [Go vulnerability tutorial](https://go.dev/doc/tutorial/govulncheck).

## Evidence required before “it works”

For the first prototype: one explicitly named client/version, a direct endpoint/model, visible routing confirmation, interactive launch, cancellation, exit status, and a subsequent ordinary agent launch with original configuration intact. Report exact tested platforms; do not generalize from one Mac.

Before a public cross-platform release: clean-machine execution without development runtimes; native terminal tests per supported OS/CPU; minimal Linux dependency checks; installation/update/uninstall tests; configuration-conflict and interrupted-write recovery; checksum/signature verification; explicit supported client versions; and credential-safe diagnostics.

These are proposed acceptance criteria, not completed tests. No build, installer, signing, or cross-platform runtime validation was performed during this research. The language decision remains with the maintainer after reviewing the Go/C# tradeoff.
