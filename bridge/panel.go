// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"strconv"
	"strings"

	"oblikovati.org/api/client"
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// FEMMPanelID is the stable dockable-window id the FEMM add-in owns.
const FEMMPanelID = "com.oblikovati.femm-bridge.panel"

// ShowPanel creates (or replaces) the FEMM simulation-parameters dockable window: the editable
// study settings plus a Run button. Edits arrive as panel.valueChanged events (handlePanelEdit).
func (e *Engine) ShowPanel() (wire.OKResult, error) {
	e.mu.Lock()
	sourceJ, scaleMax := e.sourceJ, e.scaleMax
	e.mu.Unlock()
	return e.api.DockableWindows().Set(wire.DockableWindowSpec{
		ID:      FEMMPanelID,
		Title:   "FEMM Magnetics",
		Dock:    types.DockRight,
		Visible: true,
		Controls: []wire.PanelControlSpec{
			client.PanelLabel("hdr", "— Simulation parameters —"),
			client.PanelValueEditor("scale_max", "Scale max |B| (T)", strconv.FormatFloat(scaleMax, 'g', -1, 64)),
			client.PanelTextBox("source_j", "Coil current J (MA/m²)", strconv.FormatFloat(sourceJ, 'g', -1, 64)),
			client.PanelSeparator(),
			client.PanelButton("run", "Run Magnetics Study", RunStudyCommandID),
		},
	})
}

// applyPanelEdit writes one edited simulation parameter back into the engine, keyed by control id.
func (e *Engine) applyPanelEdit(controlID, value string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch controlID {
	case "scale_max":
		e.scaleMax = simNum(value, e.scaleMax)
	case "source_j":
		e.sourceJ = simNum(value, e.sourceJ)
	}
}

// simNum reads the leading number from a form value (e.g. "2 T" → 2), keeping fallback when the
// field is empty or half-typed.
func simNum(value string, fallback float64) float64 {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return fallback
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return fallback
	}
	return v
}
