#!/usr/bin/env python3
"""
Prepare data files for stochadex simulation-based inference.

Produces:
  - dat/sbi_prescribing_input.json: prescribing time series for FromStorageIteration
  - dat/sbi_observed_resistance.json: observed resistance for DataComparisonIteration

Usage: python3 dat/prepare_sbi_data.py
"""

import json
import os
import pandas as pd

DAT_DIR = os.path.dirname(os.path.abspath(__file__))


def main():
    baseline = pd.read_csv(os.path.join(DAT_DIR, "baseline_england.csv"))
    baseline = baseline.sort_values("step")

    # Prescribing input: each step maps to [broadspectrum_fraction]
    # The FromStorageIteration reads these as sequential state values
    prescribing_data = []
    for _, row in baseline.iterrows():
        prescribing_data.append([row["broadspectrum_fraction"]])

    out = os.path.join(DAT_DIR, "sbi_prescribing_input.json")
    with open(out, "w") as f:
        json.dump(prescribing_data, f, indent=2)
    print(f"  Prescribing input: {len(prescribing_data)} steps -> {out}")

    # Observed resistance: each step maps to [resistance_fraction]
    # Used as latest_data_values for DataComparisonIteration
    resistance_data = []
    for _, row in baseline.iterrows():
        resistance_data.append([row["resistance_fraction"]])

    out = os.path.join(DAT_DIR, "sbi_observed_resistance.json")
    with open(out, "w") as f:
        json.dump(resistance_data, f, indent=2)
    print(f"  Observed resistance: {len(resistance_data)} steps -> {out}")

    # Print summary for config setup
    print(f"\n  Initial prescribing rate: {prescribing_data[0][0]:.4f}")
    print(f"  Initial resistance fraction: {resistance_data[0][0]:.4f}")
    print(f"  Final resistance fraction: {resistance_data[-1][0]:.4f}")
    print(f"  Number of timesteps: {len(prescribing_data)}")


if __name__ == "__main__":
    print("Preparing SBI data files...")
    main()
    print("\nDone.")
