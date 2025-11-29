#!/usr/bin/env python3
"""
Run a batch of gossip experiments over Mininet and save logs + results.

This script does NOT plot. Plotting is handled by plot_results.py.

Usage (default experiments):
  cd CS240/project
  sudo python3 run_experiments.py

Give the whole suite a name:
  sudo python3 run_experiments.py --suite-name nov29_run

Run a single custom experiment instead of defaults:
  sudo python3 run_experiments.py \
    --suite-name highloss_test \
    --exp-name highloss_5hosts \
    --hosts 5 \
    --bw 5 \
    --delay 50ms \
    --loss 2
"""

import argparse
import csv
import os
import re
import shutil
import subprocess
import sys
from datetime import datetime
from pathlib import Path

DEFAULT_EXPERIMENTS = [
    {"name": "period_50ms",  "hosts": 10, "bw": 10, "delay": "20ms", "loss": 0, "fanout": 2, "period": "50ms"},
    {"name": "period_200ms", "hosts": 10, "bw": 10, "delay": "20ms", "loss": 0, "fanout": 2, "period": "200ms"},
    {"name": "period_500ms", "hosts": 10, "bw": 10, "delay": "20ms", "loss": 0, "fanout": 2, "period": "500ms"},
    
    {"name": "TTL1", "hosts": 10, "bw": 10, "delay": "20ms", "loss": 0, "fanout": 1, "ttl": 1},
    {"name": "TTL2", "hosts": 10, "bw": 10, "delay": "20ms", "loss": 0, "fanout": 1, "ttl": 2},
    {"name": "TTL4", "hosts": 10, "bw": 10, "delay": "20ms", "loss": 0, "fanout": 1, "ttl": 4},
    {"name": "TTL8", "hosts": 10, "bw": 10, "delay": "20ms", "loss": 0, "fanout": 1, "ttl": 8},

]




GOSSIP_LOG_GLOB = "/tmp/gossip-*.jsonl"


# -----------------------
# CLI
# -----------------------

def parse_args():
    p = argparse.ArgumentParser(
        description="Run gossip experiments and save logs/results (no plotting)."
    )

    p.add_argument(
        "--bin",
        default="bin/linux/gossipd",
        help="Path to gossipd binary (default: bin/linux/gossipd).",
    )

    # Suite naming
    p.add_argument(
        "--suite-name",
        default=None,
        help="Optional name for this whole experiment suite. "
             "If omitted, a timestamp-based name is used."
    )

    # Optional: single custom experiment instead of defaults
    p.add_argument("--exp-name", default=None,
                   help="If set, run a single custom experiment with this name.")
    p.add_argument("--hosts", type=int, default=3,
                   help="Number of hosts for custom experiment.")
    p.add_argument("--bw", type=float, default=10.0,
                   help="Bandwidth (Mbps) for custom experiment.")
    p.add_argument("--delay", default="10ms",
                   help="Delay for custom experiment (e.g. 10ms, 50ms).")
    p.add_argument("--loss", type=float, default=0.0,
                   help="Loss percentage for custom experiment.")

    return p.parse_args()


# -----------------------
# Helpers
# -----------------------

def run_cmd(cmd, cwd=None, check=True):
    print(f"[CMD] {cmd} (cwd={cwd or os.getcwd()})")
    return subprocess.run(
        cmd,
        cwd=cwd,
        shell=isinstance(cmd, str),
        check=check,
        capture_output=False,
        text=True,
    )


def run_and_capture(cmd, cwd=None):
    proc = subprocess.run(
        cmd,
        cwd=cwd,
        shell=isinstance(cmd, str),
        check=False,
        capture_output=True,
        text=True,
    )
    return proc.returncode, proc.stdout, proc.stderr


def clean_environment():
    print("[INFO] Cleaning Mininet and old logs")
    run_cmd(["mn", "-c"], check=False)
    run_cmd("pkill gossipd || true", check=False)
    run_cmd(f"rm -f {GOSSIP_LOG_GLOB}", check=False)


def parse_analyzer_output(stdout: str):
    """
    Parse convergence summary from analyze_logs.py output.

    Returns (nodes_delivered, convergence_sec).
    """
    nodes = None
    conv = None

    for line in stdout.splitlines():
        m_nodes = re.search(
            r"Nodes that delivered rumor .*: (\d+)", line
        )
        if m_nodes:
            nodes = int(m_nodes.group(1))

        m_conv = re.search(
            r"Convergence time \(sec\):\s+([0-9.]+)", line
        )
        if m_conv:
            conv = float(m_conv.group(1))

    return nodes, conv


def make_suite_dir(project_dir: Path, suite_name_arg: str | None) -> Path:
    """
    Decide suite name and create a unique directory under experiments/.
    If suite_name_arg is None, generate a timestamp name.
    If name exists, append _2, _3, etc.
    """
    experiments_root = project_dir / "experiments"
    experiments_root.mkdir(exist_ok=True)

    if suite_name_arg:
        base = suite_name_arg
    else:
        base = "suite_" + datetime.now().strftime("%Y%m%d_%H%M%S")

    suite_dir = experiments_root / base
    if not suite_dir.exists():
        suite_dir.mkdir()
        print(f"[INFO] Suite directory: {suite_dir}")
        return suite_dir

    idx = 2
    while True:
        cand = experiments_root / f"{base}_{idx}"
        if not cand.exists():
            cand.mkdir()
            print(f"[INFO] Suite directory exists, using: {cand}")
            return cand
        idx += 1


def archive_logs(exp_dir: Path):
    logs_dir = exp_dir / "logs"
    logs_dir.mkdir(parents=True, exist_ok=True)

    for path in Path("/tmp").glob("gossip-*.jsonl"):
        target = logs_dir / path.name
        print(f"[INFO] Archiving {path} -> {target}")
        shutil.copy2(path, target)


def append_results_row(row, results_csv_path: Path):
    file_exists = results_csv_path.exists()
    with results_csv_path.open("a", newline="") as f:
        writer = csv.writer(f)
        if not file_exists:
            writer.writerow([
                "name", "hosts", "bw", "delay", "loss",
                "success", "nodes_delivered", "convergence_sec",
            ])
        writer.writerow([
            row["name"],
            row["hosts"],
            row["bw"],
            row["delay"],
            row["loss"],
            int(row["success"]),
            row["nodes_delivered"],
            row["convergence_sec"],
        ])


# -----------------------
# Core experiment runner
# -----------------------

def run_experiment(exp, project_dir: Path, bin_path: str, suite_dir: Path):
    """
    Run a single experiment:
      - clean env
      - run topo.py in headless mode
      - run analyze_logs.py
      - archive logs into suite/<exp_name>/logs
      - return summary row
    """
    name = exp["name"]
    hosts = exp["hosts"]
    bw = exp["bw"]
    delay = exp["delay"]
    loss = exp["loss"]

    print(f"\n========== Experiment: {name} ==========")

    rumor_id = f"{name}_{datetime.now().strftime('%Y%m%d_%H%M%S')}"

    clean_environment()

    exp_dir = suite_dir / name
    exp_dir.mkdir(exist_ok=True)

    # Run topo.py headless
    fanout = exp.get("fanout", 3)
    ttl = exp.get("ttl", 8)
    period = exp.get("period", "200ms")

    topo_cmd = [
        "python3", "topo.py",
        "--bin", bin_path,
        "--hosts", str(hosts),
        "--bw", str(bw),
        "--delay", delay,
        "--loss", str(loss),
        "--fanout", str(fanout),
        "--ttl", str(ttl),
        "--period", period,
        "--metrics-port", "9080",
        "--logdir", "/tmp",
        "--no-cli",
        "--inject-rumor", rumor_id,
        "--inject-ttl", "8",
        "--runtime", "2.0",
    ]
    run_cmd(topo_cmd, cwd=project_dir)

    # Analyze logs
    analyzer_cmd = [
        "python3", "analyze_logs.py",
        "--rumor-id", rumor_id,
        "--log-glob", GOSSIP_LOG_GLOB,
        "--min-hosts", str(hosts),
    ]
    ret, out, err = run_and_capture(analyzer_cmd, cwd=project_dir)
    print("----- analyzer stdout -----")
    print(out)
    if err.strip():
        print("----- analyzer stderr -----")
        print(err)

    success = (ret == 0)
    nodes_delivered, convergence_sec = parse_analyzer_output(out)

    archive_logs(exp_dir)

    return {
        "name": name,
        "hosts": hosts,
        "bw": bw,
        "delay": delay,
        "loss": loss,
        "success": success,
        "nodes_delivered": nodes_delivered,
        "convergence_sec": convergence_sec,
    }


def main():
    if os.geteuid() != 0:
        print("ERROR: run this script with sudo (Mininet requires root).", file=sys.stderr)
        sys.exit(1)

    args = parse_args()

    project_dir = Path(__file__).resolve().parent
    os.chdir(project_dir)

    suite_dir = make_suite_dir(project_dir, args.suite_name)
    results_csv_path = suite_dir / "results.csv"

    if args.exp_name:
        experiments = [{
            "name": args.exp_name,
            "hosts": args.hosts,
            "bw": args.bw,
            "delay": args.delay,
            "loss": args.loss,
        }]
    else:
        experiments = DEFAULT_EXPERIMENTS

    all_results = []

    for exp in experiments:
        result = run_experiment(exp, project_dir, args.bin, suite_dir)
        all_results.append(result)
        append_results_row(result, results_csv_path)

    print(f"\n[INFO] All experiments done.")
    print(f"[INFO] Suite directory: {suite_dir}")
    print(f"[INFO] Results: {results_csv_path}")
    print(f"[INFO] Per-experiment logs under: {suite_dir}/<exp_name>/logs/")


if __name__ == "__main__":
    main()
