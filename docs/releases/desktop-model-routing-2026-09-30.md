# Desktop model routing: implementation checks for #55

The launcher main is now the initial/default selection. The desktop catalog and
ordinary request route share one snapshot of every eligible available Token
Factory main. Experimental choices are enabled and labelled. Each conversation
retains its own selected main, and the configured launch Guardian reviews all
Token Factory conversations under native approval policy.

## Test boundary

Checks use macOS 26.6.2 arm64, Go 1.27.1 and the retained bundled engine
`codex-cli 0.155.0-alpha.16.4` from the qualified ChatGPT 26.917.71314 (10954)
combination. The engine SHA-256 is
`93169e745735930598e867ad837abf3fdc50774a3ad7e7aa89c0d0c51b0189a5`.
The launcher runs with disposable homes, synthetic credentials and local HTTP
fixtures. These checks do not establish live model compatibility or Electron UI
qualification, and do not promote experimental combinations to supported.

## Regressions demonstrated through the bundled engine

- Catalog membership is independent of the launch default. Every eligible main
  has its own metadata and the configured Guardian; native descriptors survive.
- A DeepSeek conversation switches to GLM Flash, completes a tool exchange and
  another turn, and resumes under a different launcher default. Its identity,
  ordered history, title, workspace and provider survive. A second conversation
  keeps its independent selection.
- A selection change during an active turn applies to the next turn. Cancellation
  interrupts the upstream request. An unavailable selection fails explicitly;
  selecting an eligible model recovers within the same conversation.
- Automatic reviews still enforce the configured Guardian and native allow,
  deny, invalid-result, provider-failure, cancellation and deadline behavior.
- Switching an image-bearing conversation to a text-only model retains its saved
  images. The engine omits them from inference with an explicit marker; the
  launcher prints a recovery notice. Switching back includes the images again.
- A forced low auto-compaction threshold exercises context pressure. Compaction
  can arrive through the regular Responses endpoint; the adapter now rejects
  that request before inference, with recovery guidance. Existing history stays
  intact. Live provider context limits remain unqualified.

## Desktop defaults and concurrent CLI sessions

The user's initially reported CLI-model change was intermittent: a later open,
greeting and close left Astra unchanged. An isolated reproduction established a
narrower cause: the desktop model-default `config/batchWrite` contract writes the
shared user configuration and changes concurrent CLI defaults after desktop exit.
This does not prove that write caused the original observation.

The desktop bridge now answers those writes with a session-only `okOverridden`
result after asking the engine to verify the current configuration version.
The pinned renderer's handling of that result retains its choice locally and
passes it when starting a thread. Actual-engine tests verify unchanged CLI model,
reasoning effort and configuration bytes during and after desktop use, native
unrelated settings writes, and rejection of stale or mixed model/settings writes.
The test supplies the chosen model to `thread/start` as the traced renderer does;
the renderer interaction itself still requires a visible UI check.

## Packaging and remaining qualification

The final `go test -race -json -timeout 20m ./...` run passed with
`TOFA_TEST_DESKTOP_ENGINE` set to the retained engine: 434 passing test/subtest
events, zero failures and three skipped opt-in tests. Those skips were the two
separate installed Codex CLI inference/review checks and the installed Electron
shell-isolation check. `go vet ./...` and `git diff --check` also passed.

A separate local package, `v0.1.0-issue55.1`, builds for Darwin, Linux and Windows
on amd64 and arm64. All four native macOS lifecycle checks pass through the real
installer and uninstaller with disposable homes. They cover fresh-shell PATH
discovery, repeated install, uninstall, reinstall, purge and failure preservation.
Cross-compilation does not establish native Linux or Windows execution.

Before #55 is complete, verify the actual desktop picker, session-local default
selection, concurrent CLI isolation and conversation switching in Electron, then
perform bounded live qualification. No new build has been installed into the
user's ordinary environment during these checks. Existing automatic naming is
unchanged: only the captured Kimi launch-default contract has a title route;
DeepSeek launch defaults still report unsupported automatic titles explicitly.
