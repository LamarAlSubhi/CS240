#!/usr/bin/env python3
import argparse
import csv
import os
import subprocess
import sys
import shutil
import time
from datetime import datetime
from pathlib import Path


# =============================================================
# Helpers
# =============================================================

def run_cmd(cmd, cwd=None, check=True):
    print(f"[CMD] {cmd}  (cwd={cwd or os.getcwd()})")
    return subprocess.run(
        cmd,
        cwd=cwd,
        shell=isinstance(cmd, str),
        check=check,
        capture_output=False,
        text=True
    )


def run_capture(cmd, cwd=None):
    proc = subprocess.run(
        cmd,
        cwd=cwd,
        shell=isinstance(cmd, str),
        capture_output=True,
        text=True
    )
    return proc.returncode, proc.stdout, proc.stderr


def clean_environment():
    print("[INFO] Cleaning Mininet, old logs, old daemons")
    run_cmd(["mn", "-c"], check=False)
    run_cmd("pkill -9 gossipd || true", check=False)
    run_cmd("rm -f /tmp/gossip-*.jsonl", check=False)


def fresh_suite_dir(root: Path, name: str):
    suite_dir = root / name
    if suite_dir.exists():
        print(f"[WARN] Suite dir exists, deleting: {suite_dir}")
        shutil.rmtree(suite_dir)
    suite_dir.mkdir(parents=True)
    return suite_dir


def read_convergence_txt(path):
    try:
        with open(path, "r") as f:
            return float(f.read().strip())
    except:
        return None


def append_results_row(result_row, csv_path):
    file_exists = csv_path.exists()
    with csv_path.open("a", newline="") as f:
        writer = csv.writer(f)
        if not file_exists:
            writer.writerow([
                "name", "hosts", "bw", "delay", "loss",
                "success", "nodes_delivered", "convergence_sec"
            ])
        writer.writerow([
            result_row["name"],
            result_row["hosts"],
            result_row["bw"],
            result_row["delay"],
            result_row["loss"],
            int(result_row["success"]),
            result_row["nodes_delivered"],
            result_row["convergence_sec"],
        ])


# =============================================================
# Experiment Definitions
# =============================================================

def load_suite_config(name):
    """
    Predefined suites.
    """
    if name == "scaling_N":
        return [
            {"name": f"n{n}", "hosts": n, "bw": 10, "delay": "10ms", "loss": 0}
            for n in [5, 10, 20, 30, 40, 50]
        ]

    if name == "fanout_sweep":
        return [
            {"name": f"fanout{f}", "hosts": 20, "bw": 10, "delay": "10ms", "loss": 0, "fanout": f}
            for f in [1, 2, 3, 4, 5]
        ]

    if name == "loss_sweep":
        return [
            {"name": f"loss{p}", "hosts": 20, "bw": 10, "delay": "10ms", "loss": p}
            for p in [0, 5, 10, 20, 40]
        ]

    if name == "delay_sweep":
        return [
            {"name": f"delay{d}", "hosts": 20, "bw": 10, "delay": f"{d}ms", "loss": 0}
            for d in [0, 10, 20, 40, 80]
        ]

    if name == "jitter_sweep":
        return [
            {"name": f"jitter{j}", "hosts": 20, "bw": 10, "delay": "20ms", "loss": 0, "jitter": f"{j}ms"}
            for j in [0, 5, 10, 20, 40]
        ]

    if name == "churn_sweep":
        return [
            {"name": f"churn{k}", "hosts": 20, "bw": 10, "delay": "20ms", "loss": 0}
            for k in [1, 3, 5]
        ]

    if name == "zombie_test":
        return [
            {"name": "zombie_single", "hosts": 20, "bw": 10, "delay": "10ms", "loss": 0}
        ]

    print(f"[ERROR] Unknown suite: {name}")
    sys.exit(1)


# =============================================================
# Core Experiment Logic
# =============================================================

def run_experiment(exp, bin_path, suite_dir: Path, project_dir: Path):
    name = exp["name"]
    print(f"\n========== Running experiment: {name} ==========")

    clean_environment()

    exp_dir = suite_dir / name
    exp_dir.mkdir(exist_ok=True)

    rumor_id = f"{name}_{datetime.now().strftime('%Y%m%d_%H%M%S')}"

    topo_cmd = [
        "python3", "topo.py",
        "--bin", bin_path,
        "--hosts", str(exp["hosts"]),
        "--bw", str(exp.get("bw", 10)),
        "--delay", exp.get("delay", "10ms"),
        "--loss", str(exp.get("loss", 0)),
        "--fanout", str(exp.get("fanout", 3)),
        "--ttl", "12",
        "--period", "200ms",
        "--metrics-port", "9080",
        "--logdir", "/tmp",
        "--no-cli",
        "--inject-rumor", rumor_id,
        "--inject-ttl", "8",
        "--runtime", "4.0"
    ]

    # Add optional netem params
    if "jitter" in exp:    topo_cmd += ["--jitter", exp["jitter"]]
    if "reorder" in exp:   topo_cmd += ["--reorder", str(exp["reorder"])]
    if "corrupt" in exp:   topo_cmd += ["--corrupt", str(exp["corrupt"])]
    if "duplicate" in exp: topo_cmd += ["--duplicate", str(exp["duplicate"])]
    if "burst" in exp:     topo_cmd += ["--burst", str(exp["burst"])]

    run_cmd(topo_cmd, cwd=project_dir)

    # Analyze logs
    analyze_cmd = [
        "python3", "analyze_logs.py",
        "--rumor-id", rumor_id,
        "--log-glob", "/tmp/gossip-*.jsonl",
        "--outdir", str(exp_dir),
        "--min-hosts", str(exp["hosts"])
    ]
    ret, out, err = run_capture(analyze_cmd, cwd=project_dir)
    success = (ret == 0)

    convergence_path = exp_dir / "convergence.txt"
    conv = read_convergence_txt(convergence_path)

    # Count delivered nodes
    delivered = 0
    delivery_curve_path = exp_dir / "node_delivery_times.csv"
    if delivery_curve_path.exists():
        try:
            import pandas as pd
            delivered = len(pd.read_csv(delivery_curve_path))
        except:
            delivered = 0

    return {
        "name": name,
        "hosts": exp["hosts"],
        "bw": exp.get("bw", 10),
        "delay": exp.get("delay", "10ms"),
        "loss": exp.get("loss", 0),
        "success": success,
        "nodes_delivered": delivered,
        "convergence_sec": conv
    }


# =============================================================
# Main
# =============================================================

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--suite-name", "--suite", required=True)
    parser.add_argument("--bin", default="bin/linux/gossipd")
    args = parser.parse_args()

    project_dir = Path(__file__).resolve().parent
    experiments_root = project_dir / "experiments"

    suite_name = args.suite_name
    suite_dir = fresh_suite_dir(experiments_root, suite_name)

    experiments = load_suite_config(suite_name)

    results_csv = suite_dir / f"{suite_name}_results.csv"
    print(f"[INFO] Saving results to: {results_csv}")

    all_results = []
    for exp in experiments:
        res = run_experiment(exp, args.bin, suite_dir, project_dir)
        all_results.append(res)
        append_results_row(res, results_csv)

    print("\n[INFO] Finished suite:", suite_name)
    print("[INFO] Results saved to:", results_csv)


if __name__ == "__main__":
    main()
