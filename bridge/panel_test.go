// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"testing"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// fakePanelHost records the last dockable-window spec set by the engine.
type fakePanelHost struct{ window wire.DockableWindowSpec }

func (h *fakePanelHost) Call(method string, req []byte) ([]byte, error) {
	if method == wire.MethodDockableWindowsSet {
		var args wire.SetDockableWindowArgs
		_ = json.Unmarshal(req, &args)
		h.window = args.Window
	}
	return []byte("{}"), nil
}

func TestShowPanelHasEditableSimParamsAndRun(t *testing.T) {
	h := &fakePanelHost{}
	e := NewEngine(h)
	if _, err := e.ShowPanel(); err != nil {
		t.Fatalf("ShowPanel: %v", err)
	}
	kinds := map[types.PanelControlKind]int{}
	var runCmd string
	for _, c := range h.window.Controls {
		kinds[c.Kind]++
		if c.Kind == types.PanelButton {
			runCmd = c.CommandID
		}
	}
	if kinds[types.PanelValueEditor]+kinds[types.PanelTextBox] < 2 {
		t.Errorf("want editable sim-param fields, got %+v", kinds)
	}
	if runCmd != RunStudyCommandID {
		t.Errorf("run button command = %q, want %q", runCmd, RunStudyCommandID)
	}
}

func TestNotifyPanelEditUpdatesSimParams(t *testing.T) {
	e := NewEngine(&fakePanelHost{})
	ev := `{"type":"` + wire.EventPanelValueChanged + `","windowId":"` + FEMMPanelID + `","controlId":"scale_max","value":"3.5 T"}`
	e.Notify([]byte(ev))
	if e.scaleMax != 3.5 {
		t.Errorf("scale_max = %g, want 3.5 (unit suffix stripped)", e.scaleMax)
	}
	// An edit to another window must be ignored.
	e.Notify([]byte(`{"type":"` + wire.EventPanelValueChanged + `","windowId":"other","controlId":"scale_max","value":"9"}`))
	if e.scaleMax != 3.5 {
		t.Errorf("scale_max = %g, an edit to a foreign window must not apply", e.scaleMax)
	}
}
