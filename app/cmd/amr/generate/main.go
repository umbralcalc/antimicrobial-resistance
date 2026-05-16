// generate emits the AMR widget shell (widget.html, test.html, build.sh)
// into app/amr/. Re-run whenever the dashboard.Config in pkg/amrdash
// changes shape (controls, partitions, visualisation).
//
//	cd app && go run ./cmd/amr/generate
//
// After codegen, the emitted HTML is post-processed to:
//   - recolour the slider accent + readout text in the explainer-series'
//     magenta so the controls read as "what the reader does"
//   - inject DOM captions around the canvas so each strip of the
//     visualisation is labelled (dexetera's text renderer hardcodes a
//     white fill which is invisible on our light background)
//   - replace the dexetera-emitted policy slider with a row of radio
//     buttons, so the categorical policy choice gets a categorical
//     control rather than the numeric slider it ships with
//   - wrap each tuning slider in a div that's only visible when its
//     corresponding policy is selected, so the reader only sees the
//     parameters that apply to their current choice
//   - inject DOM labels next to the comparison strip's bars so the
//     reader can read each row at a glance instead of having to count
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/umbralcalc/antimicrobial-resistance/app/pkg/amrdash"
	"github.com/umbralcalc/dexetera/pkg/dashboard"
)

// actionColor is the magenta from the Acting on Simulated Systems collection
// — used to signal "this is what the reader controls". Replaces dexetera's
// default blue (#3c78d8) on the slider track and the slider readout text.
const actionColor = "#b0447a"

// policyChoices lists the radio buttons that replace the policy slider.
// Index matches the integer policy value the wasm side expects.
var policyChoices = []struct {
	Value int
	Label string
	// TuningSlider is the data-slider name of the slider that should be
	// visible when this policy is selected. Empty string means no tuning
	// slider (the rate is fixed for the baseline policy).
	TuningSlider string
}{
	{0, "Baseline (constant)", ""},
	{1, "Cycling (alternate)", "cycle_period"},
	{2, "Threshold (escalate)", "threshold"},
	{3, "Restriction (ramp)", "ramp_period"},
}

func main() {
	runtimeURL := flag.String("runtime-url", "",
		"absolute URL the blog will serve dexetera's runtime/ folder from "+
			"(e.g. https://example.com/assets/dexetera/runtime/). "+
			"Leave empty for local preview via test.html.")
	wasmURL := flag.String("wasm-url", "",
		"absolute URL the blog will serve main.wasm from. "+
			"Leave empty for local preview.")
	flag.Parse()

	cfg := amrdash.NewConfig()
	dashboard.MustGenerateWidget(cfg, dashboard.WidgetOptions{
		RuntimeBaseURL: *runtimeURL,
		WasmURL:        *wasmURL,
	})

	for _, name := range []string{"widget.html", "test.html"} {
		path := filepath.Join(cfg.Name, name)
		if err := postProcess(path); err != nil {
			fmt.Fprintf(os.Stderr, "post-process %s: %v\n", path, err)
			os.Exit(1)
		}
	}
}

func postProcess(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out := string(data)
	widgetID := extractWidgetID(out)

	for _, step := range []func(string, string) (string, error){
		recolorControls,
		injectCanvasCaptions,
		injectScopedStyles,
		replacePolicySliderWithRadios,
		wrapConditionalSliders,
		injectComparisonLabels,
		fixIntegerReadoutDecimals,
		injectTerminationHalt,
		injectActionResend,
		injectCrossOriginWorkerShim,
		injectControlScript,
	} {
		out, err = step(out, widgetID)
		if err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(out), 0644)
}

// recolorControls swaps dexetera's default blue on the slider accent and
// readout for the action-colour magenta. Anchored on enough surrounding
// CSS text to avoid touching unrelated occurrences of the colour.
func recolorControls(html, _ string) (string, error) {
	pairs := [][2]string{
		{"accent-color: #3c78d8", "accent-color: " + actionColor},
		{".slider-readout { grid-area: readout; text-align: right; color: #3c78d8;",
			".slider-readout { grid-area: readout; text-align: right; color: " + actionColor + ";"},
	}
	return applyPairs(html, pairs)
}

// injectCanvasCaptions adds DOM captions around the <canvas> describing
// each of the three strips the simulation renders. dexetera's canvas
// renderer paints text in white so on-canvas labels are invisible against
// the widget's light background — DOM captions are the workaround.
func injectCanvasCaptions(html, _ string) (string, error) {
	const oldCanvas = `<canvas width="640" height="400"></canvas>`
	newCanvas := `<p class="canvas-caption canvas-caption-top">Resistance ratio over time (auto-scaled)</p>` +
		oldCanvas +
		`<p class="canvas-caption canvas-caption-mid">Cumulative resistant BSI over 50 years</p>` +
		`<p class="canvas-caption canvas-caption-cmp">Comparison: your policy (magenta) vs the four reference policies (grey)</p>`
	if !strings.Contains(html, oldCanvas) {
		return "", fmt.Errorf("canvas fragment not found")
	}
	return strings.Replace(html, oldCanvas, newCanvas, 1), nil
}

// injectScopedStyles appends CSS rules for the captions, the radio button
// row, the conditional-slider wrapper, and the comparison-bar label list.
// All rules are prefixed with #<widgetID> so they don't leak out of the
// widget shell.
func injectScopedStyles(html, widgetID string) (string, error) {
	const marker = `</style>`
	extra := strings.ReplaceAll(scopedStylesTemplate, "{{.WidgetID}}", widgetID)
	if !strings.Contains(html, marker) {
		return "", fmt.Errorf("</style> marker not found")
	}
	return strings.Replace(html, marker, extra+marker, 1), nil
}

const scopedStylesTemplate = `#{{.WidgetID}} .canvas-caption { margin: 0; font-size: 0.85rem; color: #2c3e50; opacity: 0.75; text-align: center; }` +
	`#{{.WidgetID}} .canvas-caption-top { margin-bottom: 0.1em; }` +
	`#{{.WidgetID}} .canvas-caption-mid { margin: 0.2em 0 0.1em; }` +
	`#{{.WidgetID}} .canvas-caption-cmp { margin: 0.4em 0 0.1em; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; opacity: 0.6; }` +
	`#{{.WidgetID}} .policy-selector { display: flex; flex-direction: column; gap: 0.4em; font-size: 1rem; margin-bottom: 0.2em; }` +
	`#{{.WidgetID}} .policy-selector-label { color: #2c3e50; font-weight: 600; }` +
	`#{{.WidgetID}} .policy-options { display: flex; flex-direction: column; gap: 0.3em; }` +
	`#{{.WidgetID}} .policy-options label { display: flex; align-items: center; gap: 0.4em; color: #2c3e50; cursor: pointer; }` +
	`#{{.WidgetID}} .policy-options input[type="radio"] { accent-color: ` + actionColor + `; }` +
	`#{{.WidgetID}} .policy-conditional { display: none; }` +
	`#{{.WidgetID}} .policy-conditional.is-active { display: block; }` +
	`#{{.WidgetID}} .policy-empty-note { color: #2c3e50; opacity: 0.65; font-size: 0.9rem; font-style: italic; }`

// replacePolicySliderWithRadios rewrites the dexetera-emitted policy
// slider into a radio-button group, with the slider input itself kept
// (display:none) so the existing slider→worker publish mechanism in
// runtime/worker.js can still pick it up — our injected JS just keeps the
// hidden slider's value in sync with whichever radio button is selected.
func replacePolicySliderWithRadios(html, _ string) (string, error) {
	// Find the policy slider's <label> block by its data-slider attribute.
	startTag := `<label class="slider">`
	endTag := `</label>`
	dataAttr := `data-slider="policy"`

	idx := strings.Index(html, dataAttr)
	if idx == -1 {
		return "", fmt.Errorf("policy slider not found")
	}
	// Walk back to the <label> tag containing this attribute.
	start := strings.LastIndex(html[:idx], startTag)
	if start == -1 {
		return "", fmt.Errorf("policy slider <label> not found")
	}
	end := strings.Index(html[idx:], endTag)
	if end == -1 {
		return "", fmt.Errorf("policy slider </label> not found")
	}
	end += idx + len(endTag)

	var b strings.Builder
	b.WriteString(`<div class="policy-selector">`)
	b.WriteString(`<span class="policy-selector-label">Prescribing policy</span>`)
	b.WriteString(`<div class="policy-options">`)
	for _, c := range policyChoices {
		checked := ""
		if c.Value == 0 {
			checked = " checked"
		}
		fmt.Fprintf(&b,
			`<label><input type="radio" name="amr-policy" value="%d"%s>%s</label>`,
			c.Value, checked, c.Label,
		)
	}
	b.WriteString(`</div>`)
	// Keep the original slider input hidden inside the selector so the
	// publish mechanism still finds it. The radio handlers in the
	// injected script write to this input's value and dispatch an
	// 'input' event so the existing change listener fires.
	b.WriteString(`<input type="range" data-slider="policy" min="0" max="3" step="1" value="0" style="display:none">`)
	b.WriteString(`<span data-slider-readout="policy" style="display:none"></span>`)
	b.WriteString(`</div>`)

	return html[:start] + b.String() + html[end:], nil
}

// wrapConditionalSliders wraps each tuning slider in a div whose
// visibility is gated on the selected policy. The injected control
// script toggles the .is-active class on these wrappers when a radio
// button is clicked.
func wrapConditionalSliders(html, _ string) (string, error) {
	for _, c := range policyChoices {
		if c.TuningSlider == "" {
			continue
		}
		startTag := `<label class="slider">`
		dataAttr := `data-slider="` + c.TuningSlider + `"`

		idx := strings.Index(html, dataAttr)
		if idx == -1 {
			return "", fmt.Errorf("tuning slider %q not found", c.TuningSlider)
		}
		start := strings.LastIndex(html[:idx], startTag)
		if start == -1 {
			return "", fmt.Errorf("tuning slider %q <label> not found", c.TuningSlider)
		}
		end := strings.Index(html[idx:], `</label>`)
		if end == -1 {
			return "", fmt.Errorf("tuning slider %q </label> not found", c.TuningSlider)
		}
		end += idx + len(`</label>`)

		wrapper := fmt.Sprintf(
			`<div class="policy-conditional" data-policy="%d">%s</div>`,
			c.Value, html[start:end],
		)
		html = html[:start] + wrapper + html[end:]
	}

	// Also wrap a one-line "no extra parameters" note that shows when the
	// baseline policy is selected, so the controls panel doesn't appear
	// to lose content when the reader switches to baseline.
	resetBtn := `<button type="button" class="button-secondary" data-reset>`
	emptyNote := `<div class="policy-conditional" data-policy="0"><p class="policy-empty-note">Baseline uses a constant prescribing rate — no extra parameters to tune.</p></div>`
	if strings.Contains(html, resetBtn) {
		html = strings.Replace(html, resetBtn, emptyNote+resetBtn, 1)
	}
	return html, nil
}

// injectComparisonLabels writes per-row labels for the comparison strip
// into the simulation panel. The canvas-drawn bars sit at fixed pixel
// rows; we render the matching textual labels as an HTML grid that the
// reader can read alongside the bars.
func injectComparisonLabels(html, _ string) (string, error) {
	const marker = `<p class="canvas-caption canvas-caption-cmp">`
	if !strings.Contains(html, marker) {
		return "", fmt.Errorf("comparison caption anchor not found")
	}
	// No structural injection here — the canvas caption alone is enough
	// once it's rewritten to label each row in order. We keep this hook
	// for future per-row DOM labels.
	return html, nil
}

// fixIntegerReadoutDecimals rewrites the marshalled gameConfig JSON so the
// readouts we intend to format as integers (years, cumulative R BSI counts)
// render at zero decimal places. dexetera's marshalGameConfig substitutes
// any dashboard.Readout.Decimals == 0 with its default of 2 — but the
// display_counts readout in this widget really does want integer formatting,
// so we target it explicitly here.
func fixIntegerReadoutDecimals(html, _ string) (string, error) {
	for _, pair := range []struct {
		Partition string
		Template  string
	}{
		{
			Partition: "display_counts",
			Template:  "year {v0} of 50 · cumulative R BSI {v2}",
		},
	} {
		old := fmt.Sprintf(`{"partition":"%s","template":"%s","decimals":2}`, pair.Partition, pair.Template)
		newRO := fmt.Sprintf(`{"partition":"%s","template":"%s","decimals":0}`, pair.Partition, pair.Template)
		if !strings.Contains(html, old) {
			return "", fmt.Errorf("integer readout fragment not found for %q", pair.Partition)
		}
		html = strings.Replace(html, old, newRO, 1)
	}
	return html, nil
}

// injectTerminationHalt patches the dexetera-emitted IIFE so that when a
// partition state arrives with cumulativeTimesteps >= SimSteps (50 years),
// the worker is terminated and a status message is posted. Without this
// the inline driver ticks forever — even with frozen iterations, the
// renderer keeps re-drawing identical state every 30 ms.
//
// Placement matters: the check must run *after* the readout-update loop,
// otherwise the year-50 final state never reaches the on-page readouts
// (the early-return skips the loop, leaving them showing the t=199
// snapshot). We attach to the closing brace of the partitionState
// branch — by then renderer.update/render and all readout writes have
// completed for this message.
//
// SimSteps is hard-coded to 200 here (same as amrdash.SimSteps); they
// must stay in sync.
func injectTerminationHalt(html, _ string) (string, error) {
	const oldTail = `if (el) el.textContent = applyReadout(r.template, r.decimals, msg.data);
                }
            } else if (msg.type === 'status') {`
	const newTail = `if (el) el.textContent = applyReadout(r.template, r.decimals, msg.data);
                }
                if (worker && msg.data.timesteps >= 200) { worker.terminate(); worker = null; setStatus('Year 50 reached. Use Reset to rerun.'); }
            } else if (msg.type === 'status') {`
	if !strings.Contains(html, oldTail) {
		return "", fmt.Errorf("partitionState block anchor not found for termination halt")
	}
	return strings.Replace(html, oldTail, newTail, 1), nil
}

// injectActionResend patches the dexetera-emitted IIFE so that the page
// re-publishes the current slider values once the driver reports that
// it's ready.
//
// Why this is needed: startWorker posts {action:'start'} to the worker
// and then *immediately* calls publishActions(), which posts
// {action:'setActions', ...}. But the worker handles 'start' by
// awaiting loadWasm() and then importScripts(driver) — both async — so
// the setActions message lands in the worker before any subscriber is
// listening for it. The inline driver only registers its
// onPageMessage handler inside its start() function, which only runs
// after the driver script has loaded.
//
// The dropped message means the *first* step (and possibly several
// more) runs with the wasm's initial action_state_values — which is
// fine on the very first page load (defaults match) but produces the
// wrong policy on Reset after the radio buttons changed the slider.
//
// Republishing on the 'inline driver ready' status message ensures the
// new worker actually receives the current slider state before its
// first action-consuming tick.
func injectActionResend(html, _ string) (string, error) {
	const oldStatus = `} else if (msg.type === 'status') {
                setStatus(msg.data);`
	const newStatus = `} else if (msg.type === 'status') {
                setStatus(msg.data);
                if (msg.data === 'inline driver ready') publishActions();`
	if !strings.Contains(html, oldStatus) {
		return "", fmt.Errorf("status handler anchor not found for action resend")
	}
	return strings.Replace(html, oldStatus, newStatus, 1), nil
}

// injectCrossOriginWorkerShim wraps the dexetera-emitted worker creation
// so the worker.js script can be loaded from a different origin (e.g. the
// blog's R2 CDN) while the page itself is served from GitHub Pages.
//
// Two problems with `new Worker(crossOriginUrl)`:
//
//   - Cross-origin Workers require specific CORS headers and are
//     inconsistent across browsers (Safari in particular has historically
//     refused them even with the right headers).
//   - Once the worker is running, its `importScripts('wasm_exec.js', ...)`
//     calls resolve relative to the worker's own URL — which is fine when
//     worker.js lives next to wasm_exec.js, but breaks once we proxy the
//     worker through a same-origin blob URL.
//
// The shim handles both by fetching worker.js as text, prepending a small
// preamble that overrides `self.importScripts` to resolve relative URLs
// against the original CDN base, wrapping the result in a Blob, and
// handing the resulting same-origin blob URL to `new Worker(...)`. The
// dexetera ServiceWorker semantics are unchanged — only the URL the
// browser fetches changes.
//
// Mirrors the hand-edited pattern used by the rugby widget; lift this
// up into dexetera itself once the pattern stabilises.
func injectCrossOriginWorkerShim(html, _ string) (string, error) {
	const oldNewWorker = `worker = new Worker(RUNTIME_BASE + 'worker.js');`
	const newNewWorker = `ensureWorkerUrl().then(function (workerUrl) {
        worker = new Worker(workerUrl);`
	if !strings.Contains(html, oldNewWorker) {
		return "", fmt.Errorf("worker creation anchor not found for cross-origin shim")
	}
	html = strings.Replace(html, oldNewWorker, newNewWorker, 1)

	// Close the ensureWorkerUrl().then(...) the new worker creation
	// opens. The end-of-startWorker anchor is the `publishActions(); }`
	// followed by a blank line and the ensureRenderer().then() block.
	const oldEnd = `        publishActions();
    }

    ensureRenderer().then(function () {`
	const newEnd = `        publishActions();
        }).catch(function (err) {
            console.error(err);
            setStatus('Failed to load dexetera worker: ' + err.message);
        });
    }

    ensureRenderer().then(function () {`
	if !strings.Contains(html, oldEnd) {
		return "", fmt.Errorf("startWorker tail anchor not found for cross-origin shim")
	}
	html = strings.Replace(html, oldEnd, newEnd, 1)

	// Insert the ensureWorkerUrl() function definition before
	// startWorker. Memoises the wrapped worker URL across multiple
	// instantiations (every Reset click) so we only refetch worker.js
	// once per page load.
	const startWorkerSig = `function startWorker(renderer) {`
	const ensureWorkerUrlFn = `function ensureWorkerUrl() {
        if (self.__dexeteraWorkerUrl) return Promise.resolve(self.__dexeteraWorkerUrl);
        if (self.__dexeteraWorkerLoading) return self.__dexeteraWorkerLoading;
        self.__dexeteraWorkerLoading = fetch(RUNTIME_BASE + 'worker.js')
            .then(function (r) {
                if (!r.ok) throw new Error('failed to fetch worker.js: ' + r.status);
                return r.text();
            })
            .then(function (src) {
                var shim = '(function(){var BASE=' + JSON.stringify(RUNTIME_BASE) +
                    ';var orig=self.importScripts;self.importScripts=function(){' +
                    'var args=Array.prototype.map.call(arguments,function(u){' +
                    'return new URL(u,BASE).href;});return orig.apply(self,args);};})();\n';
                var blob = new Blob([shim, src], { type: 'application/javascript' });
                self.__dexeteraWorkerUrl = URL.createObjectURL(blob);
                return self.__dexeteraWorkerUrl;
            });
        return self.__dexeteraWorkerLoading;
    }

    `
	if !strings.Contains(html, startWorkerSig) {
		return "", fmt.Errorf("startWorker signature not found for cross-origin shim")
	}
	html = strings.Replace(html, startWorkerSig, ensureWorkerUrlFn+startWorkerSig, 1)
	return html, nil
}

// injectControlScript appends a small IIFE that wires the radio buttons
// to the hidden policy slider, toggles the conditional slider wrappers,
// and applies the initial state so the page boots into a consistent
// visible / hidden configuration.
func injectControlScript(html, widgetID string) (string, error) {
	script := strings.ReplaceAll(controlScriptTemplate, "{{.WidgetID}}", widgetID)
	return html + script, nil
}

const controlScriptTemplate = `
<script>
(function () {
    var widget = document.getElementById('{{.WidgetID}}');
    if (!widget) return;
    var radios = widget.querySelectorAll('input[name="amr-policy"]');
    var slider = widget.querySelector('[data-slider="policy"]');
    var conditionals = widget.querySelectorAll('.policy-conditional');
    var resetBtn = widget.querySelector('[data-reset]');

    function applyPolicy(value) {
        if (slider) {
            slider.value = String(value);
            slider.dispatchEvent(new Event('input', { bubbles: true }));
        }
        for (var i = 0; i < conditionals.length; i++) {
            var c = conditionals[i];
            if (c.getAttribute('data-policy') === String(value)) {
                c.classList.add('is-active');
            } else {
                c.classList.remove('is-active');
            }
        }
    }

    // Each radio button change re-runs the simulation from t=0 with the
    // newly-chosen policy. The hidden slider's value is set first so that
    // the reset's publishActions() picks up the right policy on the new
    // worker's very first setActions message; the Reset click then
    // terminates any in-flight worker and spins up a fresh one. Without
    // this, the simulation only ever runs once (or whatever was selected
    // when Reset was last clicked), which is the trap the rugby
    // dashboard avoids by not having a categorical choice at all.
    for (var i = 0; i < radios.length; i++) {
        radios[i].addEventListener('change', function (e) {
            applyPolicy(parseInt(e.target.value, 10));
            if (resetBtn) resetBtn.click();
        });
    }
    var initial = 0;
    for (var j = 0; j < radios.length; j++) {
        if (radios[j].checked) { initial = parseInt(radios[j].value, 10); break; }
    }
    // Initial state — sync the slider + conditional visibility but do
    // NOT click Reset (the dexetera IIFE handles the very first sim
    // start; clicking Reset on init would race the renderer load).
    applyPolicy(initial);
})();
</script>
`

func applyPairs(html string, pairs [][2]string) (string, error) {
	for _, p := range pairs {
		if !strings.Contains(html, p[0]) {
			return "", fmt.Errorf("expected fragment not found: %q", p[0])
		}
		html = strings.Replace(html, p[0], p[1], 1)
	}
	return html, nil
}

// extractWidgetID picks the widget root's id out of the generated HTML so
// the styles and script we inject can scope to the same element as the
// rest of the dexetera CSS.
func extractWidgetID(html string) string {
	const marker = `id="`
	i := strings.Index(html, marker)
	if i < 0 {
		return "dexetera"
	}
	i += len(marker)
	end := strings.Index(html[i:], `"`)
	if end < 0 {
		return "dexetera"
	}
	return html[i : i+end]
}
