#!/usr/bin/env python3
from concurrent.futures import ThreadPoolExecutor
import json
import os
from pathlib import Path
import subprocess
import threading
import time
import urllib.request

HERE = Path(__file__).resolve().parent
SERVER = Path("/private/var/folders/f0/y3512xfn3vn334b7kcc6_hzw0000gn/T/opencode/voxt-verify-4525/runtime/bin/moss_transcribe_diarize_server")
MODEL = Path("/Users/shuohuang/.cache/huggingface/hub/models--OpenMOSS-Team--MOSS-Transcribe-Diarize/snapshots/704aa4a9c304e8520be88901e0d1960158ef5b15")
WAV = Path("/Users/shuohuang/voxt-dev-setup/3043/uiux/shots/verify-4389/synthetic-preview.wav")
PORT = 19254


def request(name):
    run = subprocess.run([
        "curl", "--fail-with-body", "--silent", "--show-error", "--max-time", "240",
        "-F", f"file=@{WAV};type=audio/wav", "-F", "model=moss_transcribe_diarize",
        "-F", "response_format=json", "-F", "max_new_tokens=96",
        "-F", "stop_at_end_of_text=true", "-F", "stop_on_token_loop=true",
        f"http://127.0.0.1:{PORT}/v1/audio/transcriptions",
    ], text=True, capture_output=True, timeout=250)
    (HERE / f"real-batch-{name}-response.json").write_text(run.stdout + run.stderr)
    return name, run.returncode, run.stdout, run.stderr


def main():
    log_path = HERE / "real-batch-server.log"
    env = os.environ.copy()
    env.update(HF_HUB_OFFLINE="1", TRANSFORMERS_OFFLINE="1")
    with log_path.open("w") as log:
        process = subprocess.Popen([
            str(SERVER), "--model-path", str(MODEL), "--host", "127.0.0.1",
            "--port", str(PORT), "--max-batch", "2",
        ], stdout=log, stderr=subprocess.STDOUT, env=env)
        try:
            deadline = time.monotonic() + 180
            while True:
                if process.poll() is not None:
                    raise AssertionError(f"server exited {process.returncode}: {log_path.read_text()[-3000:]}")
                try:
                    with urllib.request.urlopen(f"http://127.0.0.1:{PORT}/health", timeout=1) as response:
                        if json.load(response).get("status") == "healthy":
                            break
                except Exception:
                    pass
                if time.monotonic() >= deadline:
                    raise AssertionError("model load timeout")
                time.sleep(0.25)
            first = threading.Thread(target=request, args=("1",))
            first.start()
            time.sleep(0.05)
            with ThreadPoolExecutor(max_workers=2) as pool:
                list(pool.map(request, ["2", "3"]))
            first.join()
        finally:
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=5)
    timing = []
    for line in log_path.read_text().splitlines():
        try:
            value = json.loads(line)
        except json.JSONDecodeError:
            continue
        if value.get("event") == "moss_offline_final_timing":
            timing.append(value)
    responses = []
    for name in ("1", "2", "3"):
        text = (HERE / f"real-batch-{name}-response.json").read_text()
        value = json.loads(text)
        responses.append({"request": name, "has_transcript": isinstance(value.get("text"), str)})
    evidence = {"head": "1dd8770ce437dea203862c030850a76b03cf891a", "max_batch": 2, "request_count": 3, "timing_event_count": len(timing), "timing": timing, "responses": responses}
    (HERE / "real-batch-evidence.json").write_text(json.dumps(evidence, indent=2) + "\n")
    print(json.dumps(evidence, indent=2))
    if len(timing) != 3:
        raise AssertionError(f"three completed offline Final requests produced {len(timing)} timing events; max-batch=2 drops events from TranscribeBatch")


if __name__ == "__main__":
    main()
