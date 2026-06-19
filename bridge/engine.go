// SPDX-License-Identifier: GPL-2.0-only

// Package bridge is the host-facing core of the FEMM magnetics add-in: it turns a
// host body into a 2D magnetostatics study (section → .fem → mesh → solve → |B|
// render) using only the Apache-2.0 oblikovati.org/api client. The cgo c-shared
// shell (../export.go) owns the C ABI; this package owns the FEMM pipeline and stays
// cgo-free so it unit-tests on every platform.
package bridge

import (
	"fmt"

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
	host HostCaller
	api  *client.Client
}

// NewEngine binds the engine to the host transport.
func NewEngine(host HostCaller) *Engine {
	return &Engine{host: host, api: client.New(host)}
}

// Notify receives host event bytes. Magnetics studies are user-triggered (a ribbon
// command in a later phase); for now events are accepted and ignored so the C-ABI
// Notify path is wired end to end.
func (e *Engine) Notify(_ []byte) {}

// StudyResult summarizes one magnetics run.
type StudyResult struct {
	FemPath          string
	PointCount       int
	RegionCount      int
	FieldVertices    int
	GraphicsClientID string
}

// RunStudy is the end-to-end add-in flow for one body: section it, resolve region
// materials, emit a FEMM .fem, and render the |B| field as client graphics. Phase 2
// swaps the synthetic field for the vendored solver's parsed .ans solution.
func (e *Engine) RunStudy(bodyIndex int) (*StudyResult, error) {
	section, err := e.extractSection(bodyIndex)
	if err != nil {
		return nil, fmt.Errorf("section body %d: %w", bodyIndex, err)
	}
	regions, err := e.resolveRegionMaterials(section)
	if err != nil {
		return nil, fmt.Errorf("region materials: %w", err)
	}
	femPath, pts, err := emitFEM(section, regions)
	if err != nil {
		return nil, fmt.Errorf("emit .fem: %w", err)
	}
	field := syntheticField(section)
	clientID, err := e.pushFieldHeatmap(field)
	if err != nil {
		return nil, fmt.Errorf("push |B| heatmap: %w", err)
	}
	return &StudyResult{
		FemPath:          femPath,
		PointCount:       pts,
		RegionCount:      len(regions),
		FieldVertices:    field.vertexCount(),
		GraphicsClientID: clientID,
	}, nil
}
