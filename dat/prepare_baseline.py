#!/usr/bin/env python3
"""
Prepare baseline event rates from Fingertips data for simulation-based inference.

Produces dat/baseline_england.csv with quarterly time series of:
  - broadspectrum_pct: broad-spectrum prescribing percentage (input to simulation)
  - resistance_pct: E. coli 3GC resistance percentage (target for fitting)
  - ecoli_bsi_count: E. coli BSI specimen count (for scaling)
  - ecoli_bsi_denom: denominator (total specimens tested)

Also produces dat/baseline_icb_top10.csv with the same for the 10 ICB
sub-locations with the most complete data (for held-out validation).

Usage: python3 dat/prepare_baseline.py
"""

import os
import pandas as pd
import numpy as np

DAT_DIR = os.path.dirname(os.path.abspath(__file__))


def load(name):
    df = pd.read_csv(os.path.join(DAT_DIR, f"fingertips_{name}.csv"))
    df["Value"] = pd.to_numeric(df["Value"], errors="coerce")
    df["Count"] = pd.to_numeric(df["Count"], errors="coerce")
    df["Denominator"] = pd.to_numeric(df["Denominator"], errors="coerce")
    return df


def parse_quarter_sortable(sortable):
    """Convert sortable like 20190100 to '2019 Q1'."""
    s = str(int(sortable))
    year = s[:4]
    month = int(s[4:6])
    q = (month - 1) // 3 + 1
    return f"{year} Q{q}"


def prepare_england():
    """Merge prescribing and resistance at England level."""
    resist = load("ecoli_cephalosporin_resistance_pct")
    presc = load("broadspectrum_pct")

    eng_r = resist[resist["Area Type"] == "England"][
        ["Time period", "Time period Sortable", "Value", "Count", "Denominator"]
    ].rename(columns={"Value": "resistance_pct", "Count": "ecoli_bsi_count",
                       "Denominator": "ecoli_bsi_denom"})

    eng_p = presc[presc["Area Type"] == "England"][
        ["Time period", "Time period Sortable", "Value"]
    ].rename(columns={"Value": "broadspectrum_pct"})

    merged = eng_r.merge(eng_p, on=["Time period", "Time period Sortable"])
    merged = merged.sort_values("Time period Sortable").reset_index(drop=True)

    # Convert resistance from percentage to fraction for the simulation
    merged["resistance_fraction"] = merged["resistance_pct"] / 100.0
    merged["broadspectrum_fraction"] = merged["broadspectrum_pct"] / 100.0

    # Add a step index (0-based, each quarter = 1 timestep)
    merged["step"] = range(len(merged))

    out = os.path.join(DAT_DIR, "baseline_england.csv")
    merged.to_csv(out, index=False)
    print(f"  England baseline: {len(merged)} quarters -> {out}")
    print(f"    Time range: {merged['Time period'].iloc[0]} to {merged['Time period'].iloc[-1]}")
    print(f"    Resistance: {merged['resistance_fraction'].iloc[0]:.2f} -> {merged['resistance_fraction'].iloc[-1]:.2f}")
    print(f"    Prescribing: {merged['broadspectrum_fraction'].iloc[0]:.3f} -> {merged['broadspectrum_fraction'].iloc[-1]:.3f}")
    return merged


def prepare_icb_top10():
    """Prepare ICB sub-location level data for top 10 most complete areas."""
    resist = load("ecoli_cephalosporin_resistance_pct")
    presc = load("broadspectrum_pct")

    icb_r = resist[resist["Area Type"] == "ICB sub-locations"].copy()
    icb_p = presc[presc["Area Type"] == "ICB sub-locations"].copy()

    # Find ICBs with best data coverage (most non-null quarters in both datasets)
    r_counts = icb_r.dropna(subset=["Value"]).groupby("Area Code").size()
    p_counts = icb_p.dropna(subset=["Value"]).groupby("Area Code").size()
    combined = pd.DataFrame({"r": r_counts, "p": p_counts}).dropna()
    combined["total"] = combined["r"] + combined["p"]
    top10_codes = combined.nlargest(10, "total").index.tolist()

    # Merge for each ICB
    rows = []
    for code in top10_codes:
        r_sub = icb_r[icb_r["Area Code"] == code][
            ["Area Code", "Area Name", "Time period", "Time period Sortable",
             "Value", "Count", "Denominator"]
        ].rename(columns={"Value": "resistance_pct", "Count": "ecoli_bsi_count",
                           "Denominator": "ecoli_bsi_denom"})

        p_sub = icb_p[icb_p["Area Code"] == code][
            ["Area Code", "Time period", "Time period Sortable", "Value"]
        ].rename(columns={"Value": "broadspectrum_pct"})

        merged = r_sub.merge(p_sub, on=["Area Code", "Time period", "Time period Sortable"])
        merged = merged.sort_values("Time period Sortable")
        merged["resistance_fraction"] = merged["resistance_pct"] / 100.0
        merged["broadspectrum_fraction"] = merged["broadspectrum_pct"] / 100.0
        rows.append(merged)

    all_icb = pd.concat(rows, ignore_index=True)

    out = os.path.join(DAT_DIR, "baseline_icb_top10.csv")
    all_icb.to_csv(out, index=False)
    areas = all_icb["Area Name"].unique()
    print(f"  ICB top-10 baseline: {len(all_icb)} rows, {len(areas)} areas -> {out}")
    for a in areas:
        n = len(all_icb[all_icb["Area Name"] == a])
        print(f"    {a.strip()}: {n} quarters")

    return all_icb


if __name__ == "__main__":
    print("Preparing baseline event rates...")
    prepare_england()
    print()
    prepare_icb_top10()
    print("\nDone.")
