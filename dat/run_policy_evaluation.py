#!/usr/bin/env python3
"""
Run multiple stochastic trajectories for each prescribing policy.

For each policy config, runs N_SEEDS simulations with different RNG seeds
for the colonisation and infection partitions. Outputs are saved as:
    dat/policy_{name}_seed{i}.log

Usage: python3 dat/run_policy_evaluation.py
"""

import os
import subprocess
import tempfile
import yaml

DAT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_DIR = os.path.dirname(DAT_DIR)

POLICIES = {
    "baseline": "cfg/amr_policy_baseline.yaml",
    "cycling": "cfg/amr_policy_cycling.yaml",
    "threshold": "cfg/amr_policy_threshold.yaml",
    "restriction": "cfg/amr_policy_restriction.yaml",
}

SEEDS = [
    (9182, 3347),
    (1234, 5678),
    (4321, 8765),
    (7777, 2222),
    (5555, 1111),
    (3333, 9999),
    (6666, 4444),
    (8888, 6543),
    (2468, 1357),
    (9753, 8642),
]


def run_policy(name, config_path, seed_index, col_seed, inf_seed):
    """Run a single policy simulation with the given seeds."""
    config_abs = os.path.join(PROJECT_DIR, config_path)
    with open(config_abs) as f:
        cfg = yaml.safe_load(f)

    # Update seeds for stochastic partitions
    for partition in cfg["main"]["partitions"]:
        if partition["name"] == "colonisation":
            partition["seed"] = col_seed
        elif partition["name"] == "infection":
            partition["seed"] = inf_seed

    # Update output path
    out_log = f"./dat/policy_{name}_seed{seed_index}.log"
    out_fn = cfg["main"]["simulation"]["output_function"]
    cfg["main"]["simulation"]["output_function"] = (
        f'simulator.NewJsonLogOutputFunction("{out_log}")'
    )

    # Write temp config and run
    with tempfile.NamedTemporaryFile(
        mode="w", suffix=".yaml", dir=PROJECT_DIR, delete=False
    ) as tmp:
        yaml.dump(cfg, tmp, default_flow_style=False)
        tmp_path = tmp.name

    try:
        result = subprocess.run(
            [
                "go", "run",
                "github.com/umbralcalc/stochadex/cmd/stochadex",
                "--config", tmp_path,
            ],
            cwd=PROJECT_DIR,
            capture_output=True,
            text=True,
            env={**os.environ, "GOFLAGS": "-mod=mod"},
        )
        if result.returncode != 0:
            print(f"  ERROR: {name} seed{seed_index}: {result.stderr[:200]}")
            return False
        return True
    finally:
        os.unlink(tmp_path)


def main():
    n_seeds = len(SEEDS)
    total = len(POLICIES) * n_seeds
    done = 0

    print(f"Running {len(POLICIES)} policies x {n_seeds} seeds = {total} simulations")
    print()

    for name, config_path in POLICIES.items():
        print(f"Policy: {name}")
        for i, (col_seed, inf_seed) in enumerate(SEEDS):
            ok = run_policy(name, config_path, i, col_seed, inf_seed)
            done += 1
            status = "OK" if ok else "FAILED"
            print(f"  seed{i} ({col_seed}, {inf_seed}): {status}  [{done}/{total}]")
        print()

    print("Done.")


if __name__ == "__main__":
    main()
