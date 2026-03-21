# Antimicrobial Resistance Stewardship Simulation

A stochastic simulation of antimicrobial resistance (AMR) dynamics in English hospitals, fitted to UKHSA surveillance data via simulation-based inference, with a decision science layer that evaluates prescribing policies to reduce resistant bloodstream infections.

Built with the [stochadex](https://github.com/umbralcalc/stochadex) simulation engine.

---

## The Problem

Antibiotic-resistant infections are rising. In England, resistant infection rates increased 22.7% between FY 2019/20 and FY 2024/25. An estimated 35,000+ people die annually in the EU/EEA from antimicrobial-resistant infections. The EU has set 2030 targets for MRSA, cephalosporin-resistant *E. coli*, and carbapenem-resistant *K. pneumoniae* — but current trajectories suggest several targets will be missed without stronger intervention.

The question this project answers: **what prescribing policy minimises resistant infections over the medium term, given what we can learn from surveillance data?**

### Why stochastic simulation

A systematic review of 38 AMR mathematical models (Arepeva et al., 2018) found that only 4 were fully stochastic and only 2 validated against real data. Existing approaches fall into three camps:

| Approach | Limitation |
|----------|------------|
| Deterministic compartmental models | No stochastic dynamics, limited policy evaluation |
| RL on synthetic environments (e.g. `abx_amr_simulator`) | Not fitted to real surveillance data |
| ML prediction from genomic data | Predicts resistance phenotype, doesn't simulate policy outcomes |

This project uses a generalised stochastic simulation engine that learns parameters from real UK data via simulation-based inference, then evaluates candidate prescribing policies — bridging the gap between data-driven fitting and forward-looking policy comparison.

---

## Data

All data is freely available from UKHSA Fingertips (`dat/fetch_fingertips.sh`). The project uses 40 quarters (2015 Q4 – 2025 Q3) of England-level data:

| Dataset | Source | Frequency |
|---------|--------|-----------|
| E. coli 3rd-gen cephalosporin resistance % | Fingertips AMR indicators | Quarterly |
| Broad-spectrum prescribing % (cephalosporin/quinolone/co-amoxiclav) | Fingertips AMR indicators | Quarterly |
| E. coli bacteraemia counts and rates | Fingertips (130 acute trusts) | Annual / rolling monthly |
| MRSA, MSSA, *C. difficile* rates | Fingertips (130 acute trusts) | Annual |

### Key observations from the data

- **Resistance is trending up.** E. coli 3GC resistance rose from 11% to 17% over 40 quarters (2015–2025).
- **Prescribing fell then rebounded.** Broad-spectrum prescribing dropped from ~11% to ~7% by 2020, then partially recovered to ~9%.
- **Cross-sectional signal is weak.** ICB-level correlation between prescribing and resistance is r=0.08 (n=106) — resistance is driven by multiple factors beyond local prescribing.
- **Trust-level heterogeneity is substantial.** E. coli BSI rates range from 20–160 per 100k across trusts, supporting the case for trust-specific or archetype-based modelling.

Regenerate the exploratory plots with `python3 dat/explore.py`.

---

## Model

A two-strain (susceptible/resistant *E. coli*) stochastic simulation with three partitions:

```
┌──────────────────────────────────────────────────┐
│  PRESCRIBING PROCESS (policy lever)              │
│  State: [cephalosporin_rate]                     │
└──────────────┬───────────────────────────────────┘
               │ selective pressure
               ▼
┌───────────────────────────────────────────────────┐
│  COLONISATION DYNAMICS (Euler-Maruyama SDE)       │
│  State: [susceptible_fraction, resistant_fraction]│
│                                                   │
│  dS = turnover·(S_community - S)                  │
│     + transmission·S·U                            │
│     - selection·ceph_rate·S                       │
│     + fitness_cost·R                              │
│                                                   │
│  dR = turnover·(R_community - R)                  │
│     + transmission·R·U                            │
│     + selection·ceph_rate·S                       │
│     - fitness_cost·R                              │
│                                                   │
│  + state-dependent diffusion noise                │
└──────────────┬────────────────────────────────────┘
               │ colonisation fractions
               ▼
┌─────────────────────────────────────────────────────┐
│  INFECTION PROCESS (Poisson draws)                  │
│  State: [susceptible_bsi_count, resistant_bsi_count]│
│  λ = infection_prob × colonised_patients × dt       │
└─────────────────────────────────────────────────────┘
```

The colonisation SDE captures: patient turnover towards community baseline, within-hospital transmission, selection pressure from cephalosporin prescribing shifting the R/S ratio, and a fitness cost allowing resistant strains to revert when pressure is removed. The infection process converts colonisation fractions into clinically observable BSI events via Poisson sampling.

**Source:** `pkg/amr/colonisation.go`, `pkg/amr/infection.go`

---

## Inference

Four parameters are learned from the England-level resistance time series via simulation-based inference (`cfg/amr_inference.yaml`):

| Parameter | Learned value | Meaning |
|-----------|--------------|---------|
| `transmission_rate` | 0.038 | Within-hospital colonisation acquisition rate |
| `selection_coefficient` | 0.135 | Strength of cephalosporin prescribing in shifting R/S ratio |
| `fitness_cost` | 0.017 | Resistant strain reversion rate without selective pressure |
| `community_resistant_prevalence` | 0.116 | Baseline resistant colonisation fraction at admission |

The inference pipeline feeds the real time-varying prescribing and resistance data into the fitting process:

- A `resistance_trend` partition cycles through the 40 quarterly observed resistance values, driving the `DataGenerationIteration` mean so the inference target tracks the actual 0.11 → 0.17 trend.
- A `prescribing_trend` partition cycles through the 40 quarterly prescribing values, replayed inside the embedded simulation via `FromHistoryIteration` so the colonisation model sees time-varying prescribing pressure during fitting.
- Data is cycled over 1000 inference steps to ensure posterior convergence.
- The `NewDataReplayIteration` helper (`pkg/amr/data_replay.go`) loads JSON data files and creates cycling `FromStorageIteration` instances for use in YAML configs.

The posterior converges within ~50 steps. Run `python3 dat/plot_inference.py` for convergence diagnostics.

---

## Policy Evaluation

Four prescribing policies are implemented as drop-in replacements for the prescribing partition:

| Policy | Description | Source |
|--------|-------------|--------|
| **Baseline** | Constant cephalosporin rate (0.3) | `ParamValuesIteration` (built-in) |
| **Cycling** | Quarterly alternation: high (0.3) ↔ low (0.05) every 13 steps | `pkg/amr/cycling.go` |
| **Threshold** | Default rate (0.3) drops to escalation rate (0.05) when resistance exceeds 15% | `pkg/amr/threshold.go` |
| **Restriction** | Linear ramp-down from 0.3 to 0.1 over 26 steps, then hold | `pkg/amr/restriction.go` |

Each policy is evaluated over 200 time steps with **10 stochastic trajectories** (different RNG seeds) to quantify uncertainty. All simulations use the learned posterior mean parameters.

### Results

Cumulative resistant BSI over 200 time steps (mean ± 2σ across 10 seeds):

| Policy | Cumulative resistant BSI | vs Baseline | Final resistance ratio |
|--------|------------------------:|------------:|----------------------:|
| Baseline | 186.8 ± 28.0 | — | 0.546 ± 0.031 |
| Cycling | 165.8 ± 35.5 | −11.2% | 0.481 ± 0.034 |
| Threshold | 152.7 ± 23.1 | **−18.3%** | 0.447 ± 0.032 |
| Restriction | 152.5 ± 24.4 | **−18.4%** | 0.430 ± 0.035 |

### Findings

1. **All three active policies reduce resistant infections compared to the constant baseline.** The reductions are statistically meaningful — the uncertainty bands separate clearly by ~100 time steps.

2. **Threshold escalation and restriction perform similarly (~18% reduction), both outperforming cycling (~11%).** This suggests that sustained reduction in cephalosporin pressure is more effective than periodic oscillation. The fitness cost of resistance (0.017) is small enough that brief low-pressure windows during cycling don't allow sufficient reversion.

3. **Threshold escalation is adaptive and restriction is not, but they converge.** The threshold policy reacts to rising resistance, while the restriction policy ramps down on a fixed schedule. Both achieve similar outcomes because the resistance trajectory under the learned parameters reliably crosses the 15% threshold.

4. **Community importation dominates the resistance floor.** The learned `community_resistant_prevalence` of 0.116 means that ~12% of patients arrive already colonised with resistant strains. No prescribing policy can push resistance below this floor — stewardship limits the amplification, not the baseline.

5. **Uncertainty grows over time but doesn't change the ranking.** The ±2σ bands widen over 200 steps, but the policy ordering (restriction ≈ threshold < cycling < baseline) is consistent across seeds.

---

## Interactive Notebooks

Go notebooks in `nbs/` use the [GoNB](https://github.com/janpfeifer/gonb) Jupyter kernel with [go-echarts](https://github.com/go-echarts/go-echarts) via [gonb-echarts](https://github.com/janpfeifer/gonb-echarts):

| Notebook | Contents |
|----------|----------|
| `nbs/data_exploration.ipynb` | England time series, ICB scatter, trust bacteraemia rates |
| `nbs/model_validation.ipynb` | Simulation dynamics, posterior convergence, parameter samples, log-normalisation |
| `nbs/policy_comparison.ipynb` | Prescribing rate, resistance ratio, colonisation dynamics, cumulative resistant BSI (mean ± 2σ, 10 seeds) |

---

## Running the Project

```bash
# Build and test
go build ./...
go test -count=1 ./...

# Data acquisition
./dat/fetch_fingertips.sh
python3 dat/prepare_baseline.py
python3 dat/prepare_sbi_data.py

# Exploratory analysis
python3 dat/explore.py

# Simulation
go run github.com/umbralcalc/stochadex/cmd/stochadex --config cfg/amr_simulation.yaml

# Inference (learns 4 parameters from surveillance data)
go run github.com/umbralcalc/stochadex/cmd/stochadex --config cfg/amr_inference.yaml
python3 dat/plot_inference.py
python3 dat/plot_validation.py

# Policy evaluation (4 policies × 10 seeds = 40 simulations)
python3 dat/run_policy_evaluation.py
python3 dat/plot_policy_comparison.py
```

---

## Project Structure

```
cfg/
  amr_simulation.yaml          # forward simulation config
  amr_inference.yaml            # simulation-based inference config
  amr_policy_*.yaml             # one config per policy (4 total)
pkg/amr/
  colonisation.go               # two-strain SDE colonisation dynamics
  infection.go                  # Poisson BSI process
  cycling.go                    # cycling prescribing policy
  threshold.go                  # threshold escalation policy
  restriction.go                # restriction ramp-down policy
  data_replay.go                # JSON data loading for FromStorageIteration
  *_test.go                     # unit tests with harness checks
  *_settings.yaml               # test settings files
dat/
  fetch_fingertips.sh           # download UKHSA surveillance data
  prepare_baseline.py           # aggregate England-level time series
  prepare_sbi_data.py           # format data for inference
  run_policy_evaluation.py      # multi-seed policy evaluation runner
  explore.py                    # exploratory analysis plots
  plot_inference.py             # posterior convergence diagnostics
  plot_validation.py            # simulated vs observed comparison
  plot_policy_comparison.py     # policy comparison plots
  fingertips_*.csv              # raw Fingertips data
  baseline_england.csv          # processed England time series
  sbi_*.json                    # formatted inference input data
  policy_*_seed*.log            # simulation output logs (40 total)
nbs/
  data_exploration.ipynb        # data visualisation (GoNB)
  model_validation.ipynb        # inference diagnostics (GoNB)
  policy_comparison.ipynb       # policy comparison (GoNB)
```

---

## Extensions

1. **Multi-pathogen:** Add *K. pneumoniae* (carbapenem resistance) and *S. aureus* (MRSA) — the data is already downloaded.
2. **Trust-level fitting:** Fit per-trust or per-archetype parameters instead of England-level averages, using the ICB sub-location data already prepared.
3. **Network effects:** Model patient transfers between trusts as a transmission pathway using Hospital Episode Statistics.
4. **Economic layer:** Add treatment costs, bed-days, and mortality to produce cost-effectiveness estimates for stewardship interventions.
5. **One Health:** Incorporate agricultural antibiotic use data (EFSA) to model community resistance importation dynamics.

---

## Data Sources

| Source | Data type | Access |
|--------|-----------|--------|
| UKHSA Fingertips AMR Local Indicators | AMR indicators by trust/CCG/practice | Free, downloadable |
| OpenPrescribing.net | GP & hospital prescribing by trust/practice | Free REST API |
| ECDC Surveillance Atlas (EARS-Net) | Resistance data, 8 species, EU/EEA | Free, downloadable |
| ESPAUR Reports | Annual UK AMR and prescribing reports | Free PDF + data tables |

---

## References

- Arepeva et al. (2018) — Systematic review of 38 AMR mathematical models. *Antimicrobial Resistance & Infection Control*
- Pezzani et al. (2024) — Ross-Macdonald model adapted for CRKP hospital transmission. *Scientific Reports*
- Rawson et al. (2020) — System dynamics modelling for antibiotic prescribing policy in hospitals. *J. Operational Research Society*
- `abx_amr_simulator` — Gymnasium-compatible AMR simulation for RL policy testing (bioRxiv, March 2026)
- ESPAUR 2024–25 Report — Latest English AMR surveillance data and national action plan progress
