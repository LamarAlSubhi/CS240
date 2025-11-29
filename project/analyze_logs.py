#!/usr/bin/env python3
"""
============================================================
 Gossip Log Analyzer
============================================================

Reads gossip event logs written by gossipd and computes:

  - Which nodes delivered a given rumor.
  - Per-node delivery timestamps.
  - Global convergence time (last delivery - first delivery).

By default it looks for JSONL logs at: /tmp/gossip-*.jsonl,
and analyzes rumor id "exp1". Both are configurable via CLI.

Usage examples:

  python3 project/analyze_logs.py

  python3 project/analyze_logs.py --rumor-id test1

  python3 project/analyze_logs.py \
      --rumor-id exp1 \
      --log-glob "/tmp/gossip-*.jsonl" \
      --min-hosts 3 \
      --max-convergence 5.0
"""

# ===============================
# Imports
# ===============================

import argparse
import glob
import json
import os
import re
import sys
from typing import Dict, List, Tuple


# ===============================
# Data loading
# ===============================

def load_events(log_glob: str) -> List[Tuple[str, dict]]:
    """
    Load all JSONL events from files matching log_glob.

    Returns a list of (node_id, event_dict) pairs.
    node_id is derived from the filename (e.g. gossip-1.jsonl -> "1").
    """
    events: List[Tuple[str, dict]] = []

    for path in glob.glob(log_glob):
        node_id = guess_node_id_from_path(path)

        try:
            with open(path, "r") as f:
                for line in f:
                    line = line.strip()
                    if not line:
                        continue
                    try:
                        ev = json.loads(line)
                    except json.JSONDecodeError:
                        # Skip malformed lines rather than crashing
                        continue

                    events.append((node_id, ev))
        except OSError as e:
            print(f"[WARN] Could not read {path}: {e}", file=sys.stderr)

    return events


def guess_node_id_from_path(path: str) -> str:
    """
    Extract node id from filename gossip-<id>.jsonl.
    Falls back to the basename if no match.
    """
    name = os.path.basename(path)
    m = re.match(r"gossip-(\d+)\.jsonl$", name)
    if m:
        return m.group(1)
    return name


# ===============================
# Delivery / convergence logic
# ===============================

def extract_deliveries(
    events: List[Tuple[str, dict]],
    rumor_id: str
) -> Dict[str, int]:
    """
    From a list of (node_id, event) pairs, extract the first delivery
    timestamp per node for a given rumor id.

    Assumes events have:
      - ev: event type string, where "deliver" means rumor delivered
      - id: rumor id
      - ts: integer timestamp (nanoseconds)
    """
    first_delivery: Dict[str, int] = {}

    for node_id, ev in events:
        if ev.get("ev") != "deliver":
            continue
        if ev.get("id") != rumor_id:
            continue

        ts = ev.get("ts")
        if not isinstance(ts, int):
            continue

        if node_id not in first_delivery or ts < first_delivery[node_id]:
            first_delivery[node_id] = ts

    return first_delivery


def compute_convergence(deliveries: Dict[str, int]) -> Tuple[float, float]:
    """
    Given a mapping of node_id -> first delivery timestamp (ns),
    compute:

      - t0: time of first delivery (seconds)
      - conv: convergence time (last - first) in seconds

    Returns (t0_sec, conv_sec).
    """
    if not deliveries:
        return 0.0, 0.0

    ts_values = list(deliveries.values())
    t_min = min(ts_values)
    t_max = max(ts_values)

    t0_sec = t_min / 1e9
    conv_sec = (t_max - t_min) / 1e9
    return t0_sec, conv_sec


# ===============================
# CLI / main logic
# ===============================

def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Analyze gossipd logs and compute convergence time."
    )

    parser.add_argument(
        "--rumor-id",
        default="exp1",
        help="Rumor id to analyze (default: exp1)."
    )

    parser.add_argument(
        "--log-glob",
        default="/tmp/gossip-*.jsonl",
        help="Glob pattern for log files (default: /tmp/gossip-*.jsonl)."
    )

    parser.add_argument(
        "--min-hosts",
        type=int,
        default=0,
        help="If > 0, require at least this many hosts to have delivered."
    )

    parser.add_argument(
        "--max-convergence",
        type=float,
        default=None,
        help="If set, fail if convergence time exceeds this many seconds."
    )

    return parser.parse_args()


def main() -> None:
    args = parse_args()

    print(f"[INFO] Analyzing logs: {args.log_glob}")
    print(f"[INFO] Rumor id: {args.rumor_id}")

    events = load_events(args.log_glob)
    if not events:
        print("[ERROR] No events found. Did you run gossipd and generate logs?")
        sys.exit(1)

    deliveries = extract_deliveries(events, args.rumor_id)

    if not deliveries:
        print(f"[ERROR] No deliveries found for rumor id {args.rumor_id!r}.")
        sys.exit(1)

    t0_sec, conv_sec = compute_convergence(deliveries)

    # Sort nodes by id for stable printing
    nodes_sorted = sorted(deliveries.items(), key=lambda kv: kv[0])

    print("\n=== Delivery times (first deliver per node) ===")
    for node_id, ts_ns in nodes_sorted:
        dt_sec = (ts_ns / 1e9) - t0_sec
        print(f"  node {node_id}: ts={ts_ns}  (+{dt_sec:.6f} s from first)")

    print("\n=== Convergence summary ===")
    print(f"  Nodes that delivered rumor {args.rumor_id!r}: {len(deliveries)}")
    print(f"  First delivery time (sec): {t0_sec:.6f}")
    print(f"  Convergence time (sec):   {conv_sec:.6f}")

    # Optional “test mode” checks
    failed = False

    if args.min_hosts > 0 and len(deliveries) < args.min_hosts:
        print(
            f"[FAIL] Only {len(deliveries)} hosts delivered, "
            f"but min-hosts={args.min_hosts}."
        )
        failed = True

    if args.max_convergence is not None and conv_sec > args.max_convergence:
        print(
            f"[FAIL] Convergence time {conv_sec:.6f}s "
            f"exceeds max-convergence={args.max_convergence:.6f}s."
        )
        failed = True

    if failed:
        sys.exit(1)

    print("[OK] Gossip delivery satisfies current checks.")
    sys.exit(0)


if __name__ == "__main__":
    main()
