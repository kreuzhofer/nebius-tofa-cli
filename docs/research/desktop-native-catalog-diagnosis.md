# Native catalog freshness refusal — 2026-09-24

Follow-up to the [incomplete #50 qualification](../releases/desktop-shared-history-2026-09-24.md).
The ordinary profile has a static `model_catalog_json` override. That override
bypasses native account discovery, leaving the launcher without the freshness
evidence its existing contract requires. The refusal is intentional; its former
instruction to refresh native models did not explain this configuration obstacle.

## Reproduction and discrimination

The diagnostic used the production `App.Run` boundary and installed engine
`codex-cli 0.155.0-alpha.16.4`, SHA-256
`93169e745735930598e867ad837abf3fdc50774a3ad7e7aa89c0d0c51b0189a5`,
on macOS 26.6.2 ARM64. All test credentials, account IDs, catalog responses and
profiles were synthetic. The native HTTP fixture could serve a valid catalog;
external inference and ordinary-profile launches were unnecessary.

The original temporary diagnostic failed twice in about one second per attempt
with `native account catalog freshness could not be established`. Removing the
synthetic old-version cache still reproduced the failure: the old cache was not
a necessary cause. The remaining static-catalog setting was then varied alone.

| Hypothesis | Discriminating observation | Result |
| --- | --- | --- |
| Static catalog bypasses native discovery | Static setting present: zero native HTTP requests and refusal. Setting absent: one request and successful launcher initialization. | Confirmed |
| Production assumes an incompatible engine cache format | The installed engine's fresh account catalog passes the unchanged guard after the static setting is removed. | Not the cause of this refusal |
| Wrong profile or an injected configuration layer supplies the setting | Public `config/read` with layers locates the setting in the ordinary user configuration, using the production login-shell home resolution. | The setting belongs to the ordinary profile |

The authorized, read-only configuration query found the ordinary engine home
`~/.codex`, with the override in `~/.codex/config.toml` pointing to
`~/.codex/ollama-launch-models.json`. The effective provider was the native default.
The static file contained 13 descriptors. Its filename is not proof of which
process wrote the setting. No credential contents or raw account configuration
were retained in this report, and no ordinary configuration was changed.

## Correction and regression

The launcher now advises checking ordinary `model_catalog_json` overrides and
explains that static catalogs bypass discovery. This is a possible cause in the
general freshness error, not an assertion that every cache failure has this cause.
No authentication, freshness, compatibility or provider-routing gate changed.

The permanent regression extends `TestDesktopUsesAuthenticatedNativeCatalog` at
the existing launcher/executable/HTTP boundary. It first reproduced the missing
setting guidance, then passed with the corrected message. It checks that refusal
preserves the synthetic static setting and prevents desktop initialization; an
explicit removal of only that setting then permits native discovery and launch.
Existing descriptor-preservation and cache/account checks also run after recovery.

```sh
TOFA_TEST_DESKTOP_ENGINE=/Applications/ChatGPT.app/Contents/Resources/codex \
  go test ./internal/tofa -run '^TestDesktopUsesAuthenticatedNativeCatalog$' -count=1 -v
```

That test passed all five scenarios, including fresh, expired, different-account
and different-version cache cases. `go vet ./...` passed. The full
`go test -race ./... -count=1` suite passed with both `TOFA_TEST_DESKTOP_ENGINE`
and `TOFA_TEST_DESKTOP_APP` enabled (`internal/tofa`: 244.218 s). The two separate
installed CLI opt-ins, `TestInstalledCodexToolAndContinuationThroughAdapter` and
`TestInstalledCodexApprovalReviewThroughAdapter`, were skipped because
`TOFA_TEST_CODEX` was unset. No desktop opt-in was skipped. The temporary diagnostic
and debug logging were removed.

## Remaining ordinary-profile action

Restoring native discovery requires an explicit decision about the root-level
static `model_catalog_json` setting. If that customization is no longer wanted
for ordinary launches, remove or comment only that setting, keeping the catalog
file, credentials and other configuration intact. This changes ordinary model
discovery, so the launcher does not do it automatically. If the static catalog
is still required, its support needs separate qualification; the existing CLI
target remains available.

Deleting `models_cache.json` or repeatedly refreshing while retaining the static
override does not address the demonstrated cause. The ordinary-profile override
was left intact during this diagnosis. The same static setup therefore still
refuses launch, now with relevant recovery guidance. Removing it succeeded in
the synthetic fixture; real-account refresh and the full UI/live-inference
walkthrough still need to be performed before completing #50.

## Authorized ordinary-profile recovery — 13:22 UTC

After the diagnosis, the maintainer explicitly authorized commenting out only the
root-level `model_catalog_json` setting and retrying the live preflight. That one
line in the ordinary configuration was commented out; all other bytes and file
permissions were preserved. The catalog file was retained. No credential was
copied, and no login/logout was performed.

The installed alpha.16.4 engine's ordinary-account preflight then **passed**.
Its export contained nine native descriptors matching a fresh cache with an
account identity and `client_version: 0.155.0`. The export differed from bundled
metadata; both exports exited successfully without diagnostics. The saved Token
Factory login also continued to list Kimi-K3 successfully through the unchanged
local qualification candidate.

[Sanitized recovery evidence](evidence/desktop-native-catalog-recovery-2026-09-24.json)
records the checks separately from the earlier incomplete attempt. This resolves
the observed static-catalog prerequisite for the current account. It is an engine
and catalog preflight, not a production desktop launch, UI observation, or inference
test. #50's remaining live acceptance checks are still required.
