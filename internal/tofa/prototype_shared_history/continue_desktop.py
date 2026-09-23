"""THROWAWAY human walkthrough after run.py --desktop stops on config mutation.

Reuse only its synthetic scratch profile; preserve the failing config result.
python3 internal/tofa/prototype_shared_history/continue_desktop.py /private/tmp/PROTOTYPE-tofa34-...
"""

import argparse
import contextlib
import json
from pathlib import Path
from urllib.parse import urlparse

import run as prototype


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("profile", type=Path)
    args = parser.parse_args()
    root = args.profile.resolve()
    prototype.check(root.parent == Path("/private/tmp") and root.name.startswith("PROTOTYPE-tofa34-"),
                    "only a marked disposable prototype profile is allowed")
    prototype.check((root / "workspace/PROTOTYPE-WIPE-ME.txt").is_file(), "missing prototype marker")
    config = (root / "codex/config.toml").read_text()
    endpoint = json.loads(next(line.split(" = ", 1)[1] for line in config.splitlines()
                               if line.startswith("openai_base_url = ")))
    address = urlparse(endpoint)
    prototype.check(address.scheme == "http" and address.hostname == "127.0.0.1", "expected synthetic loopback route")
    token = json.loads((root / "codex/auth.json").read_text())["OPENAI_API_KEY"]
    native_catalog, merged = prototype.catalog(root)
    records, routes = [], []
    # Rebind the previous synthetic endpoint and token. No auth/config rewriting.
    native = prototype.Fixture("native", {row["slug"] for row in native_catalog["models"]}, records,
                               token=token, port=address.port)
    baseline = prototype.snapshot(root)
    report = {"issue": 34, "throwaway": True, "real_user_profile_used": False,
              "known_shared_config_failure_unresolved": True,
              "ordinary_ui_observation": "User saw PROTOTYPE ordinary history, Astra selected, and a native synthetic continuation.",
              "phases": [], "requests": records}
    report_path = root / "human-walkthrough.json"
    threads = {}
    try:
        for phase, mode in ((2, "tofa"), (3, "ordinary"), (4, "tofa")):
            with prototype.route(root, mode, native_catalog, merged, records, routes) as wrapper:
                with contextlib.closing(prototype.Engine(str(wrapper), root, [])) as engine:
                    if phase == 2:
                        listed = engine.call("thread/list", {"modelProviders": [], "useStateDbOnly": True})["data"]
                        for row in listed:
                            detail = engine.call("thread/read", {"threadId": row["id"], "includeTurns": True})["thread"]
                            if detail.get("name") in ("PROTOTYPE ordinary history", "PROTOTYPE tofa history"):
                                threads[row["id"]] = detail["name"]
                        ordinary_ids = [key for key, name in threads.items() if name == "PROTOTYPE ordinary history"]
                        adapted_ids = [key for key, name in threads.items() if name == "PROTOTYPE tofa history"]
                        prototype.check(len(ordinary_ids) == 1 and len(adapted_ids) <= 1, "unexpected duplicate prototype conversations")
                        ordinary = ordinary_ids[0]
                        resumed = engine.call("thread/resume", {"threadId": ordinary})
                        prototype.check(resumed["modelProvider"] == "openai" and resumed["model"] == prototype.NATIVE,
                                        "ordinary session routing changed")
                        if adapted_ids:
                            adapted = adapted_ids[0]
                        else:
                            response = engine.call("thread/start", {"cwd": str(root / "workspace"), "historyMode": "legacy", "threadSource": "user"})
                            adapted = response["thread"]["id"]
                            engine.turn(adapted, "PROTOTYPE tofa history")
                            engine.call("thread/name/set", {"threadId": adapted, "name": "PROTOTYPE tofa history"})
                            threads[adapted] = "PROTOTYPE tofa history"
                    if phase == 3:
                        response = engine.response("thread/resume", {"threadId": adapted})
                        prototype.check("Model provider `nebius-tofa` not found" in response.get("error", {}).get("message", ""),
                                        "expected explicit unavailable provider")
                    if phase == 4:
                        resumed = engine.call("thread/resume", {"threadId": adapted})
                        prototype.check(resumed["modelProvider"] == "nebius-tofa" and resumed["model"] == prototype.MODEL,
                                        "tofa session routing changed")
                    state = prototype.read_state(engine, threads)
                report["phases"].append({"phase": phase, "mode": mode, "sessions": state, "ui_verified": False})
                report_path.write_text(json.dumps(report, indent=2) + "\n")
                prototype.emit("human_checkpoint", phase=phase, mode=mode, sessions=state, report=str(report_path))
                prototype.desktop(root, wrapper, mode, phase, False)
                prototype.check(prototype.snapshot(root)["auth.json"] == baseline["auth.json"], "synthetic auth changed")
                report["config_unchanged_since_continuation"] = prototype.snapshot(root)["config.toml"] == baseline["config.toml"]
    finally:
        native.close()
        report_path.write_text(json.dumps(report, indent=2) + "\n")


if __name__ == "__main__":
    main()
