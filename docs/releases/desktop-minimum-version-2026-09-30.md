# Desktop minimum-version adaptation, 2026-09-30

Issue #35 now admits compatible newer desktop releases instead of requiring one
exact app/build/engine version. The minimums are macOS 26.6.2 arm64, ChatGPT desktop
26.917.71314 (`com.openai.codex`) and engine 0.155.0-alpha.16.4. Exact build numbers
and hashes remain qualification evidence. This checkpoint does not publish a
prerelease or establish the remaining visible desktop/live-provider checks.

The implementation supports the native packaged macOS engine, validates catalog
cache versions against the selected engine, and repairs ordinary saved references
when an app update moves the engine within the same bundle. Installer upgrades
validate both recorded layouts. Incomplete packages, below-minimum versions and
incompatible ownership/configuration still fail explicitly.

The [primary-source investigation](../research/desktop-minimum-version-2026-09-30.md)
identifies ChatGPT 26.928.21956 (12404), native engine 0.159.2 and its exact hashes.
The adjacent `codex-cli/bin/codex` is not the native macOS engine; tests select
`codex-cli/CodexCLI.app/Contents/MacOS/codex`.

## Validation checkpoint

On macOS 26.6.2 arm64 with Go 1.27.1:

- Minimum-version, packaged-layout, incomplete-package and saved-reference tests
  reproduced failures before implementation and passed after their fixes.
- Actual installer/uninstaller regression passed after an engine relocation.
- The new engine's authenticated catalog suite passed. The minimum engine's
  authenticated catalog suite also passed, including same-version foreign-cache
  ambiguity and refresh behavior.
- The initial complete race run recorded 447 passing test/subtest entries, three
  opt-in skips, and one failing leaf test (plus its enclosing test). Its assertion
  expected four Guardian HTTP 503 attempts; engine 0.159.2 made twelve. Execution
  remained denied. Retry count is engine-owned, so the corrected test requires
  bounded failure and no execution. Its targeted race rerun passed alongside the
  corrected catalog regressions. This initial run is retained as a failure.
- Go vet, all six platform builds, six packaging tests, four native distribution
  lifecycle tests and two terminal tests passed. Cross-builds are not native
  Linux/Windows qualification.
- An initial generic Python test-discovery invocation did not provide the
  lifecycle/terminal scripts' required arguments. Their proper entrypoints passed
  on rerun. The packaging check also caught the newly added x/mod dependency's
  missing notice in an earlier source snapshot; the notice was added and all six
  packaging checks passed on the corrected snapshot.
- Standards review found a stale foreign-cache fixture version; the fixture now
  matches the selected engine and asserts the ambiguity refusal. Final Standards
  and Spec reviews have zero remaining findings.

The installed-Electron startup attempt refused an already-running ordinary app,
without changing it. Final full-suite confirmation, installed-Electron ownership,
real-account UI and the immutable candidate lifecycle remain qualification work.
Earlier rc6/rc7 evidence and bytes are retained; no existing candidate is retagged.
