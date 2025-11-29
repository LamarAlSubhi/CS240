#!/usr/bin/env bash
set -e

cd "$(dirname "$0")"

echo "[INFO] Running experiments..."
sudo python3 run_experiments.py "$@"

echo "[INFO] Fixing ownership on experiments/..."
sudo chown -R "$USER:$USER" experiments

echo "[INFO] Generating plots for latest suite..."
python3 plot_results.py

echo "[INFO] Done."
