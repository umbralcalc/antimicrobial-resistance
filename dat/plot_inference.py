#!/usr/bin/env python3
"""
Plot simulation-based inference results.

Reads dat/inference_output.log and produces:
  - dat/plot_posterior_convergence.png: parameter posterior mean convergence
  - dat/plot_posterior_covariance.png: final posterior covariance matrix

Usage: python3 dat/plot_inference.py
"""

import json
import os
import numpy as np
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt

DAT_DIR = os.path.dirname(os.path.abspath(__file__))
PARAM_NAMES = [
    "transmission_rate",
    "selection_coefficient",
    "fitness_cost",
    "community_resistant_prevalence",
]


def load_inference_log():
    means = []
    covs = []
    with open(os.path.join(DAT_DIR, "inference_output.log")) as f:
        for line in f:
            record = json.loads(line)
            if record["partition_name"] == "params_posterior_mean":
                means.append((record["time"], record["state"]))
            elif record["partition_name"] == "params_posterior_cov":
                covs.append((record["time"], record["state"]))
    return means, covs


def plot_convergence(means):
    times = [t for t, _ in means]
    values = np.array([s for _, s in means])

    fig, axes = plt.subplots(2, 2, figsize=(12, 8), sharex=True)
    colors = ["#2166ac", "#b2182b", "#4daf4a", "#984ea3"]

    for i, (ax, name, color) in enumerate(
        zip(axes.flat, PARAM_NAMES, colors)
    ):
        ax.plot(times, values[:, i], color=color, linewidth=1.5)
        ax.set_ylabel(name)
        ax.axhline(y=values[-1, i], color="grey", linestyle=":", alpha=0.5)
        ax.text(
            0.95, 0.95, f"final = {values[-1, i]:.4f}",
            transform=ax.transAxes, fontsize=10,
            verticalalignment="top", horizontalalignment="right",
            bbox=dict(boxstyle="round", facecolor="wheat", alpha=0.5),
        )

    axes[1, 0].set_xlabel("Inference step")
    axes[1, 1].set_xlabel("Inference step")
    fig.suptitle("Posterior mean convergence", fontsize=14)
    fig.tight_layout()

    out = os.path.join(DAT_DIR, "plot_posterior_convergence.png")
    fig.savefig(out, dpi=150)
    plt.close(fig)
    print(f"  Saved {out}")


def plot_covariance(covs):
    # Final covariance matrix (4x4 = 16 values flattened)
    final_cov = np.array(covs[-1][1]).reshape(4, 4)

    # Convert to correlation matrix
    stds = np.sqrt(np.diag(final_cov))
    corr = final_cov / np.outer(stds, stds)
    np.fill_diagonal(corr, 1.0)

    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(12, 5))

    # Covariance
    im1 = ax1.imshow(final_cov, cmap="RdBu_r", aspect="auto")
    ax1.set_xticks(range(4))
    ax1.set_yticks(range(4))
    short_names = ["trans", "select", "fitness", "comm_R"]
    ax1.set_xticklabels(short_names, rotation=45, ha="right")
    ax1.set_yticklabels(short_names)
    ax1.set_title("Posterior covariance")
    for i in range(4):
        for j in range(4):
            ax1.text(j, i, f"{final_cov[i,j]:.2e}", ha="center", va="center", fontsize=8)
    fig.colorbar(im1, ax=ax1, shrink=0.8)

    # Correlation
    im2 = ax2.imshow(corr, cmap="RdBu_r", vmin=-1, vmax=1, aspect="auto")
    ax2.set_xticks(range(4))
    ax2.set_yticks(range(4))
    ax2.set_xticklabels(short_names, rotation=45, ha="right")
    ax2.set_yticklabels(short_names)
    ax2.set_title("Posterior correlation")
    for i in range(4):
        for j in range(4):
            ax2.text(j, i, f"{corr[i,j]:.2f}", ha="center", va="center", fontsize=10)
    fig.colorbar(im2, ax=ax2, shrink=0.8)

    fig.suptitle("Final posterior parameter estimates", fontsize=14)
    fig.tight_layout()

    out = os.path.join(DAT_DIR, "plot_posterior_covariance.png")
    fig.savefig(out, dpi=150)
    plt.close(fig)
    print(f"  Saved {out}")

    # Print summary
    print("\n  Final posterior means and standard deviations:")
    for i, name in enumerate(PARAM_NAMES):
        print(f"    {name}: {np.sqrt(final_cov[i,i]):.4f} (std)")


if __name__ == "__main__":
    print("Plotting inference results...")
    means, covs = load_inference_log()
    plot_convergence(means)
    plot_covariance(covs)
    print("\nDone.")
