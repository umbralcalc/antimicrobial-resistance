package amrdash

import (
	"sync"
	"testing"

	"github.com/umbralcalc/stochadex/pkg/simulator"
)

// shortGen returns the dashboard's generator with termination clamped to
// `steps` so individual tests stay sub-second.
func shortGen(steps int) *simulator.ConfigGenerator {
	gen := BuildAMRSimulation()
	gen.SetSimulation(&simulator.SimulationConfig{
		OutputCondition: &simulator.EveryStepOutputCondition{},
		TerminationCondition: &simulator.NumberOfStepsTerminationCondition{
			MaxNumberOfSteps: steps,
		},
		TimestepFunction: &simulator.ConstantTimestepFunction{Stepsize: 1.0},
		InitTimeValue:    0.0,
	})
	return gen
}

func TestBuildAMRSimulation(t *testing.T) {
	t.Run("harness", func(t *testing.T) {
		settings, implementations := shortGen(20).GenerateConfigs()
		if err := simulator.RunWithHarnesses(settings, implementations); err != nil {
			t.Fatalf("harness failed: %v", err)
		}
	})

	t.Run("invariants", func(t *testing.T) {
		settings, implementations := shortGen(50).GenerateConfigs()
		store := simulator.NewStateTimeStorage()
		implementations.OutputFunction = &simulator.StateTimeStorageOutputFunction{Store: store}
		coordinator := simulator.NewPartitionCoordinator(settings, implementations)
		var wg sync.WaitGroup
		for !coordinator.ReadyToTerminate() {
			coordinator.Step(&wg)
		}

		// resistance_ratio stays inside [0, 1].
		rr := store.GetValues("resistance_ratio")
		if len(rr) == 0 {
			t.Fatal("no resistance_ratio output")
		}
		for _, row := range rr {
			if row[0] < 0 || row[0] > 1 {
				t.Errorf("resistance ratio out of [0,1]: %v", row[0])
				break
			}
		}

		// cumulative_bsi is monotonic non-decreasing.
		cb := store.GetValues("cumulative_bsi")
		for i := 1; i < len(cb); i++ {
			if cb[i][0] < cb[i-1][0] {
				t.Errorf("cumulative BSI decreased at step %d: %v -> %v", i, cb[i-1][0], cb[i][0])
				break
			}
		}

		// reference_bars and user_bar: 4-aligned, never negative width.
		ref := store.GetValues("reference_bars")
		refLast := ref[len(ref)-1]
		if len(refLast) != CmpRefCount*4 {
			t.Errorf("reference_bars width=%d, want %d", len(refLast), CmpRefCount*4)
		}
		for i := 0; i+3 < len(refLast); i += 4 {
			if refLast[i+2] < 0 || refLast[i+3] < 0 {
				t.Errorf("reference bar %d has negative size", i/4)
			}
		}
		user := store.GetValues("user_bar")
		userLast := user[len(user)-1]
		if len(userLast) != 4 {
			t.Errorf("user_bar width=%d, want 4", len(userLast))
		}
		if userLast[2] < 0 || userLast[3] < 0 {
			t.Errorf("user bar has negative size: w=%v h=%v", userLast[2], userLast[3])
		}
	})
}
