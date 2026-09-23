"""THROWAWAY reproduction of the resume error behind the desktop logo loop.

Uses a new synthetic profile, the real bundled engine and desktop's app-server flag.
No desktop window or ordinary profile access.
"""
import argparse
import contextlib
import json
from pathlib import Path
import tempfile

import run as prototype


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--without-desktop-override", action="store_true")
    parser.add_argument("--override", default="plugins.codex-app-tools@openai-bundled.mcp_servers.codex_app.enabled=true")
    args = parser.parse_args()
    root = Path(tempfile.mkdtemp(prefix="PROTOTYPE-resume34-", dir="/private/tmp"))
    for name in ("codex", "workspace", "routing"):
        (root / name).mkdir(mode=0o700)
    native, merged = prototype.catalog(root)
    records, routes = [], []
    with prototype.route(root, "tofa", native, merged, records, routes) as wrapper:
        with contextlib.closing(prototype.Engine(str(wrapper), root, [])) as engine:
            thread = engine.call("thread/start", {"cwd": str(root / "workspace"), "historyMode": "legacy", "threadSource": "user"})["thread"]
            engine.turn(thread["id"], "Synthetic resume reproduction")
        overrides = [] if args.without_desktop_override else [args.override]
        with contextlib.closing(prototype.Engine(str(wrapper), root, overrides)) as engine:
            response = engine.response("thread/resume", {"threadId": thread["id"]})
            if "error" in response:
                print(json.dumps({"result": "FAIL", "error": response["error"], "desktop_override": bool(overrides)}), flush=True)
                raise SystemExit(1)
            prototype.check(response["result"]["modelProvider"] == "nebius-tofa", "provider changed")
            print(json.dumps({"result": "PASS", "provider": "nebius-tofa", "desktop_override": bool(overrides)}), flush=True)


if __name__ == "__main__":
    main()
