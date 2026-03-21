package amr

import (
	"math"
	"math/rand"

	"github.com/umbralcalc/stochadex/pkg/simulator"
)

// ColonisationDynamicsIteration implements a two-strain (susceptible/resistant)
// colonisation model as a stochastic differential equation. It tracks the
// fraction of hospital patients colonised with susceptible (S) and resistant (R)
// E. coli strains.
//
// State: [susceptible_fraction, resistant_fraction]
//
// Params:
//   - community_susceptible_prevalence: baseline S colonisation at admission
//   - community_resistant_prevalence: baseline R colonisation at admission
//   - turnover_rate: patient admission/discharge rate (1 / avg length of stay)
//   - transmission_rate: within-hospital transmission coefficient
//   - selection_coefficient: how cephalosporin use shifts R/S ratio
//   - fitness_cost: reversion rate from R to S absent selective pressure
//   - noise_scale: diffusion coefficient for stochastic fluctuations
//   - prescribing_partition: index of the partition providing prescribing rates
type ColonisationDynamicsIteration struct {
	prescribingPartitionIndex int
	rng                       *rand.Rand
}

func (c *ColonisationDynamicsIteration) Configure(
	partitionIndex int,
	settings *simulator.Settings,
) {
	c.prescribingPartitionIndex = int(
		settings.Iterations[partitionIndex].Params.Map["prescribing_partition"][0],
	)
	c.rng = rand.New(rand.NewSource(
		int64(settings.Iterations[partitionIndex].Seed),
	))
}

func (c *ColonisationDynamicsIteration) Iterate(
	params *simulator.Params,
	partitionIndex int,
	stateHistories []*simulator.StateHistory,
	timestepsHistory *simulator.CumulativeTimestepsHistory,
) []float64 {
	// Read parameters
	communityS := params.Map["community_susceptible_prevalence"][0]
	communityR := params.Map["community_resistant_prevalence"][0]
	turnover := params.Map["turnover_rate"][0]
	transmission := params.Map["transmission_rate"][0]
	selection := params.Map["selection_coefficient"][0]
	fitnessCost := params.Map["fitness_cost"][0]
	noiseScale := params.Map["noise_scale"][0]

	// Current state
	current := stateHistories[partitionIndex]
	S := current.Values.At(0, 0)
	R := current.Values.At(0, 1)

	// Prescribing input: cephalosporin rate from upstream partition
	cephRate := stateHistories[c.prescribingPartitionIndex].Values.At(0, 0)

	// Time step
	dt := timestepsHistory.NextIncrement

	// Fraction uncolonised
	U := math.Max(1.0-S-R, 0.0)

	// Drift terms for susceptible fraction
	driftS := turnover*(communityS-S) + // patient turnover towards community baseline
		transmission*S*U - // within-hospital transmission to uncolonised
		selection*cephRate*S + // selection pressure kills susceptible
		fitnessCost*R // resistant reverts to susceptible

	// Drift terms for resistant fraction
	driftR := turnover*(communityR-R) + // patient turnover towards community baseline
		transmission*R*U + // within-hospital transmission to uncolonised
		selection*cephRate*S - // selection creates resistant from susceptible
		fitnessCost*R // fitness cost of resistance

	// Diffusion (Wiener process noise scaled by sqrt(fraction) to keep
	// noise proportional to population size)
	sqrtDt := math.Sqrt(dt)
	noiseS := noiseScale * math.Sqrt(math.Max(S, 0.0)) * c.rng.NormFloat64() * sqrtDt
	noiseR := noiseScale * math.Sqrt(math.Max(R, 0.0)) * c.rng.NormFloat64() * sqrtDt

	// Euler-Maruyama update
	newS := S + driftS*dt + noiseS
	newR := R + driftR*dt + noiseR

	// Clamp to valid range [0, 1] with S + R <= 1
	newS = math.Max(newS, 0.0)
	newR = math.Max(newR, 0.0)
	total := newS + newR
	if total > 1.0 {
		newS = newS / total
		newR = newR / total
	}

	return []float64{newS, newR}
}
