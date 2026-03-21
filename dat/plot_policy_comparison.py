#!/usr/bin/env python3
"""
Plot policy comparison: run all 4 prescribing policies and compare resistance
trajectories and BSI outcomes.

Usage:
  # First run each policy simulation (or use run_all below):
  go run github.com/umbralcalc/stochadex/cmd/stochadex --config cfg/amr_policy_baseline.yaml
  go run github.com/umbralcalc/stochadex/cmd/stochadex --config cfg/amr_policy_cycling.yaml
  go run github.com/umbralcalc/stochadex/cmd/stochadex --config cfg/amr_policy_threshold.yaml
  go run github.com/umbralcalc/stochadex/cmd/stochadex --config cfg/amr_policy_restriction.yaml

  # Then plot:
  python3 dat/plot_policy_comparison.py
"""

import json
import os
import subprocess
import sys
import numpy as np
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt

DAT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_DIR = os.path.dirname(DAT_DIR)

POLICIES = {
    "baseline": {
        "config": "cfg/amr_policy_baseline.yaml",
        "log": "dat/policy_baseline_output.log",
        "color": "#999999",
        "label": "Baseline (constant 0.3)",
    },
    "cycling": {
        "config": "cfg/amr_policy_cycling.yaml",
        "log": "dat/policy_cycling_output.log",
        "color": "#2166ac",
        "label": "Cycling (quarterly 0.3/0.05)",
    },
    "threshold": {
        "config": "cfg/amr_policy_threshold.yaml",
        "log": "dat/policy_threshold_output.log",
        "color": "#b2182b",
        "label": "Threshold (switch at R>15%)",
    },
    "restriction": {
        "config": "cfg/amr_policy_restriction.yaml",
        "log": "dat/policy_restriction_output.log",
        "color": "#1b7837",
        "label": "Restriction (ramp to 0.1)",
    },
}


def run_all():
    """Run all policy simulations via the stochadex CLI."""
    for name, info in POLICIES.items():
        config = os.path.join(PROJECT_DIR, info["config"])
        print(f"Running {name} policy: {config}")
        result = subprocess.run(
            ["go", "run",
             "github.com/umbralcalc/stochadex/cmd/stochadex",
             "--config", config],
            cwd=PROJECT_DIR,
            capture_output=True, text=True,
        )
        if result.returncode != 0:
            print(f"  ERROR: {result.stderr[:500]}", file=sys.stderr)
        else:
            print(f"  Done.")


def load_log(path):
    """Parse a JSON log file into per-partition time series."""
    colonisation = []
    infection = []
    prescribing = []
    with open(path) as f:
        for line in f:
            record = json.loads(line)
            name = record["partition_name"]
            t = record["time"]
            state = record["state"]
            if name == "colonisation":
                colonisation.append((t, state[0], state[1]))
            elif name == "infection":
                infection.append((t, state[0], state[1]))
            elif name == "prescribing":
                prescribing.append((t, state[0]))
    return colonisation, infection, prescribing


def main():
    # Run simulations if logs don't exist
    any_missing = any(
        not os.path.exists(os.path.join(PROJECT_DIR, info["log"]))
        for info in POLICIES.values()
    )
    if any_missing:
        run_all()

    fig, axes = plt.subplots(3, 1, figsize=(14, 12), sharex=True)

    summary = {}

    for name, info in POLICIES.items():
        log_path = os.path.join(PROJECT_DIR, info["log"])
        if not os.path.exists(log_path):
            print(f"Warning: {log_path} not found, skipping {name}")
            continue

        colonisation, infection, prescribing = load_log(log_path)
        color = info["color"]
        label = info["label"]

        # Extract arrays
        times_col = [c[0] for c in colonisation]
        s_frac = [c[1] for c in colonisation]
        r_frac = [c[2] for c in colonisation]
        r_ratio = [r / (s + r) if (s + r) > 0 else 0
                    for s, r in zip(s_frac, r_frac)]

        times_inf = [i[0] for i in infection]
        bsi_r = [i[2] for i in infection]

        times_presc = [p[0] for p in prescribing]
        presc_rate = [p[1] for p in prescribing]

        # Panel 1: Prescribing rate over time
        axes[0].plot(times_presc, presc_rate, color=color, linewidth=1.5,
                     label=label)

        # Panel 2: Resistance ratio over time
        axes[1].plot(times_col, r_ratio, color=color, linewidth=1.5,
                     label=label)

        # Panel 3: Cumulative resistant BSI
        cum_bsi_r = np.cumsum(bsi_r)
        axes[2].plot(times_inf, cum_bsi_r, color=color, linewidth=1.5,
                     label=label)

        # Summary stats at year 1 (step ~52), 3 (~156), 5 (~200)
        summary[name] = {
            "final_r_ratio": r_ratio[-1] if r_ratio else None,
            "total_resistant_bsi": cum_bsi_r[-1] if len(cum_bsi_r) > 0 else None,
        }

    # Format panel 1
    axes[0].set_ylabel("Cephalosporin prescribing rate")
    axes[0].set_title("Prescribing policy comparison: cephalosporin rate")
    axes[0].legend(loc="upper right", fontsize=9)
    axes[0].set_ylim(-0.02, 0.42)

    # Format panel 2
    axes[1].set_ylabel("Resistant / (Susceptible + Resistant)")
    axes[1].set_title("Resistance ratio over time")
    axes[1].axhline(y=0.15, color="grey", linestyle=":", alpha=0.5,
                    label="15% threshold")
    axes[1].legend(loc="upper right", fontsize=9)
    axes[1].set_ylim(-0.02, 1.02)

    # Format panel 3
    axes[2].set_ylabel("Cumulative resistant BSI count")
    axes[2].set_xlabel("Time step")
    axes[2].set_title("Cumulative resistant bloodstream infections")
    axes[2].legend(loc="upper left", fontsize=9)

    fig.tight_layout()
    out = os.path.join(DAT_DIR, "plot_policy_comparison.png")
    fig.savefig(out, dpi=150)
    plt.close(fig)
    print(f"Saved {out}")

    # Print summary
    print("\n--- Policy Summary ---")
    for name, stats in summary.items():
        label = POLICIES[name]["label"]
        print(f"  {label}:")
        print(f"    Final resistance ratio: {stats['final_r_ratio']:.3f}"
              if stats['final_r_ratio'] is not None else "    (no data)")
        print(f"    Total resistant BSIs:   {stats['total_resistant_bsi']:.0f}"
              if stats['total_resistant_bsi'] is not None else "    (no data)")


if __name__ == "__main__":
    main()
