package amr

import (
	"github.com/umbralcalc/stochadex/pkg/simulator"
)

// ThresholdPrescribingIteration models a threshold escalation policy: when the
// resistant colonisation fraction exceeds a threshold, prescribing switches from
// the default cephalosporin rate to a lower escalation rate (i.e. alternative
// antibiotics). This creates an adaptive feedback loop — high resistance triggers
// reduced cephalosporin use, which eventually lowers resistance.
//
// State: [cephalosporin_rate]
//
// Params:
//   - default_rate: normal cephalosporin prescribing rate
//   - escalation_rate: reduced rate when resistance threshold is exceeded
//   - resistance_threshold: resistant fraction triggering the switch
//   - colonisation_partition: index of partition providing colonisation state
type ThresholdPrescribingIteration struct {
	colonisationPartitionIndex int
}

func (th *ThresholdPrescribingIteration) Configure(
	partitionIndex int,
	settings *simulator.Settings,
) {
	th.colonisationPartitionIndex = int(
		settings.Iterations[partitionIndex].Params.Map["colonisation_partition"][0],
	)
}

func (th *ThresholdPrescribingIteration) Iterate(
	params *simulator.Params,
	partitionIndex int,
	stateHistories []*simulator.StateHistory,
	timestepsHistory *simulator.CumulativeTimestepsHistory,
) []float64 {
	defaultRate := params.Map["default_rate"][0]
	escalationRate := params.Map["escalation_rate"][0]
	threshold := params.Map["resistance_threshold"][0]

	resistantFraction := stateHistories[th.colonisationPartitionIndex].Values.At(0, 1)

	if resistantFraction > threshold {
		return []float64{escalationRate}
	}
	return []float64{defaultRate}
}
