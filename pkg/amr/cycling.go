package amr

import (
	"math"

	"github.com/umbralcalc/stochadex/pkg/simulator"
)

// CyclingPrescribingIteration alternates the cephalosporin prescribing rate
// between a high and low value on a fixed schedule, modelling an antibiotic
// cycling policy.
//
// State: [cephalosporin_rate]
//
// Params:
//   - high_rate: prescribing rate during "on" phase
//   - low_rate: prescribing rate during "off" phase (alternative antibiotic)
//   - cycle_period: duration of each phase in time units
type CyclingPrescribingIteration struct{}

func (c *CyclingPrescribingIteration) Configure(
	partitionIndex int,
	settings *simulator.Settings,
) {
}

func (c *CyclingPrescribingIteration) Iterate(
	params *simulator.Params,
	partitionIndex int,
	stateHistories []*simulator.StateHistory,
	timestepsHistory *simulator.CumulativeTimestepsHistory,
) []float64 {
	highRate := params.Map["high_rate"][0]
	lowRate := params.Map["low_rate"][0]
	cyclePeriod := params.Map["cycle_period"][0]

	currentTime := timestepsHistory.Values.AtVec(0)
	cycleIndex := int(math.Floor(currentTime / cyclePeriod))

	if cycleIndex%2 == 0 {
		return []float64{highRate}
	}
	return []float64{lowRate}
}
