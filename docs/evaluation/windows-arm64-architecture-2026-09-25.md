# Native Windows architecture detection — 2026-09-25

This follow-up fixes the installer architecture mismatch recorded in
[the Windows CLI-selection validation](windows-arm64-cli-selection-2026-09-25.md).
The source candidate is `31fccb3` plus the changes in the commit containing this
report. The local binary bundle uses version `v0.0.0-test.arm64-fix`.

The PowerShell installer and Python qualification runner now query
`IsWow64Process2` for the native host machine. Neither infers the host from an
emulated process's environment. Unknown machine types and failed API calls fail
explicitly. The installer remains self-contained and supports repeated invocation
in one PowerShell process. Windows 10 version 1709 or later is required by the
[Windows API](https://learn.microsoft.com/en-us/windows/win32/api/wow64apiset/nf-wow64apiset-iswow64process2).

## Regression and validation

Before the fix, the Python regression returned `amd64` for an ARM64 host with
`PROCESSOR_ARCHITECTURE=AMD64` and no `PROCESSOR_ARCHITEW6432`. The real installer
regression failed under both Windows PowerShell 5.1 and PowerShell 7.6.6 with an
unexpected AMD64 download request. Its expected artifact uses an independent CIM
observation of the host CPU, rather than repeating the installer implementation.

Native runs used SabreTest in a disposable UTM snapshot of Windows 11 Pro ARM64
(build 26200), Python 3.13 ARM64, and verified portable PowerShell 7.6.6 / Node
22.23.3. Script execution policy was set only for the test processes. Each suite
checked that persistent user PATH was restored. No live service or ordinary
account credentials were used.

All six native runs passed:

| Check | Result |
| --- | --- |
| Qualification through Windows PowerShell 5.1 | 17 tests passed |
| Qualification through PowerShell 7.6.6 | 17 tests passed |
| Windows process/discovery checks | 10 tests passed |
| Offline installer lifecycle, PowerShell 5.1 and 7.6.6 | Both passed |
| Real ARM64 candidate lifecycle, PowerShell 7.6.6 | Passed |

Persistent user PATH was preserved in every run. The
[machine-readable evidence](evidence/windows-arm64-architecture-2026-09-25.json)
records run times, script/candidate hashes, and retained log hashes. Host logs
are under `/tmp/tofa-arm64-fix`.

The complete Go race suite and `go vet ./...` passed on macOS. The offline
Python checks passed 111 tests, with 41 platform/optional-client skips. Six
platform artifacts built successfully. Standards and specification reviews
reported zero findings.

The candidate installer and manifest were rebuilt locally; no published release
was replaced. These are synthetic installer/runner checks, not live model or
normal-account release qualification. Native AMD64 execution remains a CI check;
the local regression covers its architecture mapping at the Windows API boundary.
