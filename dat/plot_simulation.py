#!/usr/bin/env python3
"""
Plot simulation output to verify qualitative dynamics.

Reads dat/simulation_output.txt (produced by running the stochadex simulation
with StdoutOutputFunction) and produces dat/plot_simulation.png.

Usage: python3 dat/plot_simulation.py
"""

import os
import re
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt

DAT_DIR = os.path.dirname(os.path.abspath(__file__))


def parse_output(path):
    """Parse stdout lines like '5 colonisation [0.138 0.094]'."""
    data = {"colonisation": {}, "infection": {}, "prescribing": {}}
    with open(path) as f:
        for line in f:
            m = re.match(r"(\d+)\s+(\w+)\s+\[([^\]]+)\]", line.strip())
            if not m:
                continue
            step = int(m.group(1))
            partition = m.group(2)
            values = [float(x) for x in m.group(3).split()]
            if partition in data:
                data[partition][step] = values
    return data


def main():
    data = parse_output(os.path.join(DAT_DIR, "simulation_output.txt"))

    steps_col = sorted(data["colonisation"].keys())
    steps_inf = sorted(data["infection"].keys())

    s_frac = [data["colonisation"][t][0] for t in steps_col]
    r_frac = [data["colonisation"][t][1] for t in steps_col]

    bsi_s = [data["infection"][t][0] for t in steps_inf]
    bsi_r = [data["infection"][t][1] for t in steps_inf]

    fig, axes = plt.subplots(3, 1, figsize=(12, 10), sharex=True)

    # Panel 1: Colonisation fractions
    ax = axes[0]
    ax.plot(steps_col, s_frac, label="Susceptible fraction", color="#2166ac", linewidth=1.5)
    ax.plot(steps_col, r_frac, label="Resistant fraction", color="#b2182b", linewidth=1.5)
    u_frac = [1.0 - s - r for s, r in zip(s_frac, r_frac)]
    ax.plot(steps_col, u_frac, label="Uncolonised fraction", color="#999999",
            linewidth=1, linestyle="--")
    ax.set_ylabel("Fraction of patients")
    ax.set_title("Colonisation dynamics (constant cephalosporin rate = 0.3)")
    ax.legend(loc="right")
    ax.set_ylim(-0.02, 1.02)

    # Panel 2: BSI counts
    ax = axes[1]
    ax.bar([t - 0.15 for t in steps_inf], bsi_s, width=0.3, label="Susceptible BSI",
           color="#2166ac", alpha=0.7)
    ax.bar([t + 0.15 for t in steps_inf], bsi_r, width=0.3, label="Resistant BSI",
           color="#b2182b", alpha=0.7)
    ax.set_ylabel("BSI count per timestep")
    ax.set_title("Bloodstream infection events")
    ax.legend(loc="upper right")

    # Panel 3: Resistance ratio
    ax = axes[2]
    r_ratio = [r / (s + r) if (s + r) > 0 else 0 for s, r in zip(s_frac, r_frac)]
    ax.plot(steps_col, r_ratio, color="#b2182b", linewidth=2)
    ax.set_ylabel("Resistant / (Susceptible + Resistant)")
    ax.set_xlabel("Time step (months)")
    ax.set_title("Resistance ratio over time")
    ax.set_ylim(-0.02, 1.02)
    ax.axhline(y=0.5, color="grey", linestyle=":", alpha=0.5)

    fig.tight_layout()
    out = os.path.join(DAT_DIR, "plot_simulation.png")
    fig.savefig(out, dpi=150)
    plt.close(fig)
    print(f"Saved {out}")


if __name__ == "__main__":
    main()
