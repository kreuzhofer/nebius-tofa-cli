# Local prerelease qualification

The macOS runner qualifies the downloaded candidate in your **normal account**.
It installs the selected release, prompts for real login, checks Codex with
`moonshotai/Kimi-K3`, uninstalls preserving login, reinstalls and proves saved-login
reuse, then asks separately before purging local tofa state. It never uploads reports.

This is a maintainer qualification tool, not part of the published launcher bundle.
Use the runner from this repository checkout; candidate bytes and matching
installation scripts always come from the selected release. The runner does not
rebuild or replace published assets. Changed release files require a new candidate.

## Before running

Use macOS ARM64 with Python 3.9+, `curl`, the GitHub CLI (`gh`) and an installed
Codex CLI on PATH. Sign `gh` in using your existing authorized GitHub account;
this private repository's anonymous download URLs return 404. The runner uses
`gh` for both release metadata and downloads and never handles access tokens.
Normal launcher prerequisites, Keychain access and Token Factory connectivity
still apply. No extra OS account or self-hosted GitHub runner is required.

Close other launcher/Codex sessions to avoid concurrent changes. Before allowing
replacement or purge, arrange any backup, credential recovery or token reissue
**yourself**. The runner detects existing state but does not copy credentials.
Purging removes local saved credentials; it does not revoke Token Factory tokens.

From a checkout of the repository containing this runner:

```sh
python3 scripts/qualify_macos.py \
  --version v0.1.0-rc.1 \
  --output "$HOME/tofa-macos-rc1-$(date +%Y%m%d-%H%M%S).json"
```

If you need a checkout first, use `gh repo clone kreuzhofer/nebius-tofa-cli` and
enter the resulting directory. Keep the runner checkout/commit recorded with your
attached report. The tag selects the candidate automatically; no commit copying
is needed. The runner resolves its commit via GitHub metadata, verifies all
selected binary/script checksums, and installs from that verified local download.

The human steps are deliberately explicit:

1. Type `READY` after preparing credential recovery and reviewing detected state.
2. Enter your real API key and project in the installed launcher's interactive
   login. Respond to any OS access prompts. Credentials are not runner arguments.
3. Open a **new terminal**, check `command -v tofa` and `tofa --version` against
   the path/version printed by the runner, then type `FOUND` in the runner.
4. After both live checks, type `PURGE` to remove local tofa state. Declining leaves
   state available for recovery and produces an incomplete report.

`--storage file` or `--storage keyring` explicitly selects a backend when needed.
The default leaves vault-first selection and existing saved backend choice to the
launcher. File storage is unencrypted; the launcher announces its location.
`--codex /absolute/path/to/codex` selects a client. Each command/live turn defaults
to 180 seconds (`--timeout`, maximum 600); each human/login step defaults to 900
seconds (`--human-timeout`, maximum 3600). OS prompts and signing blocks should be
recorded separately in the validation issue.

## Evidence and recovery

The command reserves a new JSON file and matching `.md` summary, both mode 0600,
before changing account state, and fills them with evidence when the run ends.
It rejects unwritable destinations and refuses to overwrite earlier evidence. Exit zero requires every lifecycle
stage, preservation assertion and final cleanup assertion to pass. Attach both
files to [the real-machine validation issue](https://github.com/kreuzhofer/nebius-tofa-cli/issues/23)
yourself. A failed or interrupted run is evidence, too; retain it alongside reruns.

The live assertions reuse the [Codex compatibility harness](../prototype/LIVE-COMPATIBILITY.md):
incremental text deltas, successful shell-tool execution, independently checked
JSON results, and continuation of the exact conversation. Each check uses scratch
Codex configuration/authentication and a private workspace. The launcher retains
its normal account's saved credential access. Successful inference after reinstall,
without a second login, establishes saved-login reuse.

Reports contain only fixed stage names, counts/status/timing, public versions,
release identity and binary/script hashes. They exclude keys, credential-file
contents/hashes, private paths, project/session/credential identifiers and raw
conversation bodies. Normal Codex config/auth and unrelated files under tofa's
installation/config directories are compared in memory, including the contents of
symlinked files; no such hashes are
exported. Shell settings are compared after removing owned PATH blocks and trailing
newlines added by the installer. Only tofa's recorded Keychain references are
queried, without requesting secret values; unrelated vault entries are untouched.

On failure or Ctrl-C/SIGTERM, children are stopped and scratch directories are
removed; remaining installation/account state is reported rather than automatically
purged. Check the `cleanup` fields: `false` means state remains, `null` means a
check could not finish. Use the pinned standalone uninstaller from the release or
the remaining installed launcher's `uninstall` command to recover, after preparing
credentials. Unexplained failures cannot count as passes. A forced kill, power
loss or filesystem failure can prevent final reports/cleanup; inspect remaining
private temporary directories before rerunning. Reports stay local even on failure.

## Shared report v1

Windows qualification uses the same top-level fields and stage semantics. Platform
implementations may add evidence fields without changing these names:

| Field | Meaning |
| --- | --- |
| `schema_version` | `1` |
| `candidate.tag`, `.commit`, `.binary_sha256` | Requested prerelease, resolved commit, verified installed binary hash; unavailable values may be omitted on early failure |
| `candidate.scripts_sha256` | Verified matching installation/recovery-script hashes |
| `host.os`, `.arch`, `.release` | OS, native architecture and OS version |
| `client.version` | Validated Codex CLI version; omitted before successful preflight |
| `backend` | `keyring`, `file`, or `unknown` before successful login |
| `model` | `moonshotai/Kimi-K3` |
| `started_utc` | UTC timestamp |
| `existing_state` | Booleans for `installation`, `configuration`, `file_credentials`, `vault_references` |
| `stages` | Ordered `{name, status}` entries; statuses `pending`, `running`, `passed`, `failed`, `incomplete` |
| `preservation` | Named boolean checks; `null` means not established |
| `cleanup` | Named boolean checks; `null` means not established |
| `outcome` | `passed`, `failed`, or `incomplete` |
| `reason` | Fixed sanitized failure code, when applicable |
| `live`, `saved_login_reuse` | Sanitized live-harness assertions, when executed |

Shared stages: `preflight`, `recovery`, `download`, `install`, `login`,
`fresh_terminal`, `live`, `uninstall`, `reinstall`, `saved_login_reuse`, `purge`.
A `passed` human stage records the maintainer's explicit confirmation; synthetic
CI answers never establish real-machine acceptance. Full macOS and Windows reports
against the same final candidate remain required by #23.

## Offline development checks

```sh
python3 scripts/qualify_macos_test.py -v
python3 scripts/live_compat_test.py -v
```

The runner tests invoke its public CLI with temporary account directories, a
controlled `gh` executable, matching shell installers, a synthetic downloaded
launcher, synthetic Codex, and a synthetic Keychain command. They never access the
real account's saved credentials, native vault, GitHub or Token Factory. macOS and
Linux CI exercise offline orchestration; synthetic Keychain checks run on macOS.
Linux file-backend portability is not evidence of real Linux qualification.
