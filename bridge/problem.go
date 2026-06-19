// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "fmt"

// dirichletA0 is the index-0 boundary condition every study installs: prescribed
// A = 0, the standard "field decays to nothing" outer boundary. Segments carrying
// it get .poly marker -(0+2) = -2.
const dirichletA0Marker = -2

// boundaryMeshArea bounds element size as a fraction of the section bounding box so
// the mesh resolves the field without a per-problem tuning knob (cm^2).
const defaultRegionArea = 0.25

// buildProblem maps the host section + region materials to a complete FEMM problem:
// the .fem physics (femProblem) and the triangle PSLG (polyMesh). First cut: the
// section's first loop is the outer boundary (A=0), enclosing one region per
// material. Holes and multi-loop regions are a follow-up — see PIPELINE.md.
func buildProblem(s *section, regions []regionMaterial) (*femProblem, *polyMesh, error) {
	if len(s.loops) == 0 {
		return nil, nil, fmt.Errorf("buildProblem: section has no boundary loops")
	}
	outer := s.loops[0]
	if len(outer) < 3 {
		return nil, nil, fmt.Errorf("buildProblem: outer loop has %d points, need >= 3", len(outer))
	}
	mesh := boundaryPolyMesh(outer)
	prob := &femProblem{
		Frequency:  0,
		Precision:  1e-8,
		Planar:     true,
		Boundaries: []femBoundary{{BdryType: 0}},
		Materials:  femMaterials(regions),
		Labels:     []femLabel{{X: mesh.Regions[0].X, Y: mesh.Regions[0].Y, BlockType: 1, MaxArea: defaultRegionArea}},
	}
	return prob, mesh, nil
}

// boundaryPolyMesh turns a closed boundary loop into a PSLG: its points become
// nodes, consecutive points become A=0 boundary segments, and the centroid seeds a
// single meshed region with block-label attribute 1.
func boundaryPolyMesh(loop []point2) *polyMesh {
	n := len(loop)
	nodes := make([]polyNode, n)
	segs := make([]polySegment, n)
	for i, p := range loop {
		nodes[i] = polyNode{X: p.X, Y: p.Y}
		segs[i] = polySegment{N0: i, N1: (i + 1) % n, Marker: dirichletA0Marker}
	}
	cx, cy := centroid(loop)
	return &polyMesh{
		Nodes:    nodes,
		Segments: segs,
		Regions:  []polyRegion{{X: cx, Y: cy, Label: 1, MaxArea: defaultRegionArea}},
	}
}

// femMaterials maps host region materials to .fem block materials. Without a host
// magnetic-property group (API gap #3) permeability falls back to air (μ_r=1);
// conductivity carries through. There is no current source from host geometry yet.
func femMaterials(regions []regionMaterial) []femMaterial {
	if len(regions) == 0 {
		return []femMaterial{{MuX: 1, MuY: 1}}
	}
	ms := make([]femMaterial, len(regions))
	for i, r := range regions {
		ms[i] = femMaterial{MuX: r.muR, MuY: r.muR, Hc: r.coercivity, Sigma: r.sigma}
	}
	return ms
}

// centroid is the vertex average — inside a convex-ish loop, a valid region seed.
func centroid(loop []point2) (float64, float64) {
	var sx, sy float64
	for _, p := range loop {
		sx += p.X
		sy += p.Y
	}
	n := float64(len(loop))
	return sx / n, sy / n
}
