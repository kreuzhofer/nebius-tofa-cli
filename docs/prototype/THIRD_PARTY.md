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
0.154.0 help was inspected; inference was not run.
