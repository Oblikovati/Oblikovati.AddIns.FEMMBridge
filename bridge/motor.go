// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// mmToCm converts the descriptor's millimetres to the pipeline's centimetres (the
// .fem LengthUnits), so the solved field is consistent with bfield's cm→m scaling.
const mmToCm = 0.1

// outerBoundaryFactor places the A=0 far boundary this many times the stator outer
// radius out, so the field decays before the boundary.
const outerBoundaryFactor = 1.4

// MotorRegion is one solid region of a motor cross-section handed off from the Motor
// Designer add-in: its boundary loops (mm) become mesh segments, Seed (a point inside
// the region's material area, mm) tags the region with its material, and the magnetic
// data drives the solve. Iron sets only MuR; a magnet adds coercivity Hc (A/m) at
// HcAngleDeg (the magnetisation direction).
type MotorRegion struct {
	Name       string         `json:"name"`
	Loops      [][][2]float64 `json:"loops"`
	Seed       [2]float64     `json:"seed"`
	MuR        float64        `json:"muR"`
	HcAm       float64        `json:"hcAm,omitempty"`
	HcAngleDeg float64        `json:"hcAngleDeg,omitempty"`
}

// MotorDescriptor is the FEMM-ready motor cross-section the Motor Designer emits: the
// solid regions (stator iron, rotor iron, per-pole magnets) plus the stator outer
// diameter that sizes the air domain. Everything not inside a region's seeded area —
// the air gap, the shaft, the surrounding air — solves as the default air material.
type MotorDescriptor struct {
	StatorOuterDiaMM float64       `json:"statorOuterDiaMM"`
	GlueGapMM        float64       `json:"glueGapMM,omitempty"` // magnet↔iron bond line; 0 ⇒ default
	Regions          []MotorRegion `json:"regions"`
}

// readMotorDescriptor parses a descriptor JSON file (the Motor Designer ↔ FEMM bridge
// hand-off).
func readMotorDescriptor(path string) (*MotorDescriptor, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read motor descriptor %s: %w", path, err)
	}
	var d MotorDescriptor
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("parse motor descriptor %s: %w", path, err)
	}
	if len(d.Regions) == 0 {
		return nil, fmt.Errorf("motor descriptor %s has no regions", path)
	}
	return &d, nil
}

// buildMotorProblem assembles a multi-region magnetostatic problem from the motor
// descriptor: every region's loops become mesh segments, an A=0 circle encloses the
// domain, each region seeds a labelled material area, and air is the default label so
// the gaps (air gap, shaft, surrounding air) solve without explicit regions.
func buildMotorProblem(d *MotorDescriptor) (*femProblem, *polyMesh) {
	gap := d.GlueGapMM
	if gap <= 0 {
		gap = defaultGlueGapMM
	}
	mesh := &polyMesh{}
	addOuterBoundary(mesh, d.StatorOuterDiaMM/2*mmToCm*outerBoundaryFactor)
	placed := placeMotorRegions(mesh, d.Regions, gap)
	mergeCoincidentNodes(mesh) // collapse any exactly-coincident boundary nodes

	prob := &femProblem{Frequency: 0, Precision: 1e-8, Planar: true,
		Boundaries: []femBoundary{{BdryType: 0}}}
	// Material 0 is air, the default block that covers every unseeded element (air gap,
	// shaft, surrounding air). Each placed region's material + label follows it.
	prob.Materials = []femMaterial{{MuX: 1, MuY: 1}}
	prob.Labels = []femLabel{airDefaultLabel(d.StatorOuterDiaMM / 2 * mmToCm * (1 + outerBoundaryFactor) / 2)}
	for i, pr := range placed {
		prob.Materials = append(prob.Materials, pr.mat)
		mesh.Regions = append(mesh.Regions, polyRegion{
			X: pr.seed[0] * mmToCm, Y: pr.seed[1] * mmToCm, Label: i + 2, MaxArea: pr.area,
		})
		prob.Labels = append(prob.Labels, femLabel{
			X: pr.seed[0] * mmToCm, Y: pr.seed[1] * mmToCm, BlockType: i + 2, MaxArea: pr.area,
		})
	}
	return prob, mesh
}

// placedRegion is one labelled material area: an interior seed (mm), the material, and
// the element-area bound (cm²).
type placedRegion struct {
	seed [2]float64
	mat  femMaterial
	area float64
}

// placeMotorRegions adds every descriptor region's boundaries to the mesh and returns
// the labelled material areas (iron, or a directed magnet). A surface magnet is glued
// to the rotor iron, so its descriptor boundary coincides with the rotor's — and a
// coincident boundary (same curve, different sampling) trips fkern's singular flag.
// Each magnet is therefore inset toward its seed by the glue gap, leaving a thin
// separation that solves as the default air.
//
// The bond is magnetically epoxy ≈ μr 1 (the inset air gap gives the same field), so
// it is not meshed as its own material here; it is carried as descriptor metadata
// (MotorDescriptor.GlueGapMM) for the future thermal-simulation add-in, whose
// constitutive problem DOES depend on the epoxy's low thermal conductivity.
func placeMotorRegions(mesh *polyMesh, regions []MotorRegion, gap float64) []placedRegion {
	placed := make([]placedRegion, 0, len(regions))
	for _, r := range regions {
		for _, loop := range r.Loops {
			if r.HcAm > 0 {
				loop = insetLoop(loop, r.Seed, gap)
			}
			addClosedLoop(mesh, loop, 0)
		}
		placed = append(placed, placedRegion{seed: r.Seed, mat: regionMaterialFEM(r), area: regionMeshArea(r)})
	}
	return placed
}

// insetLoop shrinks a loop toward seed by gapMM (each vertex moves gapMM closer to the
// seed) — the magnet's glue-gap separation from the iron it is bonded to.
func insetLoop(loopMM [][2]float64, seed [2]float64, gapMM float64) [][2]float64 {
	out := make([][2]float64, len(loopMM))
	for i, p := range loopMM {
		dx, dy := p[0]-seed[0], p[1]-seed[1]
		d := math.Hypot(dx, dy)
		if d <= gapMM {
			out[i] = p
			continue
		}
		f := (d - gapMM) / d
		out[i] = [2]float64{seed[0] + dx*f, seed[1] + dy*f}
	}
	return out
}

// defaultGlueGapMM is the magnet↔iron bond line used when the descriptor leaves
// GlueGapMM unset.
const defaultGlueGapMM = 0.05

// nodeMergeTolCm: nodes closer than this (cm) are the same mesh node. Surface magnets
// are glued to the rotor, so the descriptor's magnet-inner and rotor-outer arcs lie on
// top of each other; merging them to one shared edge is what keeps fkern's matrix
// non-singular at any pole count.
const nodeMergeTolCm = 1e-4

// mergeCoincidentNodes snaps coincident nodes to a single index and drops the resulting
// duplicate boundary segments, so adjacent regions share their common edge.
func mergeCoincidentNodes(mesh *polyMesh) {
	canon := snapNodes(mesh)
	mesh.Segments = dedupSegments(mesh.Segments, canon)
}

// snapNodes replaces the mesh nodes with the de-duplicated set (nodes within
// nodeMergeTolCm collapse to one) and returns each old index's canonical index.
func snapNodes(mesh *polyMesh) []int {
	index := map[[2]int64]int{}
	canon := make([]int, len(mesh.Nodes))
	var unique []polyNode
	for i, n := range mesh.Nodes {
		k := [2]int64{int64(math.Round(n.X / nodeMergeTolCm)), int64(math.Round(n.Y / nodeMergeTolCm))}
		j, ok := index[k]
		if !ok {
			j = len(unique)
			index[k] = j
			unique = append(unique, n)
		}
		canon[i] = j
	}
	mesh.Nodes = unique
	return canon
}

// dedupSegments remaps segments to canonical node indices and drops collapsed and
// duplicate (undirected) edges.
func dedupSegments(in []polySegment, canon []int) []polySegment {
	seen := map[[2]int]bool{}
	out := in[:0:0]
	for _, s := range in {
		a, b := canon[s.N0], canon[s.N1]
		if a == b {
			continue
		}
		key := [2]int{a, b}
		if a > b {
			key = [2]int{b, a}
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, polySegment{N0: a, N1: b, Marker: s.Marker})
	}
	return out
}

// RunMotorStudy is the Motor Designer → FEMM interop: read a motor cross-section
// descriptor (the hand-off the Motor Designer emits), build + solve its multi-region
// magnetostatic field with the vendored toolchain, and push the |B| heatmap to the
// host viewport.
func (e *Engine) RunMotorStudy(descriptorPath string) (*StudyResult, error) {
	field, err := e.solveMotorField(descriptorPath)
	if err != nil {
		return nil, err
	}
	clientID, err := e.pushFieldHeatmap(field)
	if err != nil {
		return nil, fmt.Errorf("push |B| heatmap: %w", err)
	}
	return &StudyResult{FieldVertices: field.vertexCount(), GraphicsClientID: clientID}, nil
}

// solveMotorField runs the descriptor's multi-region magnetostatic problem through the
// vendored toolchain and returns the parsed |B| field (vertices in cm, scalars in tesla).
// Both the flood-plot render and the surface-tint render share this solve.
func (e *Engine) solveMotorField(descriptorPath string) (*field, error) {
	desc, err := readMotorDescriptor(descriptorPath)
	if err != nil {
		return nil, err
	}
	prob, mesh := buildMotorProblem(desc)
	bins, err := findSolverBinaries()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "femm-motor")
	if err != nil {
		return nil, fmt.Errorf("motor study workdir: %w", err)
	}
	ans, err := runSolve(dir, "motor", prob, mesh, bins)
	if err != nil {
		return nil, err
	}
	sol, err := parseAns(ans)
	if err != nil {
		return nil, fmt.Errorf("parse solution: %w", err)
	}
	return solutionField(sol), nil
}

// regionMaterialFEM maps a motor region to its FEMM block material (iron is μr-only; a
// magnet carries directed coercivity).
func regionMaterialFEM(r MotorRegion) femMaterial {
	return femMaterial{MuX: r.MuR, MuY: r.MuR, Hc: r.HcAm, HcAngle: r.HcAngleDeg}
}

// airDefaultLabel is the default air block (IsDefault) placed in the surrounding air,
// so every element the region seeds don't claim solves as air.
func airDefaultLabel(rAir float64) femLabel {
	return femLabel{X: rAir, Y: 0, BlockType: 1, InGroup: 0, IsDefault: true}
}

// addClosedLoop appends a region boundary (mm) as nodes + closing segments (cm), all
// carrying marker (0 for an interior boundary, dirichletA0Marker for the A=0 edge).
func addClosedLoop(mesh *polyMesh, loopMM [][2]float64, marker int) {
	base := len(mesh.Nodes)
	n := len(loopMM)
	for _, p := range loopMM {
		mesh.Nodes = append(mesh.Nodes, polyNode{X: p[0] * mmToCm, Y: p[1] * mmToCm})
	}
	for i := 0; i < n; i++ {
		mesh.Segments = append(mesh.Segments, polySegment{N0: base + i, N1: base + (i+1)%n, Marker: marker})
	}
}

// addOuterBoundary appends a circle of radius rCm (cm) as the A=0 far boundary.
func addOuterBoundary(mesh *polyMesh, rCm float64) {
	const seg = 96
	loop := make([][2]float64, seg)
	for i := 0; i < seg; i++ {
		a := 2 * math.Pi * float64(i) / seg
		loop[i] = [2]float64{rCm * math.Cos(a) / mmToCm, rCm * math.Sin(a) / mmToCm} // back to mm for addClosedLoop
	}
	addClosedLoop(mesh, loop, dirichletA0Marker)
}

// regionMeshArea bounds element size in a region from its loop extent, so a thin
// magnet or yoke still gets several elements across.
func regionMeshArea(r MotorRegion) float64 {
	minX, minY, maxX, maxY := loopExtentCm(r.Loops)
	w, h := maxX-minX, maxY-minY
	if w <= 0 || h <= 0 {
		return defaultRegionArea
	}
	a := w * h / 400
	if a <= 0 {
		return defaultRegionArea
	}
	return a
}

// loopExtentCm returns the bounding box (cm) of a region's loops (mm in, cm out).
func loopExtentCm(loops [][][2]float64) (minX, minY, maxX, maxY float64) {
	first := true
	for _, loop := range loops {
		for _, p := range loop {
			x, y := p[0]*mmToCm, p[1]*mmToCm
			if first {
				minX, minY, maxX, maxY, first = x, y, x, y, false
				continue
			}
			minX, minY = minf(minX, x), minf(minY, y)
			maxX, maxY = maxf(maxX, x), maxf(maxY, y)
		}
	}
	return
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
