#!/usr/bin/env python3
import argparse
import os
import json
import pandas as pd
import numpy as np
import matplotlib.pyplot as plt


# ============================================================
# Helper
# ============================================================

def load_results_csv(suite_name):
    suite_dir = os.path.join("experiments", suite_name)
    results_path = os.path.join(suite_dir, f"{suite_name}_results.csv")

    if not os.path.exists(results_path):
        print(f"[ERROR] results file not found at: {results_path}")
        return None, suite_dir

    df = pd.read_csv(results_path)
    return df, suite_dir


def safe_save(path):
    os.makedirs(os.path.dirname(path), exist_ok=True)


# ============================================================
# Plot: Convergence vs N
# ============================================================

def plot_convergence_vs_n(df, outdir):
    if "hosts" not in df or "convergence_sec" not in df:
        return

    df_sorted = df.sort_values("hosts")

    plt.figure(figsize=(8, 4))
    plt.plot(df_sorted["hosts"], df_sorted["convergence_sec"], marker="o")
    plt.xlabel("Number of Nodes (N)")
    plt.ylabel("Convergence Time (sec)")
    plt.title("Convergence Time vs Number of Nodes")
    plt.grid(True)

    outpath = os.path.join(outdir, "convergence_vs_nodes.png")
    plt.savefig(outpath)
    plt.close()


# ============================================================
# Plot: Convergence vs Loss
# ============================================================

def plot_convergence_vs_loss(df, outdir):
    if "loss" not in df or "convergence_sec" not in df:
        return

    # ensure loss is numeric
    df["loss_num"] = df["loss"].astype(str).str.replace("%", "").astype(float)

    df_sorted = df.sort_values("loss_num")

    plt.figure(figsize=(8, 4))
    plt.plot(df_sorted["loss_num"], df_sorted["convergence_sec"], marker="o")
    plt.xlabel("Packet Loss (%)")
    plt.ylabel("Convergence Time (sec)")
    plt.title("Convergence vs Loss Rate")
    plt.grid(True)

    outpath = os.path.join(outdir, "convergence_vs_loss.png")
    plt.savefig(outpath)
    plt.close()


# ============================================================
# Plot: Convergence vs Fanout
# ============================================================

def plot_convergence_vs_fanout(df, outdir):
    if "fanout" not in df or "convergence_sec" not in df:
        return

    df_sorted = df.sort_values("fanout")

    plt.figure(figsize=(8, 4))
    plt.plot(df_sorted["fanout"], df_sorted["convergence_sec"], marker="o")
    plt.xlabel("Fanout")
    plt.ylabel("Convergence Time (sec)")
    plt.title("Convergence vs Fanout")
    plt.grid(True)

    outpath = os.path.join(outdir, "convergence_vs_fanout.png")
    plt.savefig(outpath)
    plt.close()


# ============================================================
# Plot: Convergence vs Delay
# ============================================================

def plot_convergence_vs_delay(df, outdir):
    if "delay" not in df or "convergence_sec" not in df:
        return

    # Extract numeric ms value
    df["delay_ms"] = df["delay"].astype(str).str.replace("ms", "").astype(float)
    df_sorted = df.sort_values("delay_ms")

    plt.figure(figsize=(8, 4))
    plt.plot(df_sorted["delay_ms"], df_sorted["convergence_sec"], marker="o")
    plt.xlabel("Delay (ms)")
    plt.ylabel("Convergence Time (sec)")
    plt.title("Convergence vs Delay")
    plt.grid(True)

    outpath = os.path.join(outdir, "convergence_vs_delay.png")
    plt.savefig(outpath)
    plt.close()


# ============================================================
# Scan subfolders for delivery CDFs and combine (optional)
# ============================================================

def plot_combined_cdf(suite_dir):
    cdfs = []
    labels = []

    for root, dirs, files in os.walk(suite_dir):
        if "delivery_cdf.png" in files:
            p = os.path.join(root, "delivery_cdf.png")
            labels.append(os.path.basename(root))
            cdfs.append(p)

    if len(cdfs) == 0:
        return

    print(f"[INFO] Found {len(cdfs)} CDFs for combined plot")

    # Just stack images vertically as a combined report
    from PIL import Image

    images = [Image.open(p) for p in cdfs]
    widths = [img.width for img in images]
    heights = [img.height for img in images]

    total_h = sum(heights)
    max_w = max(widths)

    combined = Image.new("RGB", (max_w, total_h), "white")

    y_offset = 0
    for img in images:
        combined.paste(img, (0, y_offset))
        y_offset += img.height

    combined.save(os.path.join(suite_dir, "combined_delivery_cdfs.png"))

    print("[INFO] combined_delivery_cdfs.png generated")


# ============================================================
# MAIN
# ============================================================

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--suite-name", required=True)
    args = ap.parse_args()

    df, suite_dir = load_results_csv(args.suite_name)
    if df is None:
        return

    # High-level plots
    plot_convergence_vs_n(df, suite_dir)
    plot_convergence_vs_loss(df, suite_dir)
    plot_convergence_vs_fanout(df, suite_dir)
    plot_convergence_vs_delay(df, suite_dir)

    # Optional combined plot from subfolders
    plot_combined_cdf(suite_dir)

    print(f"[INFO] Plotting complete for suite: {args.suite_name}")


if __name__ == "__main__":
    main()
