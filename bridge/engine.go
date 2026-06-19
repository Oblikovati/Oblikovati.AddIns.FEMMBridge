// SPDX-License-Identifier: GPL-2.0-only

// Package bridge is the host-facing core of the FEMM magnetics add-in: it turns a
// host body into a 2D magnetostatics study (section → .fem → mesh → solve → |B|
// render) using only the Apache-2.0 oblikovati.org/api client. The cgo c-shared
// shell (../export.go) owns the C ABI; this package owns the FEMM pipeline and stays
// cgo-free so it unit-tests on every platform.
package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"oblikovati.org/api/client"
	"oblikovati.org/api/wire"
)

// HostCaller is the transport the engine talks to the host through — exactly the
// api/client Caller contract, supplied by the cgo shell at Activate (or a fake in
// tests). Keeping it an interface here keeps this package cgo-free and testable.
type HostCaller interface {
	Call(method string, req []byte) ([]byte, error)
}

// Engine runs magnetics studies against a live host.
type Engine struct {
	host     HostCaller
	api      *client.Client
	sourceJ  float64 // study excitation: uniform current density (MA/m^2) on region 0
	scaleMax float64 // |B| color-scale ceiling (tesla) for the tint + legend

	mu      sync.Mutex // guards running + the sim params
	running bool       // a study is in flight (coalesces overlapping command triggers)
}

// NewEngine binds the engine to the host transport with the default simulation parameters.
func NewEngine(host HostCaller) *Engine {
	return &Engine{host: host, api: client.New(host), scaleMax: defaultScaleMax}
}

// defaultScaleMax is the default |B| color-scale ceiling (tesla); see fieldScaleMax.
const defaultScaleMax = 2.0

// SetSourceCurrent sets the study excitation — a uniform current density (MA/m^2)
// applied to the first region. Returns the engine for chaining. Interim stand-in
// for the dockable UI's per-region coil currents (see buildProblem).
func (e *Engine) SetSourceCurrent(jr float64) *Engine {
	e.sourceJ = jr
	return e
}

// RunStudyCommandID is the host command the add-in registers; firing it (a ribbon click or
// the MCP bridge's execute_command) runs the magnetics study on the active motor.
const RunStudyCommandID = "FEMM.RunStudy"

// RegisterCommands registers the magnetics study command with the host so it is invokable the
// same way a ribbon click is — including over the MCP bridge's execute_command. The host action
// is a no-op; executing the command fires command.started, which Notify turns into a study run.
func (e *Engine) RegisterCommands() error {
	_, err := e.api.Commands().Create(wire.CreateCommandArgs{
		ID:          RunStudyCommandID,
		DisplayName: "Run Magnetics Study",
		Category:    "FEMM",
		Tooltip:     "Solve the |B| field for the active motor and tint its surfaces by flux density.",
	})
	return err
}

// Setup performs the one-time host-facing initialization: register the study command and show
// the simulation-parameters panel. It MUST NOT run on the host's session goroutine (host calls
// there block until the frame loop drains the dispatcher, deadlocking the head) — the cgo shell
// runs it on its own goroutine.
func (e *Engine) Setup() error {
	if err := e.RegisterCommands(); err != nil {
		return err
	}
	_, err := e.ShowPanel()
	return err
}

// Notify receives host event bytes. A command.started carrying RunStudyCommandID runs the
// magnetics study on a SEPARATE goroutine — never inline, because Notify is invoked on the
// host's session goroutine and a host call from there blocks until the frame loop drains the
// dispatcher (which cannot happen while we're inside it), deadlocking every host call. A guard
// coalesces overlapping triggers so one study is in flight at a time.
func (e *Engine) Notify(ev []byte) {
	var hdr struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(ev, &hdr) != nil {
		return
	}
	switch hdr.Type {
	case wire.EventCommandStarted:
		var c struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(ev, &c) == nil && c.Command == RunStudyCommandID {
			e.launchStudy()
		}
	case wire.EventPanelValueChanged:
		var p struct {
			WindowId  string `json:"windowId"`
			ControlId string `json:"controlId"`
			Value     string `json:"value"`
		}
		// Editing a sim parameter only mutates engine state (no host call) — safe inline.
		if json.Unmarshal(ev, &p) == nil && p.WindowId == FEMMPanelID {
			e.applyPanelEdit(p.ControlId, p.Value)
		}
	}
}

// launchStudy starts one study goroutine, coalescing overlapping triggers, and reports the
// outcome to the host status bar so a failed solve is visible rather than silently empty.
func (e *Engine) launchStudy() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.mu.Unlock()

	go func() {
		defer func() {
			e.mu.Lock()
			e.running = false
			e.mu.Unlock()
		}()
		if _, err := e.RunMotorStudyOnHost(); err != nil {
			e.reportStatus("FEMM study failed: " + err.Error())
			return
		}
		e.reportStatus("FEMM study complete: |B| field tinted onto the motor.")
	}()
}

// reportStatus surfaces a study's outcome on the host status bar (best-effort: a status
// failure must not mask the study result).
func (e *Engine) reportStatus(msg string) { _, _ = e.api.Status().SetText(msg) }

// StudyResult summarizes one magnetics run.
type StudyResult struct {
	AnsPath          string // the fkern .ans solution file
	PointCount       int
	RegionCount      int
	FieldVertices    int
	GraphicsClientID string
}

// RunStudy is the end-to-end add-in flow for one body: section it, resolve region
// materials, build + solve the FEMM problem (write .fem/.poly/.pbc, mesh with
// triangle, solve with fkern), and render the |B| field as client graphics. The
// .ans solution is parsed into the real field by parseAns (the synthetic field is
// the interim render until that lands).
func (e *Engine) RunStudy(bodyIndex int) (*StudyResult, error) {
	section, err := e.extractSection(bodyIndex)
	if err != nil {
		return nil, fmt.Errorf("section body %d: %w", bodyIndex, err)
	}
	regions, err := e.resolveRegionMaterials(section)
	if err != nil {
		return nil, fmt.Errorf("region materials: %w", err)
	}
	ansPath, err := e.solveStudy(section, regions)
	if err != nil {
		return nil, err
	}
	res, err := e.renderAns(ansPath)
	if err != nil {
		return nil, err
	}
	res.PointCount = section.pointCount()
	res.RegionCount = len(regions)
	return res, nil
}

// RunSquareStudy solves the validated side×side cm square with current density
// sourceJ and pushes the real |B| heatmap to the host viewport. It exercises the
// full live path (transport → solve → parse → clientGraphics) on a known problem,
// independent of host body sectioning (which awaits a planar-section host op).
func (e *Engine) RunSquareStudy(side, sourceJ float64) (*StudyResult, error) {
	prob, mesh := squareProblem(side, sourceJ)
	bins, err := findSolverBinaries()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "femm-square")
	if err != nil {
		return nil, fmt.Errorf("study workdir: %w", err)
	}
	ansPath, err := runSolve(dir, "square", prob, mesh, bins)
	if err != nil {
		return nil, err
	}
	return e.renderAns(ansPath)
}

// renderAns parses a .ans solution into the |B| field and pushes it as a host
// heatmap, returning the run summary.
func (e *Engine) renderAns(ansPath string) (*StudyResult, error) {
	sol, err := parseAns(ansPath)
	if err != nil {
		return nil, fmt.Errorf("parse solution: %w", err)
	}
	field := solutionField(sol)
	clientID, err := e.pushFieldHeatmap(field)
	if err != nil {
		return nil, fmt.Errorf("push |B| heatmap: %w", err)
	}
	return &StudyResult{AnsPath: ansPath, FieldVertices: field.vertexCount(), GraphicsClientID: clientID}, nil
}

// solveStudy builds the FEMM problem from the section + materials and runs the
// vendored toolchain, returning the .ans path. The work files live in a temp dir.
func (e *Engine) solveStudy(s *section, regions []regionMaterial) (string, error) {
	prob, mesh, err := buildProblem(s, regions, e.sourceJ)
	if err != nil {
		return "", err
	}
	bins, err := findSolverBinaries()
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("", "femm-study")
	if err != nil {
		return "", fmt.Errorf("study workdir: %w", err)
	}
	return runSolve(dir, "study", prob, mesh, bins)
}
