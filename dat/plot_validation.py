#!/usr/bin/env python3
"""
Plot validation: simulated resistance trajectories vs observed England data.

Usage: python3 dat/plot_validation.py
"""

import os
import re
import pandas as pd
import numpy as np
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt

DAT_DIR = os.path.dirname(os.path.abspath(__file__))


def parse_colonisation(path):
    steps, s_vals, r_vals = [], [], []
    with open(path) as f:
        for line in f:
            m = re.match(r"(\d+)\s+colonisation\s+\[([^\]]+)\]", line.strip())
            if m:
                step = int(m.group(1))
                vals = [float(x) for x in m.group(2).split()]
                steps.append(step)
                s_vals.append(vals[0])
                r_vals.append(vals[1])
    return steps, s_vals, r_vals


def parse_quarter(period_str):
    parts = period_str.strip().split()
    if len(parts) == 2 and parts[1].startswith("Q"):
        year = int(parts[0])
        quarter = int(parts[1][1])
        month = (quarter - 1) * 3 + 1
        return pd.Timestamp(year=year, month=month, day=1)
    return pd.NaT


def main():
    # Load observed England data
    baseline = pd.read_csv(os.path.join(DAT_DIR, "baseline_england.csv"))
    baseline = baseline.sort_values("step")
    obs_steps = baseline["step"].values
    obs_resistance = baseline["resistance_fraction"].values

    # Parse quarter labels for x-axis
    baseline["Date"] = baseline["Time period"].apply(parse_quarter)

    # Load simulation replicates
    seeds = [9182, 1234, 5678, 4321, 8765]
    replicates = []
    for seed in seeds:
        path = f"/tmp/val_{seed}.txt"
        if os.path.exists(path):
            steps, _, r_vals = parse_colonisation(path)
            replicates.append((steps, r_vals))

    fig, ax = plt.subplots(figsize=(12, 6))

    # Plot observed
    ax.plot(obs_steps, obs_resistance, "k-", linewidth=2.5, label="Observed (England)",
            zorder=10)

    # Plot replicates
    for i, (steps, r_vals) in enumerate(replicates):
        label = "Simulated (learned params)" if i == 0 else None
        ax.plot(steps, r_vals, color="#b2182b", alpha=0.4, linewidth=1, label=label)

    # Compute and plot envelope
    if replicates:
        max_len = min(len(r) for _, r in replicates)
        all_r = np.array([r[:max_len] for _, r in replicates])
        mean_r = np.mean(all_r, axis=0)
        std_r = np.std(all_r, axis=0)
        steps_arr = np.array(replicates[0][0][:max_len])
        ax.fill_between(steps_arr, mean_r - 2*std_r, mean_r + 2*std_r,
                        color="#b2182b", alpha=0.15, label="Simulated ±2σ")
        ax.plot(steps_arr, mean_r, color="#b2182b", linewidth=2, linestyle="--",
                label="Simulated mean")

    # Add quarter labels on x-axis (every 4th quarter)
    tick_positions = obs_steps[::4]
    tick_labels = [baseline["Time period"].iloc[i] for i in range(0, len(baseline), 4)]
    ax.set_xticks(tick_positions)
    ax.set_xticklabels(tick_labels, rotation=45, ha="right", fontsize=9)

    ax.set_xlabel("Quarter")
    ax.set_ylabel("E. coli 3GC resistance fraction")
    ax.set_title("Validation: simulated resistance vs observed England data (learned parameters)")
    ax.legend(loc="upper left")
    fig.tight_layout()

    out = os.path.join(DAT_DIR, "plot_validation.png")
    fig.savefig(out, dpi=150)
    plt.close(fig)
    print(f"Saved {out}")


if __name__ == "__main__":
    main()
