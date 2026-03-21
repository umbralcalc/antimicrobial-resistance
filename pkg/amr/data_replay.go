package amr

import (
	"encoding/json"
	"os"

	"github.com/umbralcalc/stochadex/pkg/general"
)

// NewDataReplayIteration loads a JSON file containing [][]float64 data and
// returns a FromStorageIteration that cycles through it for maxSteps entries.
// This allows a finite dataset (e.g. 40 quarterly observations) to be replayed
// over a longer simulation (e.g. 1000 inference iterations).
func NewDataReplayIteration(path string, maxSteps int) *general.FromStorageIteration {
	raw, err := os.ReadFile(path)
	if err != nil {
		panic("failed to read data file: " + err.Error())
	}
	var orig [][]float64
	if err := json.Unmarshal(raw, &orig); err != nil {
		panic("failed to parse data file: " + err.Error())
	}
	if len(orig) == 0 {
		panic("data file is empty: " + path)
	}
	data := make([][]float64, maxSteps)
	for i := 0; i < maxSteps; i++ {
		data[i] = orig[i%len(orig)]
	}
	return &general.FromStorageIteration{Data: data}
}
