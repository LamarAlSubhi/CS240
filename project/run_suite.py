#!/usr/bin/env python3
import os, sys, subprocess, yaml, time, json, glob
from datetime import datetime

CONFIG_DIR = "configs"
EXPERIMENTS_DIR = "experiments"


# ============================================================
# Helpers
# ============================================================

def safe_mkdir(path):
    if not os.path.exists(path):
        os.makedirs(path)


def unique_suite_name(base):
    candidate = base
    counter = 2
    while os.path.exists(os.path.join(EXPERIMENTS_DIR, candidate)):
        candidate = f"{base}_{counter}"
        counter += 1
    return candidate


def run_cmd(cmd, cwd="."):
    """
    Run shell commands correctly.
    Strings => shell=True
    Lists   => normal safe mode
    """
    if isinstance(cmd, str):
        result = subprocess.run(
            cmd,
            cwd=cwd,
            shell=True,
            capture_output=True,
            text=True
        )
    else:
        result = subprocess.run(
            cmd,
            cwd=cwd,
            capture_output=True,
            text=True
        )

    if result.returncode != 0:
        print(f"[WARN] Command failed: {cmd}")
        print(result.stderr)

    return result.stdout, result.stderr


def load_suite_config(path):
    with open(path, "r") as f:
        return yaml.safe_load(f)


# ============================================================
# Running a single experiment
# ============================================================

def run_single_exp(exp_dir, params):

    print(f"\n========== Running: {exp_dir} ==========\n")

    # Fully merged parameters are guaranteed here
    # Required: hosts, bw, delay, loss, fanout, ttl, period, runtime, rumor_id

    # Clean Mininet
    run_cmd(["mn", "-c"], cwd=".")

    # Kill older gossip processes
    run_cmd("pkill gossipd || true", cwd=".")
    run_cmd("rm -f /tmp/gossip-*.jsonl", cwd=".")

    # Build topo command
    topo_cmd = [
        "python3", "topo.py",
        "--bin", "bin/linux/gossipd",
        "--hosts", str(params["hosts"]),
        "--bw", str(params["bw"]),        # FIXED
        "--delay", str(params["delay"]),
        "--loss", str(params["loss"]),
        "--fanout", str(params["fanout"]),
        "--ttl", str(params["ttl"]),
        "--period", str(params["period"]),
        "--metrics-port", "9080",
        "--logdir", "/tmp",
        "--no-cli",
        "--inject-rumor", params["rumor_id"],
        "--inject-ttl", str(params["ttl"]),
        "--runtime", str(params["runtime"])
    ]

    print("[INFO] Running topo.py...")
    out, err = run_cmd(topo_cmd, cwd=".")
    print(out)

    # Run analyzer
    print("[INFO] Running analyzer...")
    anal_cmd = ["python3", "analyze_logs.py", "--rumor-id", params["rumor_id"]]
    anal_out, anal_err = run_cmd(anal_cmd, cwd=".")

    print("\n----- analyzer stdout -----")
    print(anal_out)

    # Archive logs
    log_dest = os.path.join(exp_dir, "logs")
    safe_mkdir(log_dest)
    for lf in glob.glob("/tmp/gossip-*.jsonl"):
        dst = os.path.join(log_dest, os.path.basename(lf))
        print(f"[INFO] Archiving {lf} -> {dst}")
        os.rename(lf, dst)

    # Extract convergence
    conv = None
    for line in anal_out.splitlines():
        if "Convergence time" in line:
            try:
                conv = float(line.split()[-1])
            except:
                pass

    return conv


# ============================================================
# Parameter grid building
# ============================================================

def build_param_grid(defaults, run_cfg):
    """
    Build all experiments for this run block.
    Handles parameter sweeps and default merging.
    """

    grid = []

    # Fully merged defaults must convert "bandwidth" into "bw"
    # This is the field topo.py expects.
    merged_defaults = defaults.copy()

    # FIX: unify naming
    if "bandwidth" in merged_defaults:
        merged_defaults["bw"] = merged_defaults["bandwidth"]
    if "bw" not in merged_defaults and "bandwidth" not in merged_defaults:
        merged_defaults["bw"] = 10   # fallback default

    # Map sweep fields
    sweep_fields = ["hosts", "fanout", "loss", "delay", "rumor_count", "bw"]

    # Detect list-valued fields in run_cfg
    sweep_lists = {k: v for k, v in run_cfg.items() if isinstance(v, list)}

    # Base = defaults overridden by non-list fields in run_cfg
    base = merged_defaults.copy()
    for k, v in run_cfg.items():
        if not isinstance(v, list):
            base[k] = v

    # Cartesian expansion
    def expand(current, keys):
        if not keys:
            grid.append(current.copy())
            return
        k = keys[0]
        for v in sweep_lists[k]:
            current[k] = v
            expand(current, keys[1:])
        current.pop(k, None)

    if sweep_lists:
        expand(base.copy(), list(sweep_lists.keys()))
    else:
        grid.append(base)

    return grid


# ============================================================
# Main
# ============================================================

def main():
    if len(sys.argv) < 2:
        print("Usage: python3 run_suite.py configs/suite.yaml")
        sys.exit(1)

    cfg = load_suite_config(sys.argv[1])
    if cfg is None:
        print("[ERROR] YAML parsed as None. Fix indentation.")
        sys.exit(1)

    defaults = cfg.get("defaults", {})
    runs = cfg.get("runs", [])
    suite_name = cfg.get("suite_name", "suite")

    # Normalize defaults: rename bandwidth→bw
    if "bandwidth" in defaults:
        defaults["bw"] = defaults["bandwidth"]
    if "bw" not in defaults:
        defaults["bw"] = 10

    # Ensure TTL, period, runtime exist
    defaults.setdefault("ttl", 8)
    defaults.setdefault("period", "200ms")
    defaults.setdefault("runtime", 2.0)

    safe_mkdir(EXPERIMENTS_DIR)
    suite_name = unique_suite_name(suite_name)
    suite_dir = os.path.join(EXPERIMENTS_DIR, suite_name)
    safe_mkdir(suite_dir)

    print(f"[INFO] Suite directory: {suite_dir}")

    # Create results CSV
    results_csv = os.path.join(suite_dir, "results.csv")
    with open(results_csv, "w") as f:
        f.write("exp_name,repeat,hosts,fanout,loss,delay,rumor_count,bw,convergence\n")

    # Process each run block
    for run_block in runs:
        run_name = run_block["name"]
        print(f"\n-------- Run block: {run_name} --------\n")

        repeat = run_block.get("repeat", defaults.get("repeat", 1))

        # Build full param grid
        exp_grid = build_param_grid(defaults, run_block)

        block_dir = os.path.join(suite_dir, run_name)
        safe_mkdir(block_dir)

        for rep in range(repeat):
            for params in exp_grid:

                # Auto-build rumor id
                rumor_suffix = datetime.now().strftime("%H%M%S")
                params["rumor_id"] = f"{run_name}_{rumor_suffix}_rep{rep}"

                # Build experiment directory name
                exp_dir_name = f"{run_name}_r{rep}_h{params['hosts']}_f{params['fanout']}_l{params['loss']}"
                exp_dir = os.path.join(block_dir, exp_dir_name)
                safe_mkdir(exp_dir)

                conv = run_single_exp(exp_dir, params)

                # Write results
                with open(results_csv, "a") as f:
                    f.write(f"{run_name},{rep},{params['hosts']},{params['fanout']},{params['loss']},{params['delay']},{params.get('rumor_count','')},{params['bw']},{conv}\n")

    print("\n[INFO] All experiments done.")
    print("[INFO] Generating plots...")

    run_cmd(["python3", "plot_results.py", "--suite", suite_name])

    print("[INFO] Done.")


if __name__ == "__main__":
    main()
