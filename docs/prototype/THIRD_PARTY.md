# Dependencies and source references

No Ollama source code was copied into this prototype. The researched launcher
pattern informs the command flow; Ollama's MIT license is documented separately
in the [launcher research](https://github.com/kreuzhofer/nebius-tofa-cli/issues/2).
This document does not select a license for the project's own code.

Direct runtime dependencies are pinned in `go.mod` and checksummed in `go.sum`:

- [zalando/go-keyring v0.2.6](https://github.com/zalando/go-keyring/tree/v0.2.6): MIT,
  with Apache-2.0 notices on some source files. macOS uses Apple's `security -i`
  with the key on stdin; Windows uses Credential Manager; Linux uses Secret Service.
- [golang.org/x/term](https://pkg.go.dev/golang.org/x/term) and
  [golang.org/x/sys](https://pkg.go.dev/golang.org/x/sys): BSD-3-Clause.
- [yaml.v3](https://github.com/go-yaml/yaml/tree/v3.0.1): MIT and Apache-2.0.

Transitive runtime modules include `al.essio.dev/pkg/shellescape` (MIT),
`github.com/danieljoos/wincred` (MIT), and `github.com/godbus/dbus/v5` (BSD-2-Clause).
Before distribution, ship the full dependency license notices alongside artifacts;
`go.sum` is an integrity file, not a license notice bundle.

Configuration references:

- [Codex configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference)
- [Codex advanced configuration](https://learn.chatgpt.com/docs/config-file/config-advanced)
- [Token Factory model listing](https://docs.tokenfactory.nebius.com/api-reference/models/list-models)
- [Token Factory Responses](https://docs.tokenfactory.nebius.com/api-reference/inference/create-a-response)

These establish configuration/API contracts, not live compatibility. Local Codex
0.154.0 help was inspected; subsequent synthetic integration tests use 0.155.1.
No assistant-initiated remote inference was run.

## Vendored Codex instructions

`internal/tofa/assets/codex-prompt.md` is an unmodified copy of
[`codex-rs/models-manager/prompt.md`](https://github.com/openai/codex/blob/be2951ea34f0d295ed0becf97079f92fa5f6950e/codex-rs/models-manager/prompt.md)
from Codex `rust-v0.155.1`, commit `be2951ea34f0d295ed0becf97079f92fa5f6950e`.
SHA-256: `ac8ae107a0d72fe3476b430afb161ea4e67da2e446d778aefc44828160559807`.
It preserves the existing fallback model's coding instructions when a scoped Kimi
catalog replaces the fallback descriptor.

The upstream [Apache-2.0 license](../../internal/tofa/assets/codex-LICENSE) and
[NOTICE](../../internal/tofa/assets/codex-NOTICE) are retained verbatim alongside
the prompt. `scripts/build.sh` includes copies as `LICENSE-CODEX.txt` and
`NOTICE-CODEX.txt` in the artifact directory. Only the prompt is reused here;
the full upstream notice is preserved, not a claim that this launcher embeds
Codex's UI or its other components. Full notices for the Go dependency graph
remain a separate prerequisite for published installable releases.
