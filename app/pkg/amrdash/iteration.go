package amrdash

import (
	"math"

	"github.com/umbralcalc/stochadex/pkg/simulator"
)

// SimSteps is the number of timesteps the dashboard runs the simulation for
// before resetting via the reset button. 200 quarterly steps = 50 years,
// matching the project's offline policy evaluation horizon.
const SimSteps = 200

// Policy indices. The PolicyAction partition's state[0] picks among these.
const (
	PolicyBaseline    = 0
	PolicyCycling     = 1
	PolicyThreshold   = 2
	PolicyRestriction = 3
)

// Policy action vector layout. The slider panel writes to action_state_values
// in this order; PolicyActionIteration latches it onto state.
const (
	PAIdxPolicy      = 0
	PAIdxCyclePeriod = 1
	PAIdxThreshold   = 2
	PAIdxRampPeriod  = 3
	PolicyActionLen  = 4
)

// Defaults match the project's existing policy YAML configs so the
// dashboard's "default" sliders land on the same parameter values the
// offline evaluation uses for the comparison strip's reference bars.
var (
	DefaultPolicyIndex  = float64(PolicyBaseline)
	DefaultCyclePeriod  = 13.0  // weeks per phase (quarterly)
	DefaultThreshold    = 0.15  // resistance fraction trigger
	DefaultRampPeriod   = 26.0  // ramp duration in weeks
	BaselineRate        = 0.3   // constant cephalosporin rate
	CyclingHighRate     = 0.3   // on-phase rate during cycling
	CyclingLowRate      = 0.05  // off-phase rate during cycling
	ThresholdDefault    = 0.3   // before escalation triggers
	ThresholdEscalation = 0.05  // after escalation triggers
	RestrictionInitial  = 0.3   // start of ramp
	RestrictionTarget   = 0.1   // end of ramp
)

// terminated reports whether the simulation has reached the SimSteps
// horizon. The dexetera inline driver keeps invoking coordinator.Step()
// past the termination condition (stochadex only sets a "ready to
// terminate" flag — it doesn't actually halt the coordinator when the
// driver is calling Step directly), so we have to freeze our custom
// iterations explicitly. The amr.Colonisation / amr.Infection iterations
// continue running underneath, but as long as every downstream partition
// we own freezes, the user-visible charts and bars settle at year 50.
func terminated(timestepsHistory *simulator.CumulativeTimestepsHistory) bool {
	return timestepsHistory.Values.AtVec(0) >= float64(SimSteps)
}

// PolicyActionIteration is the slider-driven action partition. It echoes the
// most recent `action_state_values` vector as its state, so downstream
// iterations can read the active policy choice and tuning parameters from
// state history rather than chasing the live params map.
//
// State width: PolicyActionLen.
type PolicyActionIteration struct{}

func (p *PolicyActionIteration) Configure(int, *simulator.Settings) {}

func (p *PolicyActionIteration) Iterate(
	params *simulator.Params,
	partitionIndex int,
	stateHistories []*simulator.StateHistory,
	timestepsHistory *simulator.CumulativeTimestepsHistory,
) []float64 {
	// At the termination horizon, freeze the action snapshot too — that
	// way switching the radio after the simulation has ended doesn't
	// silently rewind the trajectory (the reader uses Reset to rerun).
	if terminated(timestepsHistory) {
		return stateHistories[partitionIndex].CopyStateRow(0)
	}
	out := make([]float64, PolicyActionLen)
	actions, ok := params.GetOk("action_state_values")
	if ok {
		for i := 0; i < PolicyActionLen && i < len(actions); i++ {
			out[i] = actions[i]
		}
		return out
	}
	prev := stateHistories[partitionIndex].CopyStateRow(0)
	copy(out, prev[:PolicyActionLen])
	return out
}

// PolicySwitchPrescribingIteration picks one of four prescribing-rate
// strategies based on the upstream `policy_action` partition's state[0]:
//
//	0 = baseline  (constant rate)
//	1 = cycling   (alternate high/low on `cycle_period`)
//	2 = threshold (drop rate when resistance > `threshold`)
//	3 = restriction (linear ramp from initial to target over `ramp_period`)
//
// The threshold strategy reads back from the colonisation partition via the
// `colonisation_partition` param (resolved as a partition index).
//
// State: [cephalosporin_rate].
type PolicySwitchPrescribingIteration struct {
	colonisationPartitionIndex int
}

func (p *PolicySwitchPrescribingIteration) Configure(
	partitionIndex int,
	settings *simulator.Settings,
) {
	p.colonisationPartitionIndex = int(
		settings.Iterations[partitionIndex].Params.Map["colonisation_partition"][0],
	)
}

func (p *PolicySwitchPrescribingIteration) Iterate(
	params *simulator.Params,
	partitionIndex int,
	stateHistories []*simulator.StateHistory,
	timestepsHistory *simulator.CumulativeTimestepsHistory,
) []float64 {
	if terminated(timestepsHistory) {
		return stateHistories[partitionIndex].CopyStateRow(0)
	}
	action := params.Get("policy_action")
	policy := int(math.Round(action[PAIdxPolicy]))
	cyclePeriod := action[PAIdxCyclePeriod]
	threshold := action[PAIdxThreshold]
	rampPeriod := action[PAIdxRampPeriod]

	t := timestepsHistory.Values.AtVec(0)

	switch policy {
	case PolicyCycling:
		if cyclePeriod <= 0 {
			cyclePeriod = DefaultCyclePeriod
		}
		cycleIndex := int(math.Floor(t / cyclePeriod))
		if cycleIndex%2 == 0 {
			return []float64{CyclingHighRate}
		}
		return []float64{CyclingLowRate}
	case PolicyThreshold:
		resistantFraction := stateHistories[p.colonisationPartitionIndex].Values.At(0, 1)
		if resistantFraction > threshold {
			return []float64{ThresholdEscalation}
		}
		return []float64{ThresholdDefault}
	case PolicyRestriction:
		if rampPeriod <= 0 {
			rampPeriod = DefaultRampPeriod
		}
		progress := math.Min(t/rampPeriod, 1.0)
		return []float64{RestrictionInitial + (RestrictionTarget-RestrictionInitial)*progress}
	default:
		return []float64{BaselineRate}
	}
}

// ResistanceRatioIteration is a one-wide passthrough that exposes the
// resistant colonisation fraction (state[1] of the colonisation partition)
// as state[0] so the line chart renderer — which always plots state[0] —
// can render it directly.
type ResistanceRatioIteration struct{}

func (r *ResistanceRatioIteration) Configure(int, *simulator.Settings) {}

func (r *ResistanceRatioIteration) Iterate(
	params *simulator.Params,
	partitionIndex int,
	stateHistories []*simulator.StateHistory,
	timestepsHistory *simulator.CumulativeTimestepsHistory,
) []float64 {
	if terminated(timestepsHistory) {
		return stateHistories[partitionIndex].CopyStateRow(0)
	}
	colonisation := params.Get("colonisation_values")
	return []float64{colonisation[1]}
}

// CumulativeResistantBsiIteration accumulates the resistant BSI counts
// emitted by the infection partition each step. State[0] is the running
// total since simulation start.
type CumulativeResistantBsiIteration struct{}

func (c *CumulativeResistantBsiIteration) Configure(int, *simulator.Settings) {}

func (c *CumulativeResistantBsiIteration) Iterate(
	params *simulator.Params,
	partitionIndex int,
	stateHistories []*simulator.StateHistory,
	timestepsHistory *simulator.CumulativeTimestepsHistory,
) []float64 {
	prev := stateHistories[partitionIndex].Values.At(0, 0)
	if terminated(timestepsHistory) {
		return []float64{prev}
	}
	infection := params.Get("infection_values")
	return []float64{prev + infection[1]}
}

// Canvas geometry shared by the visualization and the bar iterations.
// Keep these in sync with amrdash.go's WithCanvas / AddRectangleSet calls.
const (
	CanvasWidth  = 640
	CanvasHeight = 400

	// Comparison strip panel — five horizontal bars stacked vertically.
	// Rows 0..3 are the four reference policies (slate); row 4 is the
	// live trajectory (action colour, magenta). Sized so each row is
	// taller than the cmpLabelFontSize text rendered alongside it.
	CmpX0       = 220
	CmpY0       = 240
	CmpWidth    = 260
	CmpRowH     = 28
	CmpBarH     = 16
	CmpRefCount = 4
	CmpBarCount = 5
)

// ReferenceCumulativeBSI is the 10-seed mean cumulative resistant BSI over
// 200 timesteps for each of the four canonical policies, computed from
// dat/policy_<name>_seed<0..9>.log. Order is [baseline, cycling, threshold,
// restriction] — the same order the radio buttons present.
//
// Values (±2σ across 10 seeds):
//   baseline    186.8 ± 29.5
//   cycling     165.8 ± 37.5  (−11.2% vs baseline)
//   threshold   152.7 ± 24.4  (−18.3% vs baseline)
//   restriction 152.5 ± 25.7  (−18.4% vs baseline)
//
// Recompute when policy YAML defaults or learned parameters change:
//
//	for p in baseline cycling threshold restriction; do
//	    for s in 0 1 2 3 4 5 6 7 8 9; do
//	        python3 -c "import json;t=0;[t:=t+json.loads(l)['state'][1] \
//	         for l in open('dat/policy_${p}_seed${s}.log') \
//	         if json.loads(l)['partition_name']=='infection'];print(t)"
//	    done
//	done
var ReferenceCumulativeBSI = []float64{186.8, 165.8, 152.7, 152.5}

// barScale picks a denominator that's robust to the live trajectory
// overshooting the worst reference. We scale against the larger of
// (max reference) and (live value), so an out-of-distribution live bar
// doesn't squash the references into invisibility.
func barScale(live float64) float64 {
	maxRef := 0.0
	for _, v := range ReferenceCumulativeBSI {
		if v > maxRef {
			maxRef = v
		}
	}
	if live > maxRef {
		maxRef = live
	}
	if maxRef <= 0 {
		return 1.0
	}
	return maxRef
}

// ReferenceBarsIteration projects the four reference policies' cumulative
// resistant BSI counts onto canvas-space (x, y, w, h) rectangles. These
// are static in the sense that they don't depend on the live simulation,
// but they're rendered every step so the bar scale stays in sync with
// the live bar when the live value exceeds the worst reference.
//
// State width: CmpRefCount * 4.
type ReferenceBarsIteration struct{}

func (r *ReferenceBarsIteration) Configure(int, *simulator.Settings) {}

func (r *ReferenceBarsIteration) Iterate(
	params *simulator.Params,
	partitionIndex int,
	stateHistories []*simulator.StateHistory,
	timestepsHistory *simulator.CumulativeTimestepsHistory,
) []float64 {
	cumulative := params.Get("cumulative_values")[0]
	scale := barScale(cumulative)

	out := make([]float64, CmpRefCount*4)
	for i := 0; i < CmpRefCount; i++ {
		frac := ReferenceCumulativeBSI[i] / scale
		if frac > 1.0 {
			frac = 1.0
		}
		if frac < 0 {
			frac = 0
		}
		out[i*4+0] = float64(CmpX0)
		out[i*4+1] = float64(CmpY0) + float64(i)*float64(CmpRowH)
		out[i*4+2] = frac * float64(CmpWidth)
		out[i*4+3] = float64(CmpBarH)
	}
	return out
}

// UserBarIteration projects the live trajectory's cumulative resistant BSI
// onto a single canvas-space (x, y, w, h) rectangle, sitting one row below
// the four reference bars. Rendered separately so it can use the action
// colour and stand out against the slate references.
//
// State width: 4.
type UserBarIteration struct{}

func (u *UserBarIteration) Configure(int, *simulator.Settings) {}

func (u *UserBarIteration) Iterate(
	params *simulator.Params,
	partitionIndex int,
	stateHistories []*simulator.StateHistory,
	timestepsHistory *simulator.CumulativeTimestepsHistory,
) []float64 {
	cumulative := params.Get("cumulative_values")[0]
	scale := barScale(cumulative)

	frac := cumulative / scale
	if frac > 1.0 {
		frac = 1.0
	}
	if frac < 0 {
		frac = 0
	}
	return []float64{
		float64(CmpX0),
		float64(CmpY0) + float64(CmpRefCount)*float64(CmpRowH),
		frac * float64(CmpWidth),
		float64(CmpBarH),
	}
}

// DisplayCountsIteration is the integer-format readout partition: simulation
// year, current policy index, and the cumulative resistant BSI count. The
// readout template that binds to this partition uses Decimals=0.
//
// State layout:
//
//	state[0] simulation year (timesteps/4)
//	state[1] policy index (rounded to int when rendered)
//	state[2] cumulative resistant BSI
type DisplayCountsIteration struct{}

func (d *DisplayCountsIteration) Configure(int, *simulator.Settings) {}

func (d *DisplayCountsIteration) Iterate(
	params *simulator.Params,
	partitionIndex int,
	stateHistories []*simulator.StateHistory,
	timestepsHistory *simulator.CumulativeTimestepsHistory,
) []float64 {
	action := params.Get("policy_action")
	cumulative := params.Get("cumulative_values")

	t := timestepsHistory.Values.AtVec(0)
	if t > float64(SimSteps) {
		t = float64(SimSteps)
	}
	return []float64{
		t / 4.0,
		math.Round(action[PAIdxPolicy]),
		cumulative[0],
	}
}

// DisplayRatesIteration is the float-format readout partition: current
// cephalosporin rate and resistant colonisation fraction. The readout
// template that binds to this partition uses Decimals=3.
//
// State layout:
//
//	state[0] current cephalosporin rate
//	state[1] current resistance ratio
type DisplayRatesIteration struct{}

func (d *DisplayRatesIteration) Configure(int, *simulator.Settings) {}

func (d *DisplayRatesIteration) Iterate(
	params *simulator.Params,
	partitionIndex int,
	stateHistories []*simulator.StateHistory,
	timestepsHistory *simulator.CumulativeTimestepsHistory,
) []float64 {
	if terminated(timestepsHistory) {
		return stateHistories[partitionIndex].CopyStateRow(0)
	}
	prescribing := params.Get("prescribing_values")
	colonisation := params.Get("colonisation_values")
	return []float64{prescribing[0], colonisation[1]}
}
