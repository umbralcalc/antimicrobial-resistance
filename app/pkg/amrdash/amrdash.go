// Package amrdash is the dexetera dashboard for the AMR
// hospital-prescribing-policy post. The simulator under the hood is the
// project's two-strain colonisation + Poisson infection model, parameterised
// with the inference-derived posterior means used by the offline policy
// evaluation. The controls are a discrete policy selector (snap slider) and
// three tuning sliders; the visualisation is a resistance-ratio line chart,
// a cumulative-resistant-BSI line chart, and a comparison strip that places
// the reader's live trajectory next to the four canonical policy means from
// the offline evaluation.
//
// See app/cmd/amr/{register_step,generate} for the wasm entry-point and the
// codegen that emits the widget shell respectively.
package amrdash

import (
	"fmt"

	"github.com/umbralcalc/dexetera/pkg/dashboard"
)

// actionColorHex is the magenta the Acting on Simulated Systems collection
// uses to signal "this is what the reader controls". Kept in sync with the
// recolouring constant in cmd/amr/generate so the canvas rectangle and the
// HTML slider/radio accents match.
const actionColorHex = "#b0447a"

// cmpLabelFontSize is the font size for the comparison-strip row labels
// ("Baseline", "Cycling", …) and their accompanying cumulative R BSI
// values. The canvas natively renders at 640px but is CSS-scaled to fit
// the panel (typically ~410px wide), so font sizes need to be set in
// canvas-space pixels that look right after that ~0.65× scale-down.
// 18px canvas-space lands at ~12px visible, which dominates the chart
// axes and reads as the primary scalar display in the panel.
const cmpLabelFontSize = 18

const (
	// Top strip: resistance ratio over time (auto-scaling line chart).
	resistanceChartX      = 60
	resistanceChartY      = 36
	resistanceChartWidth  = 520
	resistanceChartHeight = 90

	// Middle strip: cumulative resistant BSI over time (auto-scaling).
	bsiChartX      = 60
	bsiChartY      = 156
	bsiChartWidth  = 520
	bsiChartHeight = 60

	// Bottom panel: comparison strip — five horizontal bars.
	cmpPanelY = CmpY0 - 18
)

// NewConfig returns the dashboard.Config for the AMR widget. Declaration
// order of renderers matters: later renderers draw on top.
func NewConfig() *dashboard.Config {
	vb := dashboard.NewVisualizationBuilder().
		WithCanvas(CanvasWidth, CanvasHeight).
		WithBackground("#fafafa").
		WithUpdateInterval(0).
		// Resistance ratio chart frame (baseline + top border for the panel).
		AddLine("", resistanceChartX, resistanceChartY+resistanceChartHeight,
			resistanceChartX+resistanceChartWidth, resistanceChartY+resistanceChartHeight,
			&dashboard.LineOptions{Color: "#2c3e50", Width: 1}).
		// Resistance ratio line — auto-scaled, blue (simulation output).
		AddLineChart("resistance_ratio",
			resistanceChartX, resistanceChartY,
			resistanceChartWidth, resistanceChartHeight,
			&dashboard.ChartOptions{
				Color:     "#3c78d8",
				LineWidth: 2,
			}).
		// Section divider between the two charts.
		AddLine("",
			resistanceChartX, bsiChartY-12,
			resistanceChartX+resistanceChartWidth, bsiChartY-12,
			&dashboard.LineOptions{Color: "#e3e6ec", Width: 1}).
		// Cumulative BSI chart axis.
		AddLine("",
			bsiChartX, bsiChartY+bsiChartHeight,
			bsiChartX+bsiChartWidth, bsiChartY+bsiChartHeight,
			&dashboard.LineOptions{Color: "#2c3e50", Width: 1}).
		// Cumulative BSI line — same blue, headline outcome variable.
		AddLineChart("cumulative_bsi",
			bsiChartX, bsiChartY, bsiChartWidth, bsiChartHeight,
			&dashboard.ChartOptions{
				Color:     "#3c78d8",
				LineWidth: 2,
			}).
		// Divider above the comparison strip.
		AddLine("",
			resistanceChartX, cmpPanelY,
			resistanceChartX+resistanceChartWidth, cmpPanelY,
			&dashboard.LineOptions{Color: "#e3e6ec", Width: 1}).
		// Reference bars: four canonical policies, in slate grey so they
		// read as "reference distribution" rather than competing for
		// attention with the reader's chosen policy.
		AddRectangleSet("reference_bars", 0, 0, &dashboard.ShapeOptions{
			FillColor: "#7d8aa1",
			Anchor:    "topLeft",
		}).
		// User bar: the live trajectory, drawn last so it sits on top of
		// the row of reference bars. Magenta picks up the action colour
		// to say "this is what your choice produces".
		AddRectangleSet("user_bar", 0, 0, &dashboard.ShapeOptions{
			FillColor: actionColorHex,
			Anchor:    "topLeft",
		})

	// Row labels for the comparison strip — drawn as canvas text in the
	// same dark colour as the chart axes so they're legible on the light
	// background. Right-aligned at CmpX0-8 so they sit just left of each
	// bar. Without these the comparison strip is unreadable: bars without
	// names tell the reader nothing.
	rowLabels := []string{"Baseline", "Cycling", "Threshold", "Restriction", "Your policy"}
	for i, label := range rowLabels {
		color := "#2c3e50"
		if i == CmpRefCount {
			color = actionColorHex
		}
		vb = vb.AddText("", label, CmpX0-8, CmpY0+i*CmpRowH+CmpBarH+2, &dashboard.TextOptions{
			Color:     color,
			FontSize:  cmpLabelFontSize,
			TextAlign: "right",
		})
	}

	// Reference value labels — fixed text rendered to the right of the
	// reference bars at full bar width, so the reader can see each
	// policy's reference cumulative BSI count next to its bar.
	for i, v := range ReferenceCumulativeBSI {
		vb = vb.AddText("",
			fmt.Sprintf("%.0f", v),
			CmpX0+CmpWidth+8, CmpY0+i*CmpRowH+CmpBarH+2,
			&dashboard.TextOptions{
				Color:     "#2c3e50",
				FontSize:  cmpLabelFontSize,
				TextAlign: "left",
			})
	}

	// Live cumulative R BSI value next to the user bar — bound to
	// cumulative_bsi, "{value}" substitutes its state[0] (floored to int)
	// at render time so the reader watches the count grow as the
	// simulation runs and settle at the year-50 horizon.
	vb = vb.AddText("cumulative_bsi", "{value}",
		CmpX0+CmpWidth+8, CmpY0+CmpRefCount*CmpRowH+CmpBarH+2,
		&dashboard.TextOptions{
			Color:     actionColorHex,
			FontSize:  cmpLabelFontSize,
			TextAlign: "left",
		})

	vis := vb.Build()

	cfg := dashboard.NewConfigBuilder("amr").
		WithDescription("Hospital prescribing policy support: pick a stewardship strategy and policy parameters; the simulator (fitted to UKHSA surveillance data) shows the resulting resistance ratio and the cumulative burden of resistant bloodstream infections over 50 years. This is a research model fitted to surveillance data, not a clinical decision aid.").
		WithServerPartition("resistance_ratio").
		WithServerPartition("cumulative_bsi").
		WithServerPartition("reference_bars").
		WithServerPartition("user_bar").
		WithServerPartition("display_counts").
		WithServerPartition("display_rates").
		WithActionStatePartition("policy_action").
		WithVisualization(vis).
		WithSimulation(BuildAMRSimulation)

	cfg = cfg.
		// The policy slider is replaced with radio buttons in generate.go.
		// We keep it in the data model so dexetera's slider→worker action
		// publication mechanism still carries the policy value to wasm.
		// The label below is what generate.go uses to find and hide it.
		WithSlider(dashboard.Slider{
			Name:       "policy",
			Label:      "Policy (radio-controlled)",
			Partition:  "policy_action",
			ValueIndex: PAIdxPolicy,
			Min:        0,
			Max:        3,
			Step:       1,
			Default:    DefaultPolicyIndex,
			Decimals:   0,
		}).
		// Tuning sliders. generate.go wraps each in a conditional block so
		// only the slider relevant to the selected policy is visible.
		WithSlider(dashboard.Slider{
			Name:       "cycle_period",
			Label:      "Cycling period (timesteps)",
			Partition:  "policy_action",
			ValueIndex: PAIdxCyclePeriod,
			Min:        1,
			Max:        52,
			Step:       1,
			Default:    DefaultCyclePeriod,
			Decimals:   0,
		}).
		WithSlider(dashboard.Slider{
			Name:       "threshold",
			Label:      "Threshold resistance fraction",
			Partition:  "policy_action",
			ValueIndex: PAIdxThreshold,
			Min:        0.05,
			Max:        0.5,
			Step:       0.01,
			Default:    DefaultThreshold,
			Decimals:   2,
		}).
		WithSlider(dashboard.Slider{
			Name:       "ramp_period",
			Label:      "Restriction ramp (timesteps)",
			Partition:  "policy_action",
			ValueIndex: PAIdxRampPeriod,
			Min:        4,
			Max:        104,
			Step:       1,
			Default:    DefaultRampPeriod,
			Decimals:   0,
		})

	cfg = cfg.
		WithReadout(dashboard.Readout{
			Partition: "display_counts",
			Template:  fmt.Sprintf("year {v%d} of 50 · cumulative R BSI {v%d}", 0, 2),
			Decimals:  0,
		}).
		WithReadout(dashboard.Readout{
			Partition: "display_rates",
			Template:  fmt.Sprintf("ceph rate {v%d} · R fraction {v%d}", 0, 1),
			Decimals:  3,
		}).
		WithResetButton().
		WithInlineDriver(30)

	return cfg.Build()
}
