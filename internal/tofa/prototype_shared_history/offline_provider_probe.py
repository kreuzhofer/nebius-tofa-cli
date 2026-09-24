"""THROWAWAY: ordinary history hydration with inactive provider metadata.
No desktop, credentials, listening socket or inference service is used.
"""
import contextlib
import json
from pathlib import Path
import subprocess
import tempfile

import run as prototype


def main():
    root = Path(tempfile.mkdtemp(prefix="PROTOTYPE-offline34-", dir="/private/tmp"))
    for name in ("codex", "workspace"):
        (root / name).mkdir(mode=0o700)
    version = subprocess.check_output([str(prototype.ENGINE), "--version"], env=prototype.scratch_env(root), text=True).strip()
    prototype.check(version == "codex-cli 0.155.0-alpha.16.3", "requalify this probe for the installed engine")
    provider = 'model_providers.nebius-tofa={name="Token Factory (launcher required)",base_url="http://127.0.0.1:0",env_key="TOFA_OFFLINE_PROBE_MISSING_KEY",wire_api="responses",requires_openai_auth=false,request_max_retries=0,stream_max_retries=0}'
    config = root / "codex/config.toml"
    config.write_text('model="gpt-6-astra"\nmodel_provider="openai"\ncli_auth_credentials_store="file"\n')
    with contextlib.closing(prototype.Engine(str(prototype.ENGINE), root, [provider, 'model_provider="nebius-tofa"', 'model="moonshotai/Kimi-K3"'])) as engine:
        thread = engine.call("thread/start", {"cwd": str(root / "workspace"), "historyMode": "legacy", "threadSource": "user"})["thread"]
        try:
            engine.turn(thread["id"], "Synthetic offline probe: no inference credential is available.")
        except RuntimeError as error:
            prototype.check("TOFA_OFFLINE_PROBE_MISSING_KEY" in str(error), "unexpected offline error")
        else:
            raise RuntimeError("offline turn unexpectedly succeeded")
    with contextlib.closing(prototype.Engine(str(prototype.ENGINE), root, [])) as engine:
        missing = engine.response("thread/resume", {"threadId": thread["id"]})
        prototype.check("Model provider `nebius-tofa` not found" in missing.get("error", {}).get("message", ""),
                        "baseline did not reproduce absent-provider resume error")
    # Add only inactive metadata to this synthetic profile. Ordinary startup has
    # no launch overrides, no real endpoint and no credentials.
    with config.open("a") as out:
        out.write('\n[model_providers.nebius-tofa]\nname="Token Factory (launcher required)"\nbase_url="http://127.0.0.1:0"\nenv_key="TOFA_OFFLINE_PROBE_MISSING_KEY"\nwire_api="responses"\nrequires_openai_auth=false\nrequest_max_retries=0\nstream_max_retries=0\n')
    with contextlib.closing(prototype.Engine(str(prototype.ENGINE), root, [])) as engine:
        result = engine.call("thread/resume", {"threadId": thread["id"]})
        prototype.check(result["modelProvider"] == "nebius-tofa", "stored provider changed")
        history = engine.call("thread/read", {"threadId": thread["id"], "includeTurns": True})["thread"]
        prototype.check(len(history["turns"]) >= 1, "stored history was lost")
        try:
            engine.turn(thread["id"], "Continuation must still fail explicitly without a credential.")
        except RuntimeError as error:
            prototype.check("TOFA_OFFLINE_PROBE_MISSING_KEY" in str(error), "unexpected offline error")
        else:
            raise RuntimeError("ordinary mode unexpectedly performed inference")
    report = {"issue": 34, "throwaway": True, "engine_version": version, "result": "PASS at engine boundary only",
              "baseline_without_provider": "missing-provider resume error",
              "ordinary_resume_with_inactive_provider": "success; original provider preserved",
              "stored_history_readable_via_rpc": True, "inference_without_credential": "explicit missing-credential error",
              "real_user_profile_used": False, "live_adapter": False, "credential_supplied": False,
              "desktop_ui_verified": False,
              "limitations": ["synthetic failed-turn history; prior separate probe also resumed the successful human-trial history", "inactive metadata is persistent in scratch config", "desktop rendering and interaction with live launch overrides remain unverified"]}
    (prototype.HERE / "offline-provider-probe-evidence.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report))


if __name__ == "__main__":
    main()
