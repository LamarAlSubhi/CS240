#!/usr/bin/env python3
import argparse
import glob
import json
import os
import numpy as np
import matplotlib.pyplot as plt
import pandas as pd


# =========================
# Helpers
# =========================

def load_logs(pattern):
    logs = []
    for path in glob.glob(pattern):
        with open(path, "r") as f:
            for line in f:
                try:
                    logs.append(json.loads(line))
                except:
                    pass
    return logs


# =========================
# Parse events
# =========================

def parse_events(logs, rumor_id):
    inject_ts = None
    delivery_times = {}
    sends = []
    recvs = []

    for ev in logs:
        if ev.get("id") != rumor_id:
            continue

        if ev["ev"] == "inject":
            inject_ts = ev["ts"]

        elif ev["ev"] == "deliver":
            node = ev["node"]
            if node not in delivery_times:
                delivery_times[node] = ev["ts"]

        elif ev["ev"] == "send":
            sends.append(ev)

        elif ev["ev"] == "recv":
            recvs.append(ev)

    return inject_ts, delivery_times, sends, recvs


# =========================
# Plot: Delivery CDF
# =========================

def plot_delivery_cdf(latencies, outdir):
    x = sorted(latencies)
    y = np.linspace(0, 1, len(x))

    plt.figure(figsize=(6, 4))
    plt.plot(x, y)
    plt.grid(True)
    plt.xlabel("Latency (s)")
    plt.ylabel("CDF")
    plt.title("Delivery Latency CDF")

    plt.savefig(os.path.join(outdir, "delivery_cdf.png"))
    plt.close()


# =========================
# Plot: Delivery Curve
# =========================

def plot_delivery_curve(latencies, outdir):
    xs = sorted(latencies)
    ys = [i / len(xs) for i in range(len(xs))]

    plt.figure(figsize=(6, 4))
    plt.plot(xs, ys)
    plt.grid(True)
    plt.xlabel("Time (s)")
    plt.ylabel("% delivered")
    plt.title("Delivery Curve")

    plt.savefig(os.path.join(outdir, "delivery_curve.png"))
    plt.close()


# =========================
# Plot: Message Overhead
# =========================

def plot_overhead(sends, recvs, outdir):
    counts = {}
    for ev in sends:
        n = ev["node"]
        counts[n] = counts.get(n, 0) + 1
    for ev in recvs:
        n = ev["node"]
        counts[n] = counts.get(n, 0) + 1

    df = pd.DataFrame({
        "node": list(counts.keys()),
        "messages": list(counts.values())
    }).sort_values("node")

    df.to_csv(os.path.join(outdir, "overhead.csv"), index=False)

    plt.figure(figsize=(10, 4))
    plt.bar(df["node"], df["messages"])
    plt.xticks(rotation=90)
    plt.ylabel("Messages")
    plt.title("Message Overhead")
    plt.grid(True, axis="y")
    plt.savefig(os.path.join(outdir, "overhead.png"))
    plt.close()


# =========================
# Plot: Node Delivery Table (pretty version)
# =========================

def plot_node_delivery_table(delivery_times_s, outdir):
    """
    Makes a table plot that scales to 50/100 nodes.
    Much cleaner than a bar graph.
    """
    df = pd.DataFrame({
        "node": list(delivery_times_s.keys()),
        "latency_s": list(delivery_times_s.values())
    }).sort_values("node")

    df.to_csv(os.path.join(outdir, "node_delivery_times.csv"), index=False)

    fig, ax = plt.subplots(figsize=(8, len(df) * 0.25 + 1))
    ax.axis('tight')
    ax.axis('off')

    table = ax.table(
        cellText=df.values,
        colLabels=df.columns,
        loc='center'
    )

    table.auto_set_font_size(False)
    table.set_fontsize(10)

    plt.savefig(os.path.join(outdir, "node_delivery_table.png"), bbox_inches='tight')
    plt.close()


# =========================
# Plot: Message rate over time
# =========================

def plot_message_rate(sends, recvs, inject_ts, outdir, bucket_size=0.2):
    times = [(ev["ts"] - inject_ts) / 1e9 for ev in (sends + recvs)]

    if not times:
        return

    max_t = max(times)
    bins = np.arange(0, max_t + bucket_size, bucket_size)
    hist, _ = np.histogram(times, bins=bins)

    df = pd.DataFrame({
        "time_bucket_start": bins[:-1],
        "messages": hist
    })
    df.to_csv(os.path.join(outdir, "message_rate.csv"), index=False)

    plt.figure(figsize=(9, 4))
    plt.plot(bins[:-1], hist)
    plt.grid(True)
    plt.xlabel("Time (s)")
    plt.ylabel("Messages")
    plt.title("Message Rate Over Time")
    plt.savefig(os.path.join(outdir, "message_rate.png"))
    plt.close()


# =========================
# Zombie detection
# =========================

def zombie_report(delivery_dict, sends, recvs, total_hosts, outdir):
    delivered_nodes = set(delivery_dict.keys())
    senders = set(ev["node"] for ev in sends)
    receivers = set(ev["node"] for ev in recvs)

    all_nodes = set(str(i) for i in range(1, total_hosts + 1))

    zombies = all_nodes - delivered_nodes
    no_send = all_nodes - senders
    no_recv = all_nodes - receivers

    report = [
        f"Total hosts: {total_hosts}",
        f"Delivered: {len(delivered_nodes)}/{total_hosts}",
        f"Zombie nodes (never delivered): {sorted(zombies)}",
        f"Nodes that never sent: {sorted(no_send)}",
        f"Nodes that never received: {sorted(no_recv)}",
    ]

    with open(os.path.join(outdir, "zombie_report.txt"), "w") as f:
        f.write("\n".join(report))

    plt.figure(figsize=(4, 4))
    plt.bar(["Delivered", "Not Delivered"], [len(delivered_nodes), len(zombies)])
    plt.title("Zombie Bar Chart")
    plt.savefig(os.path.join(outdir, "zombie_bar.png"))
    plt.close()


# =========================
# Main
# =========================

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--rumor-id", required=True)
    ap.add_argument("--log-glob", required=True)
    ap.add_argument("--outdir", required=False)
    ap.add_argument("--min-hosts", type=int, default=1)
    args = ap.parse_args()

    outdir = args.outdir if args.outdir else os.getcwd()
    os.makedirs(outdir, exist_ok=True)

    logs = load_logs(args.log_glob)

    inject_ts, delivery_dict, sends, recvs = parse_events(logs, args.rumor_id)

    if inject_ts is None:
        print("[ERROR] No inject event found")
        return

    delivery_latencies = [(ts - inject_ts) / 1e9 for ts in delivery_dict.values()]

    # Save convergence
    with open(os.path.join(outdir, "convergence.txt"), "w") as f:
        f.write(str(max(delivery_latencies)) if delivery_latencies else "0")

    # Plots
    if delivery_latencies:
        plot_delivery_cdf(delivery_latencies, outdir)
        plot_delivery_curve(delivery_latencies, outdir)
        plot_node_delivery_table(
            {node: (ts - inject_ts) / 1e9 for node, ts in delivery_dict.items()},
            outdir
        )

    plot_overhead(sends, recvs, outdir)
    plot_message_rate(sends, recvs, inject_ts, outdir)

    zombie_report(delivery_dict, sends, recvs, args.min_hosts, outdir)


if __name__ == "__main__":
    main()
