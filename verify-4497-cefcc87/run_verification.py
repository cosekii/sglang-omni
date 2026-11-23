#!/usr/bin/env python3
import argparse
import json
import os
from pathlib import Path
import subprocess


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def run(binary, scenario, output_dir, env_flags=None, sample_count=None):
    env = os.environ.copy()
    env.update(env_flags or {})
    command = [str(binary), scenario]
    if sample_count is not None:
        command.append(str(sample_count))
    completed = subprocess.run(command, env=env, text=True, capture_output=True, timeout=15)
    name = scenario if sample_count is None else f"{scenario}-{sample_count}"
    (output_dir / f"{name}.stdout.jsonl").write_text(completed.stdout)
    (output_dir / f"{name}.stderr.log").write_text(completed.stderr)
    require(completed.returncode == 0, f"{name}: exit {completed.returncode}: {completed.stderr}")
    return [json.loads(line) for line in completed.stdout.splitlines()]


def check_probe(binary, samples, expected_preview, output_dir, arm, evidence):
    result = run(binary, "probe", output_dir, sample_count=samples)[0]
    segments = result["segments"]
    require(bool(segments) == expected_preview, f"{arm} probe {samples}: {result}")
    require(result["transcribe_entered"] == int(expected_preview), f"{arm} probe {samples}: {result}")
    if segments:
        require(len(segments) == 1 and segments[0].get("is_final") is False,
                f"{arm} probe {samples}: {segments}")
    evidence[str(samples)] = {"expected_preview": expected_preview, "result": "pass"}


def final_kinds(result):
    return [segment["is_final"] for segment in result["segments"]]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--bin", required=True, type=Path)
    parser.add_argument("--old-bin", required=True, type=Path)
    parser.add_argument("--head", required=True)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    evidence = {"head": args.head, "current": {}, "old_gate": {}, "surrounding": {}, "stale_preview": []}

    for samples, expected in [(11999, False), (12000, True), (12001, True)]:
        check_probe(args.bin, samples, expected, args.output, "current", evidence["current"])
    for samples, expected in [(12000, False), (19999, False), (20000, True)]:
        check_probe(args.old_bin, samples, expected, args.output, "old", evidence["old_gate"])

    for scenario in ["cadence", "final", "window_preempt", "flush", "invalid"]:
        events = run(args.bin, scenario, args.output)
        evidence["surrounding"][scenario] = {"events": events, "result": "pass"}
    evidence["surrounding"]["error"] = {
        "events": run(args.bin, "error", args.output, {"MOSS_TEST_FAIL_FIRST": "1"}),
        "result": "pass",
    }
    evidence["surrounding"]["cancel"] = {
        "events": run(args.bin, "cancel", args.output, {"MOSS_TEST_WAIT_CANCEL": "1"}),
        "result": "pass",
    }

    for iteration in range(1, 4):
        stale_dir = args.output / f"stale-{iteration}"
        stale_dir.mkdir()
        for scenario in ("stale_short", "stale_window"):
            result = run(args.bin, scenario, stale_dir, {
                "MOSS_TEST_WAIT_GATE": "1",
                "MOSS_TEST_IGNORE_CANCEL": "1",
                "MOSS_TEST_STALE_RESULT": "1",
            })[0]
            segments = result["segments"]
            require(len(segments) == 1 and segments[0].get("is_final") is True,
                    f"{scenario}: cancelled preview published: {result}")
            require(segments[0].get("text") == "FINAL_TRANSCRIPT",
                    f"{scenario}: final text replaced: {result}")
            evidence["stale_preview"].append({
                "run": iteration,
                "scenario": scenario,
                "published_kinds": final_kinds(result),
                "final_text": segments[0]["text"],
                "result": "pass",
            })

    short_reproductions = []
    timeline_reproductions = []
    for iteration in range(1, 4):
        short_dir = args.output / f"short-{iteration}"
        timeline_dir = args.output / f"timeline-{iteration}"
        short_dir.mkdir()
        timeline_dir.mkdir()
        current = run(args.bin, "short_stop", short_dir, {"MOSS_TEST_DELAY_MS": "400"})[0]
        old = run(args.old_bin, "short_stop", short_dir, {"MOSS_TEST_DELAY_MS": "400"})[0]
        require(final_kinds(current) == [True], f"short stop current decodes: {current}")
        require(final_kinds(old) == [True], f"short stop old decodes: {old}")
        require(current["segments"][-1]["text"] == old["segments"][-1]["text"],
                "short stop final text changed")
        require(current["elapsed_ms"] <= old["elapsed_ms"] + 100,
                f"short stop Final still later: {current} vs {old}")
        require(current["elapsed_ms"] < 700,
                f"short stop waited out two controlled decodes: {current}")
        short_reproductions.append({
            "run": iteration,
            "current_elapsed_ms": current["elapsed_ms"],
            "old_gate_elapsed_ms": old["elapsed_ms"],
            "new_minus_old_ms": current["elapsed_ms"] - old["elapsed_ms"],
            "current_decodes": final_kinds(current),
            "old_gate_decodes": final_kinds(old),
            "final_text_identical": True,
            "result": "pass",
        })

        current = run(args.bin, "timeline_current", timeline_dir, {"MOSS_TEST_DELAY_MS": "400"})[0]
        old = run(args.old_bin, "timeline_old", timeline_dir, {"MOSS_TEST_DELAY_MS": "400"})[0]
        require(final_kinds(current) == [False, False, False, True],
                f"timeline current decodes: {current}")
        require(final_kinds(old) == [False, False, False, True],
                f"timeline old decodes: {old}")
        require(current["segments"][-1]["text"] == old["segments"][-1]["text"],
                "timeline final text changed")
        require(current["elapsed_ms"] <= old["elapsed_ms"] + 100,
                f"timeline Final still later: {current} vs {old}")
        timeline_reproductions.append({
            "run": iteration,
            "current_elapsed_ms": current["elapsed_ms"],
            "old_gate_elapsed_ms": old["elapsed_ms"],
            "new_minus_old_ms": current["elapsed_ms"] - old["elapsed_ms"],
            "current_preview_count": final_kinds(current).count(False),
            "old_gate_preview_count": final_kinds(old).count(False),
            "final_text_identical": True,
            "result": "pass",
        })

    evidence["short_stop"] = {
        "input": {"append_samples": 12000, "done_at_ms": 100},
        "controlled_decode_ms": 400,
        "cancel_poll_ms": 1,
        "no_later_slack_ms": 100,
        "reproductions": short_reproductions,
    }
    evidence["timeline"] = {
        "controlled_decode_ms": 400,
        "cancel_poll_ms": 1,
        "no_later_slack_ms": 100,
        "reproductions": timeline_reproductions,
    }
    output = args.output / "followup-evidence.json"
    output.write_text(json.dumps(evidence, indent=2) + "\n")
    print(json.dumps({"short_stop": short_reproductions, "timeline": timeline_reproductions}, indent=2))
    print(f"PASS: {output}")


if __name__ == "__main__":
    main()
