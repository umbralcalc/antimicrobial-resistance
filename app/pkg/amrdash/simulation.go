package amrdash

import (
	"github.com/umbralcalc/antimicrobial-resistance/pkg/amr"
	"github.com/umbralcalc/stochadex/pkg/simulator"
)

// BuildAMRSimulation constructs the stochadex generator for the AMR
// dashboard. Eight partitions, wired in declaration order:
//
//	policy_action      action-state input from the sliders (policy choice + tuning)
//	prescribing        per-step cephalosporin rate, picked by the chosen policy
//	colonisation       two-strain S/R SDE on hospital colonisation fractions
//	infection          Poisson BSI counts driven by colonisation
//	resistance_ratio   passthrough of colonisation[1] for the line chart
//	cumulative_bsi     running total of resistant BSI counts
//	comparison_bars    (x, y, w, h) rectangles for the policy comparison strip
//	display            aggregate readout values (time, policy, rate, ratio, total)
//
// The colonisation / infection iterations are the same ones the project's
// CLI configs use; only the prescribing partition is dashboard-specific
// (it dispatches across all four policies in one iteration).
func BuildAMRSimulation() *simulator.ConfigGenerator {
	policyAction := &simulator.PartitionConfig{
		Name:      "policy_action",
		Iteration: &PolicyActionIteration{},
		Params: simulator.NewParams(map[string][]float64{
			"action_state_values": {
				DefaultPolicyIndex,
				DefaultCyclePeriod,
				DefaultThreshold,
				DefaultRampPeriod,
			},
		}),
		InitStateValues: []float64{
			DefaultPolicyIndex,
			DefaultCyclePeriod,
			DefaultThreshold,
			DefaultRampPeriod,
		},
		StateHistoryDepth: 1,
		Seed:              0,
	}

	prescribing := &simulator.PartitionConfig{
		Name:      "prescribing",
		Iteration: &PolicySwitchPrescribingIteration{},
		Params: simulator.NewParams(map[string][]float64{
			"colonisation_partition": {2},
		}),
		ParamsFromUpstream: map[string]simulator.NamedUpstreamConfig{
			"policy_action": {Upstream: "policy_action"},
		},
		InitStateValues:   []float64{BaselineRate},
		StateHistoryDepth: 1,
		Seed:              0,
	}

	colonisation := &simulator.PartitionConfig{
		Name:      "colonisation",
		Iteration: &amr.ColonisationDynamicsIteration{},
		Params: simulator.NewParams(map[string][]float64{
			"community_susceptible_prevalence": {0.15},
			"community_resistant_prevalence":   {0.1164},
			"turnover_rate":                    {0.1},
			"transmission_rate":                {0.0384},
			"selection_coefficient":            {0.1351},
			"fitness_cost":                     {0.0170},
			"noise_scale":                      {0.01},
			"prescribing_partition":            {1},
		}),
		InitStateValues:   []float64{0.15, 0.05},
		StateHistoryDepth: 1,
		Seed:              9182,
	}

	infection := &simulator.PartitionConfig{
		Name:      "infection",
		Iteration: &amr.InfectionProcessIteration{},
		Params: simulator.NewParams(map[string][]float64{
			"infection_probability":  {0.01},
			"patient_population":     {500.0},
			"colonisation_partition": {2},
		}),
		InitStateValues:   []float64{0.0, 0.0},
		StateHistoryDepth: 1,
		Seed:              3347,
	}

	resistanceRatio := &simulator.PartitionConfig{
		Name:      "resistance_ratio",
		Iteration: &ResistanceRatioIteration{},
		Params:    simulator.NewParams(map[string][]float64{}),
		ParamsFromUpstream: map[string]simulator.NamedUpstreamConfig{
			"colonisation_values": {Upstream: "colonisation"},
		},
		InitStateValues:   []float64{0.05},
		StateHistoryDepth: 1,
		Seed:              0,
	}

	cumulativeBsi := &simulator.PartitionConfig{
		Name:      "cumulative_bsi",
		Iteration: &CumulativeResistantBsiIteration{},
		Params:    simulator.NewParams(map[string][]float64{}),
		ParamsFromUpstream: map[string]simulator.NamedUpstreamConfig{
			"infection_values": {Upstream: "infection"},
		},
		InitStateValues:   []float64{0.0},
		StateHistoryDepth: 1,
		Seed:              0,
	}

	referenceBars := &simulator.PartitionConfig{
		Name:      "reference_bars",
		Iteration: &ReferenceBarsIteration{},
		Params:    simulator.NewParams(map[string][]float64{}),
		ParamsFromUpstream: map[string]simulator.NamedUpstreamConfig{
			"cumulative_values": {Upstream: "cumulative_bsi"},
		},
		InitStateValues:   make([]float64, CmpRefCount*4),
		StateHistoryDepth: 1,
		Seed:              0,
	}

	userBar := &simulator.PartitionConfig{
		Name:      "user_bar",
		Iteration: &UserBarIteration{},
		Params:    simulator.NewParams(map[string][]float64{}),
		ParamsFromUpstream: map[string]simulator.NamedUpstreamConfig{
			"cumulative_values": {Upstream: "cumulative_bsi"},
		},
		InitStateValues:   make([]float64, 4),
		StateHistoryDepth: 1,
		Seed:              0,
	}

	displayCounts := &simulator.PartitionConfig{
		Name:      "display_counts",
		Iteration: &DisplayCountsIteration{},
		Params:    simulator.NewParams(map[string][]float64{}),
		ParamsFromUpstream: map[string]simulator.NamedUpstreamConfig{
			"policy_action":     {Upstream: "policy_action"},
			"cumulative_values": {Upstream: "cumulative_bsi"},
		},
		InitStateValues:   []float64{0.0, DefaultPolicyIndex, 0.0},
		StateHistoryDepth: 1,
		Seed:              0,
	}

	displayRates := &simulator.PartitionConfig{
		Name:      "display_rates",
		Iteration: &DisplayRatesIteration{},
		Params:    simulator.NewParams(map[string][]float64{}),
		ParamsFromUpstream: map[string]simulator.NamedUpstreamConfig{
			"prescribing_values":  {Upstream: "prescribing"},
			"colonisation_values": {Upstream: "colonisation"},
		},
		InitStateValues:   []float64{BaselineRate, 0.05},
		StateHistoryDepth: 1,
		Seed:              0,
	}

	gen := simulator.NewConfigGenerator()
	for _, p := range []*simulator.PartitionConfig{
		policyAction,
		prescribing,
		colonisation,
		infection,
		resistanceRatio,
		cumulativeBsi,
		referenceBars,
		userBar,
		displayCounts,
		displayRates,
	} {
		gen.SetPartition(p)
	}

	gen.SetSimulation(&simulator.SimulationConfig{
		OutputCondition: &simulator.EveryStepOutputCondition{},
		TerminationCondition: &simulator.NumberOfStepsTerminationCondition{
			MaxNumberOfSteps: SimSteps,
		},
		TimestepFunction: &simulator.ConstantTimestepFunction{Stepsize: 1.0},
		InitTimeValue:    0.0,
	})
	return gen
}
