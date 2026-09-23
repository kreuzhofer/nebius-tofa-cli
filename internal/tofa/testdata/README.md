# Desktop title fixture

`desktop-title-request.json` is the complete JSON request captured on 2026-09-23
from ChatGPT 26.915.31945 (9922), bundled Codex 0.155.0-alpha.9.2, macOS 26.6.2
arm64, using an isolated Codex/Electron home and the launcher's Kimi catalog.
The human entered the synthetic calculator prompt visible in the fixture.
The authenticated local capture endpoint returned HTTP 503; it made no upstream
inference calls. `thread_source` and `turn_trigger` both identify `thread_title`.

Sanitization replaces the user's home and temporary state paths, UUIDs (including
message, installation, session, thread, turn, window and cache identifiers), and
the turn timestamp. No Authorization header or real provider credential is in
this fixture. Instructions, synthetic prompt, complete tool definitions, schema,
reasoning settings and request-field structure are retained. There are 11 top-level
tool entries: ten functions and one namespace containing three functions, for
13 callable tools. Tool parameter definitions are preserved unchanged by adaptation;
recognition checks names, types and namespace membership, not their descriptions.

This is an observed desktop contract, not a fabricated CLI title-only request.
The installed archive members were checked byte-for-byte against the inspected
source copies; their hashes and configuration investigation are documented in
[`desktop-title-generation.md`](../../../docs/research/desktop-title-generation.md).
Codex-derived instructions/tool text remain covered by the repository's existing
Codex license and third-party notice.
