#!/usr/bin/env python3
"""
Prepare data files for stochadex simulation-based inference.

Produces:
  - dat/sbi_prescribing_input.json: prescribing time series (kept for reference/tooling)
  - dat/sbi_observed_resistance.json: observed resistance (kept for reference/tooling)
  - dat/sbi_inference_series.csv: the joined series the pure-data cfg/amr_inference.yaml
    loads via its `data: source: csv` tier. Columns: time,resistance,prescribing. The 40
    quarterly observations are cycled to INFERENCE_STEPS rows so the posterior has enough
    steps to converge (the old Go DataReplayIteration cycled the series at run time; the
    data: tier loads a fixed table, so the cycling is baked in here).

Usage: python3 dat/prepare_sbi_data.py
"""

import csv
import json
import os
import pandas as pd

DAT_DIR = os.path.dirname(os.path.abspath(__file__))

# The horizon cfg/amr_inference.yaml runs the posterior loop over. The 40-row baseline is
# cycled to this many rows; keep this in sync with the config's data CSV length.
INFERENCE_STEPS = 600


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

    # Joined series for the pure-data config's `data: source: csv` tier. Cycle the 40-row
    # baseline to INFERENCE_STEPS rows (row i uses baseline row i % len), one column each for
    # the observed resistance target and the time-varying prescribing input.
    n = len(resistance_data)
    out = os.path.join(DAT_DIR, "sbi_inference_series.csv")
    with open(out, "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["time", "resistance", "prescribing"])
        for i in range(INFERENCE_STEPS):
            w.writerow([i, resistance_data[i % n][0], prescribing_data[i % n][0]])
    print(f"  Inference series: {INFERENCE_STEPS} rows -> {out}")

    # Print summary for config setup
    print(f"\n  Initial prescribing rate: {prescribing_data[0][0]:.4f}")
    print(f"  Initial resistance fraction: {resistance_data[0][0]:.4f}")
    print(f"  Final resistance fraction: {resistance_data[-1][0]:.4f}")
    print(f"  Number of timesteps: {len(prescribing_data)}")


if __name__ == "__main__":
    print("Preparing SBI data files...")
    main()
    print("\nDone.")
