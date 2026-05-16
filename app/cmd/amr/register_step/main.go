//go:build js && wasm

// register_step is the AMR widget compiled as a WebAssembly module.
// It registers `stepSimulation` on the JS global and blocks forever so
// the Go runtime stays alive to service per-step calls from
// dexetera's runtime/worker.js.
//
// Build with the codegen-emitted app/amr/build.sh or directly:
//
//	GOOS=js GOARCH=wasm go build -o app/amr/src/main.wasm ./app/cmd/amr/register_step
package main

import (
	"github.com/umbralcalc/antimicrobial-resistance/app/pkg/amrdash"
	"github.com/umbralcalc/dexetera/pkg/simio"
)

func main() {
	simio.RegisterStep(amrdash.NewConfig())
}
