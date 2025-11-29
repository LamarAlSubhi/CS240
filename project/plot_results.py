#!/usr/bin/env python3
"""
Plot results for a given experiment suite.

Usage:
  python3 plot_results.py                 # plots latest suite
  python3 plot_results.py --suite-name X  # plots experiments/X
"""

import argparse
import csv
import os
from pathlib import Path

import matplotlib.pyplot as plt


def parse_args():
    p = argparse.ArgumentParser(
        description="Plot gossip experiment results for a suite."
    )
    p.add_argument(
        "--suite-name",
        default=None,
        help="Name of the suite directory under experiments/. "
             "If omitted, the most recently modified suite is used.",
    )
    return p.parse_args()


def find_suite_dir(project_dir: Path, suite_name: str | None) -> Path:
    experiments_root = project_dir / "experiments"
    if not experiments_root.exists():
        raise SystemExit("[ERROR] experiments/ directory does not exist.")

    if suite_name:
        suite_dir = experiments_root / suite_name
        if not suite_dir.exists():
            raise SystemExit(f"[ERROR] Suite {suite_name!r} not found under {experiments_root}")
        return suite_dir

    dirs = [d for d in experiments_root.iterdir() if d.is_dir()]
    if not dirs:
        raise SystemExit("[ERROR] No suites found under experiments/.")

    suite_dir = max(dirs, key=lambda d: d.stat().st_mtime)
    print(f"[INFO] No suite-name given, using latest suite: {suite_dir.name}")
    return suite_dir


def load_results(suite_dir: Path):
    results_csv = suite_dir / "results.csv"
    if not results_csv.exists():
        raise SystemExit(f"[ERROR] {results_csv} does not exist.")

    rows = []
    with results_csv.open() as f:
        reader = csv.DictReader(f)
        for row in reader:
            try:
                row["convergence_sec"] = float(row["convergence_sec"])
            except (ValueError, TypeError):
                row["convergence_sec"] = None

            # parse delay like "20ms" -> 20.0
            delay_str = row.get("delay", "0ms")
            if delay_str.endswith("ms"):
                try:
                    row["delay_ms"] = float(delay_str[:-2])
                except ValueError:
                    row["delay_ms"] = None
            else:
                row["delay_ms"] = None

            # parse loss
            try:
                row["loss_pct"] = float(row.get("loss", 0))
            except (ValueError, TypeError):
                row["loss_pct"] = None

            rows.append(row)
    return rows


def plot_summary(suite_dir: Path, rows):
    """Horizontal bar chart, sorted by convergence time, with labels."""
    valid = [r for r in rows if r["convergence_sec"] is not None]
    if not valid:
        print("[WARN] No valid convergence values to plot in summary.")
        return

    # sort by convergence time
    valid.sort(key=lambda r: r["convergence_sec"])

    labels = [
        f"{r['name']} (d={r['delay']}, loss={r['loss']})"
        for r in valid
    ]
    conv = [r["convergence_sec"] for r in valid]

    plt.figure(figsize=(8, max(3, 0.5 * len(valid))))
    y_pos = range(len(valid))
    plt.barh(y_pos, conv)
    plt.yticks(y_pos, labels)
    plt.xlabel("Convergence time (sec)")
    plt.title(f"Gossip convergence across experiments ({suite_dir.name})")

    # annotate bars with numeric values
    for i, v in enumerate(conv):
        plt.text(v, i, f" {v:.3f}s", va="center")

    plt.tight_layout()
    out_path = suite_dir / "convergence_summary.png"
    plt.savefig(out_path)
    plt.close()
    print(f"[INFO] Saved summary plot to {out_path}")


def plot_vs_delay(suite_dir: Path, rows):
    """Line plot: convergence vs delay for experiments with zero loss."""
    pts = [
        (r["delay_ms"], r["convergence_sec"])
        for r in rows
        if r["convergence_sec"] is not None and r["delay_ms"] is not None and r["loss_pct"] == 0
    ]
    if len(pts) < 2:
        print("[INFO] Not enough zero-loss points to plot convergence vs delay.")
        return

    pts.sort()
    delays = [d for d, _ in pts]
    conv = [c for _, c in pts]

    plt.figure()
    plt.plot(delays, conv, marker="o")
    plt.xlabel("Delay (ms)")
    plt.ylabel("Convergence time (sec)")
    plt.title(f"Convergence vs delay (loss = 0) - {suite_dir.name}")
    plt.grid(True)
    plt.tight_layout()

    out_path = suite_dir / "convergence_vs_delay.png"
    plt.savefig(out_path)
    plt.close()
    print(f"[INFO] Saved convergence-vs-delay plot to {out_path}")


def plot_vs_loss(suite_dir: Path, rows):
    """Line plot: convergence vs loss for experiments with delay 20 ms."""
    pts = [
        (r["loss_pct"], r["convergence_sec"])
        for r in rows
        if r["convergence_sec"] is not None and r["loss_pct"] is not None and r["delay_ms"] == 20.0
    ]
    if len(pts) < 2:
        print("[INFO] Not enough fixed-delay points to plot convergence vs loss.")
        return

    pts.sort()
    losses = [l for l, _ in pts]
    conv = [c for _, c in pts]

    plt.figure()
    plt.plot(losses, conv, marker="o")
    plt.xlabel("Loss (%)")
    plt.ylabel("Convergence time (sec)")
    plt.title(f"Convergence vs loss (delay = 20 ms) - {suite_dir.name}")
    plt.grid(True)
    plt.tight_layout()

    out_path = suite_dir / "convergence_vs_loss.png"
    plt.savefig(out_path)
    plt.close()
    print(f"[INFO] Saved convergence-vs-loss plot to {out_path}")


def main():
    args = parse_args()
    project_dir = Path(__file__).resolve().parent
    os.chdir(project_dir)

    suite_dir = find_suite_dir(project_dir, args.suite_name)
    rows = load_results(suite_dir)

    plot_summary(suite_dir, rows)
    plot_vs_delay(suite_dir, rows)
    plot_vs_loss(suite_dir, rows)

    print(f"[INFO] Plotting complete for suite: {suite_dir.name}")


if __name__ == "__main__":
    main()
