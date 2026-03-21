package amr

import (
	"math"

	"github.com/umbralcalc/stochadex/pkg/simulator"
)

// RestrictionPrescribingIteration models a restriction policy that gradually
// reduces the cephalosporin prescribing rate from an initial level to a target
// cap over a ramp period, then holds at the target.
//
// State: [cephalosporin_rate]
//
// Params:
//   - initial_rate: starting cephalosporin prescribing rate
//   - target_rate: cap to reach after ramp period
//   - ramp_period: time units over which the transition occurs
type RestrictionPrescribingIteration struct{}

func (r *RestrictionPrescribingIteration) Configure(
	partitionIndex int,
	settings *simulator.Settings,
) {
}

func (r *RestrictionPrescribingIteration) Iterate(
	params *simulator.Params,
	partitionIndex int,
	stateHistories []*simulator.StateHistory,
	timestepsHistory *simulator.CumulativeTimestepsHistory,
) []float64 {
	initialRate := params.Map["initial_rate"][0]
	targetRate := params.Map["target_rate"][0]
	rampPeriod := params.Map["ramp_period"][0]

	currentTime := timestepsHistory.Values.AtVec(0)
	progress := math.Min(currentTime/rampPeriod, 1.0)
	rate := initialRate + (targetRate-initialRate)*progress

	return []float64{rate}
}
