// SPDX-License-Identifier: GPL-2.0-only

// Package bridge is the host-facing core of the FEMM magnetics add-in: it turns a
// host body into a 2D magnetostatics study (section → .fem → mesh → solve → |B|
// render) using only the Apache-2.0 oblikovati.org/api client. The cgo c-shared
// shell (../export.go) owns the C ABI; this package owns the FEMM pipeline and stays
// cgo-free so it unit-tests on every platform.
package bridge

import (
	"fmt"
	"os"

	"oblikovati.org/api/client"
)

// HostCaller is the transport the engine talks to the host through — exactly the
// api/client Caller contract, supplied by the cgo shell at Activate (or a fake in
// tests). Keeping it an interface here keeps this package cgo-free and testable.
type HostCaller interface {
	Call(method string, req []byte) ([]byte, error)
}

// Engine runs magnetics studies against a live host.
type Engine struct {
	host    HostCaller
	api     *client.Client
	sourceJ float64 // study excitation: uniform current density (MA/m^2) on region 0
}

// NewEngine binds the engine to the host transport.
func NewEngine(host HostCaller) *Engine {
	return &Engine{host: host, api: client.New(host)}
}

// SetSourceCurrent sets the study excitation — a uniform current density (MA/m^2)
// applied to the first region. Returns the engine for chaining. Interim stand-in
// for the dockable UI's per-region coil currents (see buildProblem).
func (e *Engine) SetSourceCurrent(jr float64) *Engine {
	e.sourceJ = jr
	return e
}

// Notify receives host event bytes. Magnetics studies are user-triggered (a ribbon
// command in a later phase); for now events are accepted and ignored so the C-ABI
// Notify path is wired end to end.
func (e *Engine) Notify(_ []byte) {}

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
