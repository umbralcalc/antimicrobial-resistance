# Stochadex SDK — Project Conventions

This project uses the [stochadex](https://github.com/umbralcalc/stochadex) SDK to build and run simulations.

## The Iteration Interface

Every simulation component implements `simulator.Iteration`:

```go
type Iteration interface {
    Configure(partitionIndex int, settings *Settings)
    Iterate(params *Params, partitionIndex int, stateHistories []*StateHistory,
            timestepsHistory *CumulativeTimestepsHistory) []float64
}
```

**Rules:**
- `Configure` is called once at setup. Use it to seed RNGs, read fixed config, allocate buffers. All mutable state must be re-initializable here (no statefulness residues between runs).
- `Iterate` is called each step. It must NOT mutate `params`. It returns the next state as `[]float64` with length equal to `StateWidth`.
- `stateHistories` gives access to all partitions' rolling state windows. `stateHistories[i].Values.At(row, col)` where row=0 is the latest state.
- `timestepsHistory.Values.AtVec(0)` is the current time. `timestepsHistory.NextIncrement` is the upcoming time step.
- Partitions communicate by wiring one partition's output state into another's params via `params_from_upstream` in config.

## YAML Config Format (API Code-Generation Path)

Simulations are defined in YAML and run via the stochadex CLI, which generates and executes Go code.

```yaml
main:
  partitions:
  - name: my_partition              # unique name
    iteration: myVar                # references a variable from extra_vars
    params:
      some_param: [1.0, 2.0]       # all param values are []float64
    params_from_upstream:           # wire upstream partition output → this partition's params
      latest_values:
        upstream: other_partition   # name of the upstream partition
    params_as_partitions:           # reference partition names as param values (resolved to indices)
      data_partition: [some_partition]
    init_state_values: [0.0, 0.0]  # initial state (determines state_width)
    state_history_depth: 1          # rolling window size
    seed: 1234                      # RNG seed (0 = no randomness needed)
    extra_packages:                 # Go import paths
    - github.com/umbralcalc/stochadex/pkg/continuous
    extra_vars:                     # Go variable declarations
    - myVar: "&continuous.WienerProcessIteration{}"

  simulation:
    output_condition: "&simulator.EveryStepOutputCondition{}"
    output_function: "&simulator.StdoutOutputFunction{}"
    termination_condition: "&simulator.NumberOfStepsTerminationCondition{MaxNumberOfSteps: 100}"
    timestep_function: "&simulator.ConstantTimestepFunction{Stepsize: 1.0}"
    init_time_value: 0.0
```

### Common Output Functions
- `&simulator.StdoutOutputFunction{}` — print to stdout
- `simulator.NewJsonLogOutputFunction("./data.log")` — write JSON log file
- `&simulator.NilOutputFunction{}` — no output (for embedded sims)

### Common Termination Conditions
- `&simulator.NumberOfStepsTerminationCondition{MaxNumberOfSteps: N}`
- `&simulator.TimeElapsedTerminationCondition{MaxTimeElapsed: T}`

### Common Timestep Functions
- `&simulator.ConstantTimestepFunction{Stepsize: 0.1}`
- `&simulator.ExponentialDistributionTimestepFunction{RateLambda: 1.0}`

## Build & Run

```bash
go build ./...                                    # compile this project
go test -count=1 ./...                            # run all tests
./dat/fetch_fingertips.sh                         # download Fingertips AMR data
python3 dat/prepare_baseline.py                   # prepare baseline event rates
python3 dat/prepare_sbi_data.py                   # format data for inference
python3 dat/explore.py                            # exploratory analysis plots
go run github.com/umbralcalc/stochadex/cmd/stochadex --config cfg/amr_simulation.yaml
go run github.com/umbralcalc/stochadex/cmd/stochadex --config cfg/amr_inference.yaml
python3 dat/plot_inference.py                     # plot inference results
python3 dat/plot_validation.py                    # plot validation comparison
python3 dat/run_policy_evaluation.py              # run 4 policies × 10 seeds
python3 dat/plot_policy_comparison.py             # plot policy comparison
```

## Project-Specific Iterations

### `ColonisationDynamicsIteration` (`pkg/amr/colonisation.go`)

Two-strain (S/R) Euler-Maruyama SDE tracking colonisation fractions. Reads prescribing rate from an upstream partition (`prescribing_partition` param) or a direct `prescribing_rate` param. Supports a `learned_params` vector `[transmission_rate, selection_coefficient, fitness_cost, community_resistant_prevalence]` wired from the inference posterior.

State: `[susceptible_fraction, resistant_fraction]`

### `InfectionProcessIteration` (`pkg/amr/infection.go`)

Converts colonisation fractions to BSI counts via Poisson sampling. Reads from an upstream colonisation partition (`colonisation_partition` param).

State: `[susceptible_bsi_count, resistant_bsi_count]`

### Policy Iterations

All output `[cephalosporin_rate]` (state width 1), drop-in replacements for partition 0:

| Iteration | Source | Key Params |
|-----------|--------|------------|
| `CyclingPrescribingIteration` | `pkg/amr/cycling.go` | `high_rate`, `low_rate`, `cycle_period` |
| `ThresholdPrescribingIteration` | `pkg/amr/threshold.go` | `default_rate`, `escalation_rate`, `resistance_threshold`, `colonisation_partition` |
| `RestrictionPrescribingIteration` | `pkg/amr/restriction.go` | `initial_rate`, `target_rate`, `ramp_period` |

### `NewDataReplayIteration` (`pkg/amr/data_replay.go`)

Helper function that loads a JSON file containing `[][]float64` data and returns a cycling `FromStorageIteration`. Used in YAML `extra_vars` to feed real time-varying data into inference configs:

```yaml
extra_vars:
- myIter: "amr.NewDataReplayIteration(\"./dat/sbi_observed_resistance.json\", 1001)"
```

## YAML Config Files

| Config | Purpose |
|--------|---------|
| `cfg/amr_simulation.yaml` | Forward simulation with constant prescribing |
| `cfg/amr_inference.yaml` | SBI: learns 4 parameters from surveillance data (10 partitions + embedded sim) |
| `cfg/amr_policy_baseline.yaml` | Constant prescribing policy |
| `cfg/amr_policy_cycling.yaml` | Quarterly cycling policy |
| `cfg/amr_policy_threshold.yaml` | Adaptive threshold escalation policy |
| `cfg/amr_policy_restriction.yaml` | Ramp-down restriction policy |

### Inference Config Structure

The inference config (`cfg/amr_inference.yaml`) has 10 outer partitions:

0. `resistance_trend` — cycles real observed resistance data (drives time-varying inference target)
1. `prescribing_trend` — cycles real prescribing data (fed into embedded sim)
2. `observed_resistance` — `DataGenerationIteration` with time-varying mean from resistance_trend
3. `observed_rolling_mean` — exponential kernel rolling mean of observed data
4. `observed_rolling_cov` — exponential kernel rolling covariance
5. `params_posterior_log_norm` — log-normalisation tracking (loglike at index **6**)
6. `params_posterior_mean` — posterior mean of 4 learned params
7. `params_posterior_cov` — posterior covariance (4×4 = 16 values)
8. `params_generating_process` — posterior sampler
9. `amr_embedded_simulation` — runs inner sim (6 inner partitions, 40 steps)

**Embedded sim inner partitions:**
0. `observed_rolling_mean_copy` (FromHistoryIteration)
1. `observed_rolling_cov_copy` (FromHistoryIteration)
2. `prescribing_copy` (FromHistoryIteration — replays prescribing into colonisation)
3. `colonisation_sim` (ColonisationDynamicsIteration — reads prescribing from partition 2)
4. `resistance_extractor` (CopyValuesIteration — copies index 1 from partition **3**)
5. `data_comparison` (DataComparisonIteration)

### Learned Parameters (current posterior means)

Used in all `cfg/amr_policy_*.yaml` configs:
- `transmission_rate: [0.0384]`
- `selection_coefficient: [0.1351]`
- `fitness_cost: [0.0170]`
- `community_resistant_prevalence: [0.1164]`

## Notebooks

Interactive Go notebooks in `nbs/` use the [GoNB](https://github.com/janpfeifer/gonb) Jupyter kernel with [go-echarts](https://github.com/go-echarts/go-echarts) for visualisation via [gonb-echarts](https://github.com/janpfeifer/gonb-echarts).

| Notebook | Purpose |
|----------|---------|
| `nbs/data_exploration.ipynb` | Fingertips data visualisation: England time series, ICB scatter, trust bacteraemia rates |
| `nbs/model_validation.ipynb` | Simulation output plots (colonisation, infection) and inference diagnostics (posterior convergence, parameter samples, log-normalisation) |
| `nbs/policy_comparison.ipynb` | Multi-seed policy comparison: prescribing rate, resistance ratio, colonisation dynamics, cumulative resistant BSI (mean ± 2σ across 10 seeds) |

**Conventions:**
- Each code cell is self-contained: imports above `%%`, executable code below. Variables from `%%` blocks do NOT persist across cells in gonb.
- Data loading is repeated per cell (e.g. `NewStateTimeStorageFromJsonLogEntries`) rather than shared across cells.
- The `!*go mod edit -replace` cell at the top of each notebook is for local development — do not remove it.
- Use `opts.Float(0.5)` not raw `0.5` for go-echarts `types.Float` fields (e.g. `Opacity` in `LineStyle`/`ItemStyle`).

## Testing Conventions

- Unit tests live alongside source as `*_test.go` files.
- Use `t.Run("description", func(t *testing.T) { ... })` subtests.
- Always include a subtest using `simulator.RunWithHarnesses(settings, implementations)` — this checks for NaN outputs, wrong state widths, params mutation, state history integrity, and statefulness residues.
- Load settings from a colocated YAML file (e.g., `my_iteration_settings.yaml` next to `my_iteration_test.go`).
- Use `gonum.org/v1/gonum/floats` for float comparisons, never raw `==`.
- No mocking — use real implementations.

## Multi-Seed Policy Evaluation

`dat/run_policy_evaluation.py` runs each policy config with 10 different RNG seeds (modifying colonisation and infection seeds via temp YAML files). Output logs are saved as `dat/policy_{name}_seed{i}.log`. The notebook loads all seeds and computes per-timestep mean ± 2σ.

## Built-In Iterations Reference

### continuous (github.com/umbralcalc/stochadex/pkg/continuous)

| Iteration | Params | Description |
|-----------|--------|-------------|
| `WienerProcessIteration` | `variances` | Brownian motion |
| `GeometricBrownianMotionIteration` | `variances` | Multiplicative Brownian motion |
| `OrnsteinUhlenbeckIteration` | `thetas`, `mus`, `sigmas` | Mean-reverting process |
| `DriftDiffusionIteration` | `drift_coefficients`, `diffusion_coefficients` | General drift-diffusion SDE |
| `DriftJumpDiffusionIteration` | `drift_coefficients`, `diffusion_coefficients`, `jump_rates` | Drift-diffusion with Poisson jumps |
| `CompoundPoissonProcessIteration` | `rates` | Compound Poisson process |
| `GradientDescentIteration` | `gradient`, `learning_rate`, `ascent` (optional) | Gradient-based optimization |
| `CumulativeTimeIteration` | (none) | Outputs cumulative simulation time |

### discrete (github.com/umbralcalc/stochadex/pkg/discrete)

| Iteration | Params | Description |
|-----------|--------|-------------|
| `PoissonProcessIteration` | `rates` | Poisson counting process |
| `BernoulliProcessIteration` | `state_value_observation_probs` | Binary outcomes |
| `BinomialObservationProcessIteration` | `observed_values`, `state_value_observation_probs`, `state_value_observation_indices` | Binomial draws |
| `CoxProcessIteration` | `rates` | Doubly-stochastic Poisson |
| `HawkesProcessIteration` | `intensity` | Self-exciting point process |
| `HawkesProcessIntensityIteration` | `background_rates` | Hawkes intensity function |
| `CategoricalStateTransitionIteration` | `transitions_from_N`, `transition_rates` | State machine |

### general (github.com/umbralcalc/stochadex/pkg/general)

| Iteration | Params | Description |
|-----------|--------|-------------|
| `ConstantValuesIteration` | (none) | Returns unchanged initial state |
| `CopyValuesIteration` | `partitions`, `partition_state_values` | Copies values from other partitions |
| `ParamValuesIteration` | `param_values` | Injects param values as state |
| `ValuesFunctionIteration` | (varies by Function) | User-defined function of state |
| `CumulativeIteration` | (none, wraps another) | Accumulates wrapped iteration output |
| `FromStorageIteration` | (none, uses Data field) | Streams pre-computed data |
| `FromHistoryIteration` | `latest_data_values` | Replays state history data |
| `EmbeddedSimulationRunIteration` | (varies) | Runs nested simulation each step |
| `ValuesCollectionIteration` | `values_state_width`, `empty_value` | Rolling collection of values |
| `ValuesChangingEventsIteration` | `default_values` | Routes by event value |
| `ValuesWeightedResamplingIteration` | `log_weight_partitions`, `data_values_partitions`, `past_discounting_factor` | Weighted resampling |
| `ValuesFunctionVectorMeanIteration` | `data_values_partition`, `latest_data_values` | Kernel-weighted rolling mean |
| `ValuesFunctionVectorCovarianceIteration` | `data_values_partition`, `latest_data_values`, `mean` | Kernel-weighted rolling covariance |

### inference (github.com/umbralcalc/stochadex/pkg/inference)

| Iteration | Params | Description |
|-----------|--------|-------------|
| `DataGenerationIteration` | `steps_per_resample`, `correlation_with_previous` (optional) | Synthetic data generation |
| `DataComparisonIteration` | `latest_data_values`, `cumulative`, `burn_in_steps` | Log-likelihood evaluation |
| `PosteriorMeanIteration` | `loglike_partitions`, `param_partitions`, `posterior_log_normalisation` | Posterior mean estimation |
| `PosteriorCovarianceIteration` | `loglike_partitions`, `param_partitions`, `posterior_log_normalisation`, `mean` | Posterior covariance estimation |
| `PosteriorLogNormalisationIteration` | `loglike_partitions`, `past_discounting_factor` | Log-normalisation tracking |

### kernels (github.com/umbralcalc/stochadex/pkg/kernels)

Kernels are not iterations — they implement `IntegrationKernel` and are used by iterations like `ValuesFunctionVectorMeanIteration`. Available: `ConstantIntegrationKernel`, `ExponentialIntegrationKernel`, `PeriodicIntegrationKernel`, `GaussianStateIntegrationKernel`, `TDistributionStateIntegrationKernel`, `BinnedIntegrationKernel`, `ProductIntegrationKernel`, `InstantaneousIntegrationKernel`.
