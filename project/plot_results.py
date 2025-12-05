#!/usr/bin/env python3
import os
import sys
import argparse
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns

EXPERIMENTS_DIR = "experiments"


# ================================================================
# Helpers
# ================================================================

def safe_mkdir(path):
    if not os.path.exists(path):
        os.makedirs(path)


def load_results(suite):
    path = os.path.join(EXPERIMENTS_DIR, suite, "results.csv")
    if not os.path.isfile(path):
        print(f"[ERROR] results.csv not found: {path}")
        sys.exit(1)

    df = pd.read_csv(path)

    # Normalize convergence column
    if "convergence_sec" in df.columns:
        df["convergence"] = pd.to_numeric(df["convergence_sec"], errors="coerce")
    else:
        raise RuntimeError("results.csv missing convergence_sec")

    # extract delay as numeric
    if "delay" in df.columns:
        df["delay_ms"] = df["delay"].str.replace("ms", "", regex=False)
        df["delay_ms"] = pd.to_numeric(df["delay_ms"], errors="coerce")

    df["loss_pct"] = pd.to_numeric(df["loss"], errors="coerce")
    df["nodes_delivered"] = pd.to_numeric(df["nodes_delivered"], errors="coerce")

    return df


def detect_varying(df):
    sweep = []
    for col in ["hosts", "fanout", "loss_pct", "delay_ms"]:
        if col in df and df[col].nunique() > 1:
            sweep.append(col)
    return sweep


# ================================================================
# Plot types
# ================================================================

def plot_ranked(df, outdir):
    sub = df.dropna(subset=["convergence"]).sort_values("convergence")

    plt.figure(figsize=(12, 5))
    sns.barplot(data=sub, x="name", y="convergence", palette="Blues_d")
    plt.xticks(rotation=45, ha="right")
    plt.title("Ranked Convergence Times (Fast → Slow)")
    plt.tight_layout()

    path = os.path.join(outdir, "ranked_convergence.png")
    plt.savefig(path)
    plt.close()
    print(f"[PLOT] {path}")


def plot_distribution(df, outdir):
    sub = df.dropna(subset=["convergence"])

    # Histogram + KDE
    plt.figure(figsize=(10, 4))
    sns.histplot(sub["convergence"], kde=True, bins=10, color="steelblue")
    plt.title("Distribution of Convergence Times")
    plt.xlabel("Convergence (sec)")
    plt.tight_layout()
    path = os.path.join(outdir, "distribution.png")
    plt.savefig(path)
    plt.close()
    print(f"[PLOT] {path}")

    # CDF
    sorted_vals = sub["convergence"].sort_values()
    y = [i / len(sorted_vals) for i in range(len(sorted_vals))]

    plt.figure(figsize=(10, 4))
    plt.plot(sorted_vals, y)
    plt.title("CDF of Convergence Times")
    plt.xlabel("Convergence (sec)")
    plt.ylabel("Probability")
    plt.grid(True)
    plt.tight_layout()
    path = os.path.join(outdir, "cdf.png")
    plt.savefig(path)
    plt.close()
    print(f"[PLOT] {path}")


def plot_correlation(df, outdir):
    numeric = df.select_dtypes(include=["number"])
    corr = numeric.corr()

    plt.figure(figsize=(8, 6))
    sns.heatmap(corr, annot=True, cmap="coolwarm", fmt=".2f")
    plt.title("Correlation Between Parameters")
    plt.tight_layout()

    path = os.path.join(outdir, "correlation_heatmap.png")
    plt.savefig(path)
    plt.close()
    print(f"[PLOT] {path}")


def plot_nodes_vs_convergence(df, outdir):
    sub = df.dropna(subset=["nodes_delivered", "convergence"])

    plt.figure(figsize=(6, 5))
    sns.scatterplot(data=sub, x="nodes_delivered", y="convergence", s=100)
    plt.title("Nodes Delivered vs Convergence Time")
    plt.grid(True)
    plt.tight_layout()

    path = os.path.join(outdir, "nodes_vs_convergence.png")
    plt.savefig(path)
    plt.close()
    print(f"[PLOT] {path}")


def plot_sweep(df, param, outdir):
    plt.figure(figsize=(8, 5))
    sns.lineplot(data=df, x=param, y="convergence", marker="o")
    sns.scatterplot(data=df, x=param, y="convergence", s=120)
    plt.title(f"{param} Sweep")
    plt.tight_layout()

    path = os.path.join(outdir, f"{param}_sweep.png")
    plt.savefig(path)
    plt.close()
    print(f"[PLOT] {path}")


# ================================================================
# Stats tables
# ================================================================

def table_stats(df, outdir):
    sub = df.dropna(subset=["convergence"])
    stats = sub["convergence"].describe()

    out = os.path.join(outdir, "convergence_stats.csv")
    stats.to_csv(out)
    print(f"[TABLE] {out}")


def table_ranking(df, outdir):
    sub = df[["name", "convergence", "nodes_delivered"]].dropna()
    sub = sub.sort_values("convergence")

    out = os.path.join(outdir, "ranking_table.csv")
    sub.to_csv(out, index=False)
    print(f"[TABLE] {out}")


# ================================================================
# Main plotting
# ================================================================

def plot_suite(df, suite_dir):
    outdir = os.path.join(suite_dir, "plots")
    safe_mkdir(outdir)

    varying = detect_varying(df)

    # Sweep plots
    for p in varying:
        plot_sweep(df.dropna(subset=["convergence"]), p, outdir)

    # Universal plots
    plot_ranked(df, outdir)
    plot_distribution(df, outdir)
    plot_correlation(df, outdir)
    plot_nodes_vs_convergence(df, outdir)

    # Tables
    table_stats(df, outdir)
    table_ranking(df, outdir)

    print(f"[INFO] All plots saved in {outdir}")


# ================================================================
# Entry point
# ================================================================

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--suite", required=True)
    args = ap.parse_args()

    df = load_results(args.suite)
    print(f"[INFO] Loaded {len(df)} experiments from suite: {args.suite}")

    suite_dir = os.path.join(EXPERIMENTS_DIR, args.suite)
    plot_suite(df, suite_dir)


if __name__ == "__main__":
    main()
