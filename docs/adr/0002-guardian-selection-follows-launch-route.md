# Guardian selection follows the launch route

Adapted Codex launches default to `zai-org/GLM-5.3-Flash` as Guardian, with an
optional explicit `--guardian-model` override and no Guardian picker: the prior
CLI campaign supports this starting choice, while each desktop combination still
requires qualification under [#54](https://github.com/kreuzhofer/nebius-tofa-cli/issues/54).
The diagnostic CLI `--direct` route retains native reviewer selection, announces
that exception and rejects `--guardian-model`, preserving its existing contract
instead of claiming unverified direct Guardian support. Both routes retain native
approval-policy enforcement, and an unavailable configured Guardian is an error,
not a reason to substitute a different model or route.
