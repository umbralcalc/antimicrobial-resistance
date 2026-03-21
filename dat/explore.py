#!/usr/bin/env python3
"""
Exploratory analysis: prescribing pressure vs E. coli cephalosporin resistance.

Produces three plots in dat/:
  1. England-level time series of broad-spectrum prescribing % and resistance %
  2. Scatter of ICB-level prescribing % vs resistance % (cross-sectional)
  3. E. coli bacteraemia rates over time for selected acute trusts

Usage: python3 dat/explore.py
"""

import os
import pandas as pd
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np

DAT_DIR = os.path.dirname(os.path.abspath(__file__))


def load_fingertips(name):
    path = os.path.join(DAT_DIR, f"fingertips_{name}.csv")
    df = pd.read_csv(path)
    df["Value"] = pd.to_numeric(df["Value"], errors="coerce")
    df["Count"] = pd.to_numeric(df["Count"], errors="coerce")
    df["Denominator"] = pd.to_numeric(df["Denominator"], errors="coerce")
    return df


def parse_quarter(period_str):
    """Convert '2019 Q1' to a datetime."""
    parts = period_str.strip().split()
    if len(parts) == 2 and parts[1].startswith("Q"):
        year = int(parts[0])
        quarter = int(parts[1][1])
        month = (quarter - 1) * 3 + 1
        return pd.Timestamp(year=year, month=month, day=1)
    return pd.NaT


# ── Plot 1: England-level time series ──────────────────────────────────────

def plot_england_timeseries():
    prescribing = load_fingertips("broadspectrum_pct")
    resistance = load_fingertips("ecoli_cephalosporin_resistance_pct")

    # England-level rows only
    eng_presc = prescribing[prescribing["Area Type"] == "England"].copy()
    eng_resist = resistance[resistance["Area Type"] == "England"].copy()

    eng_presc["Date"] = eng_presc["Time period"].apply(parse_quarter)
    eng_resist["Date"] = eng_resist["Time period"].apply(parse_quarter)

    eng_presc = eng_presc.dropna(subset=["Date", "Value"]).sort_values("Date")
    eng_resist = eng_resist.dropna(subset=["Date", "Value"]).sort_values("Date")

    fig, ax1 = plt.subplots(figsize=(12, 5))

    color1 = "#2166ac"
    color2 = "#b2182b"

    ax1.set_xlabel("Date")
    ax1.set_ylabel("Broad-spectrum prescribing (%)", color=color1)
    ax1.plot(eng_presc["Date"], eng_presc["Value"], color=color1, linewidth=2,
             label="Broad-spectrum %")
    ax1.tick_params(axis="y", labelcolor=color1)

    ax2 = ax1.twinx()
    ax2.set_ylabel("E. coli 3GC resistance (%)", color=color2)
    ax2.plot(eng_resist["Date"], eng_resist["Value"], color=color2, linewidth=2,
             label="3GC resistance %")
    ax2.tick_params(axis="y", labelcolor=color2)

    fig.suptitle(
        "England: Broad-spectrum antibiotic prescribing vs E. coli 3rd-gen cephalosporin resistance",
        fontsize=12,
    )
    fig.tight_layout()

    out = os.path.join(DAT_DIR, "plot_england_timeseries.png")
    fig.savefig(out, dpi=150)
    plt.close(fig)
    print(f"  Saved {out}")


# ── Plot 2: ICB-level cross-sectional scatter ──────────────────────────────

def plot_icb_scatter():
    prescribing = load_fingertips("broadspectrum_pct")
    resistance = load_fingertips("ecoli_cephalosporin_resistance_pct")

    # ICB sub-location rows, most recent common quarter
    icb_presc = prescribing[prescribing["Area Type"] == "ICB sub-locations"].copy()
    icb_resist = resistance[resistance["Area Type"] == "ICB sub-locations"].copy()

    # Find common time periods
    common_periods = set(icb_presc["Time period"]) & set(icb_resist["Time period"])
    latest = sorted(common_periods)[-1]
    print(f"  Cross-sectional scatter using period: {latest}")

    presc_latest = icb_presc[icb_presc["Time period"] == latest][
        ["Area Code", "Area Name", "Value"]
    ].rename(columns={"Value": "broadspectrum_pct"})

    resist_latest = icb_resist[icb_resist["Time period"] == latest][
        ["Area Code", "Value"]
    ].rename(columns={"Value": "resistance_pct"})

    merged = presc_latest.merge(resist_latest, on="Area Code").dropna()

    fig, ax = plt.subplots(figsize=(8, 6))
    ax.scatter(merged["broadspectrum_pct"], merged["resistance_pct"],
               alpha=0.6, edgecolors="k", linewidth=0.5, s=50)

    # Fit and plot trend line
    if len(merged) > 2:
        z = np.polyfit(merged["broadspectrum_pct"], merged["resistance_pct"], 1)
        p = np.poly1d(z)
        x_range = np.linspace(merged["broadspectrum_pct"].min(),
                              merged["broadspectrum_pct"].max(), 50)
        ax.plot(x_range, p(x_range), "r--", alpha=0.7, linewidth=1.5)

        corr = merged["broadspectrum_pct"].corr(merged["resistance_pct"])
        ax.text(0.05, 0.95, f"r = {corr:.2f} (n={len(merged)})",
                transform=ax.transAxes, fontsize=11, verticalalignment="top",
                bbox=dict(boxstyle="round", facecolor="wheat", alpha=0.5))

    ax.set_xlabel("Broad-spectrum prescribing (%)")
    ax.set_ylabel("E. coli 3GC resistance (%)")
    ax.set_title(f"ICB sub-locations: prescribing vs resistance ({latest})")
    fig.tight_layout()

    out = os.path.join(DAT_DIR, "plot_icb_scatter.png")
    fig.savefig(out, dpi=150)
    plt.close(fig)
    print(f"  Saved {out}")


# ── Plot 3: E. coli bacteraemia rates for selected trusts ─────────────────

def plot_trust_bacteraemia():
    bact = load_fingertips("ecoli_bacteraemia_rolling")

    trusts = bact[bact["Area Type"] == "Acute Trust"].copy()

    # Pick 10 large trusts with the most data points
    trust_counts = trusts.groupby("Area Name")["Value"].count()
    top_trusts = trust_counts.nlargest(10).index.tolist()

    trusts = trusts[trusts["Area Name"].isin(top_trusts)].copy()
    trusts["Date"] = trusts["Time period"].apply(
        lambda x: pd.to_datetime(x, format="%B %Y", errors="coerce")
    )
    trusts = trusts.dropna(subset=["Date", "Value"])

    fig, ax = plt.subplots(figsize=(12, 6))
    for name, group in trusts.groupby("Area Name"):
        group = group.sort_values("Date")
        # Shorten trust name for legend
        short = name.replace("NHS Foundation Trust", "").replace("NHS Trust", "").strip()
        ax.plot(group["Date"], group["Value"], alpha=0.7, linewidth=1.2, label=short)

    ax.set_xlabel("Date")
    ax.set_ylabel("E. coli bacteraemia rate (per 100k)")
    ax.set_title("12-month rolling E. coli bacteraemia rates — selected acute trusts")
    ax.legend(fontsize=7, loc="upper left", ncol=2)
    fig.tight_layout()

    out = os.path.join(DAT_DIR, "plot_trust_bacteraemia.png")
    fig.savefig(out, dpi=150)
    plt.close(fig)
    print(f"  Saved {out}")


if __name__ == "__main__":
    print("Running exploratory analysis...")
    plot_england_timeseries()
    plot_icb_scatter()
    plot_trust_bacteraemia()
    print("Done.")
