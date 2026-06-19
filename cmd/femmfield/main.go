// SPDX-License-Identifier: GPL-2.0-only

// Command femmfield solves a magnetostatic square study with the vendored fkern and
// writes the exact clientGraphics payload the add-in would push to the host — the |B|
// heatmap as a wire.SetClientGraphicsArgs JSON. A live driver (or the mcp-bridge's
// set_client_graphics tool) replays that payload into the running viewport, so the
// FEMM field can be live-tested through the bridge without a render dependency here.
//
//	OBK_FEMM_BIN=vendor-src/femm/build go run ./cmd/femmfield -out /tmp/femm-field.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"oblikovati.org/api/wire"
	"oblikovati.org/femm-bridge/bridge"
)

// captureHost is a HostCaller that records the clientGraphics.set request the engine
// emits instead of forwarding it to a live host — the payload we want to persist.
type captureHost struct{ heatmap []byte }

func (c *captureHost) Call(method string, req []byte) ([]byte, error) {
	if method == wire.MethodClientGraphicsSet {
		c.heatmap = append([]byte(nil), req...)
	}
	return []byte("{}"), nil
}

func main() {
	out := flag.String("out", "/tmp/femm-field.json", "clientGraphics payload output (JSON)")
	side := flag.Float64("side", 4, "square side in cm")
	current := flag.Float64("current", 2, "current density MA/m^2")
	flag.Parse()
	if err := run(*out, *side, *current); err != nil {
		fmt.Fprintln(os.Stderr, "femmfield:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "wrote", *out)
}

// run solves the study through the real engine, capturing the heatmap payload.
func run(out string, side, current float64) error {
	h := &captureHost{}
	res, err := bridge.NewEngine(h).RunSquareStudy(side, current)
	if err != nil {
		return fmt.Errorf("run study: %w", err)
	}
	if h.heatmap == nil {
		return fmt.Errorf("engine pushed no clientGraphics payload")
	}
	if err := os.WriteFile(out, indentJSON(h.heatmap), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	fmt.Printf("solved %d-vertex |B| field (client %q)\n", res.FieldVertices, res.GraphicsClientID)
	return nil
}

// indentJSON pretty-prints the payload, falling back to the raw bytes on error.
func indentJSON(b []byte) []byte {
	var v any
	if json.Unmarshal(b, &v) != nil {
		return b
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return b
	}
	return pretty
}
