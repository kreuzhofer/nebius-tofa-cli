"""THROWAWAY interactive #34 trial: durable bridge plus inactive provider metadata.
Run from the repository root. Each window stays open until Enter in this terminal.
All inference replies are synthetic. Ctrl-C closes only this trial's resources.
"""
import contextlib
import json
from pathlib import Path
import subprocess
import tempfile

import durable_route_probe as durable
import run as prototype


def main():
    root = Path(tempfile.mkdtemp(prefix="PROTOTYPE-live34-", dir="/private/tmp"))
    for name in ("codex", "workspace", "electron", "routing", "shell", "bridge"):
        (root / name).mkdir(mode=0o700)
    (root / "workspace/PROTOTYPE-WIPE-ME.txt").write_text("Synthetic live trial. No real credentials.\n")
    version = subprocess.check_output([str(prototype.ENGINE), "--version"], env=prototype.scratch_env(root), text=True).strip()
    prototype.check(version == "codex-cli 0.155.0-alpha.16.3", "requalify for the installed engine")
    native_catalog, merged = prototype.catalog(root)
    records, routes, threads = [], [], {}
    native = prototype.Fixture("native", {row["slug"] for row in native_catalog["models"]}, records)
    (root / "codex/auth.json").write_text(json.dumps({"OPENAI_API_KEY": native.token}))
    (root / "codex/auth.json").chmod(0o600)
    (root / "codex/config.toml").write_text(
        'model="' + prototype.NATIVE + '"\nmodel_provider="openai"\nopenai_base_url=' + json.dumps(native.endpoint) +
        '\ncli_auth_credentials_store="file"\napproval_policy="never"\nsandbox_mode="read-only"\nweb_search="disabled"\n'
        '[features]\nshell_snapshot=false\nanalytics=false\n'
        '[model_providers.nebius-tofa]\nname="Token Factory (launcher required)"\nbase_url="http://127.0.0.1:0"\n'
        'env_key="TOFA_OFFLINE_PROBE_MISSING_KEY"\nwire_api="responses"\nrequires_openai_auth=false\n'
        'supports_websockets=false\nrequest_max_retries=0\nstream_max_retries=0\n')
    auth = prototype.snapshot(root)["auth.json"]
    bridge = durable.durable_bridge(root)
    report = {"issue": 34, "throwaway": True, "engine_version": version, "real_user_profile_used": False,
              "inactive_provider_metadata": True, "durable_bridge": True, "phases": [], "requests": records}
    report_path = root / "live-history-evidence.json"
    prototype.emit("scratch_profile", path=str(root))
    try:
        # Seed successful history from both routes, then close the tofa seed route.
        for mode in ("ordinary", "tofa"):
            with durable.launch(root, bridge, mode, merged, records, routes) as (invoke, env, manifest):
                with contextlib.closing(prototype.Engine(str(invoke), root, [])) as engine:
                    name = "PROTOTYPE ordinary history" if mode == "ordinary" else "PROTOTYPE tofa history"
                    thread = engine.call("thread/start", {"cwd": str(root / "workspace"), "historyMode": "legacy", "threadSource": "user"})["thread"]
                    engine.turn(thread["id"], name)
                    engine.call("thread/name/set", {"threadId": thread["id"], "name": name})
                    threads[thread["id"]] = name
        adapted = next(key for key, name in threads.items() if name == "PROTOTYPE tofa history")
        for phase, mode in enumerate(("ordinary", "tofa", "ordinary", "tofa"), 1):
            with durable.launch(root, bridge, mode, merged, records, routes) as (invoke, env, manifest):
                with contextlib.closing(prototype.Engine(str(invoke), root, [])) as engine:
                    resumed = engine.call("thread/resume", {"threadId": adapted})
                    prototype.check(resumed["modelProvider"] == "nebius-tofa", "stored provider changed")
                    state = prototype.read_state(engine, threads)
                report["phases"].append({"phase": phase, "mode": mode, "sessions_before_ui": state, "ui_verified": False})
                report_path.write_text(json.dumps(report, indent=2) + "\n")
                prototype.emit("human_checkpoint", phase=phase, mode=mode,
                               instruction="Ordinary: open tofa history; sending should report missing launcher credential. Tofa: continuing should return Token Factory reply.")
                prototype.desktop(root, bridge, mode, phase, False, env)
                prototype.check(prototype.snapshot(root)["auth.json"] == auth, "synthetic auth changed")
    finally:
        native.close()
        report_path.write_text(json.dumps(report, indent=2) + "\n")
        prototype.emit("cleanup_complete", report=str(report_path), scratch_retained=str(root))


if __name__ == "__main__":
    main()
