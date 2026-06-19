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
	prob := &femProblem{Frequency: 0, Precision: 1e-8, Planar: true,
		Boundaries: []femBoundary{{BdryType: 0}}}
	// Material 0 is air; one material per region follows (iron or a directed magnet).
	prob.Materials = []femMaterial{{MuX: 1, MuY: 1}}
	// Label 0 is the default air label (covers every unseeded element). Region labels
	// follow, each pointing at its 1-based material.
	prob.Labels = []femLabel{{X: 0, Y: 0, BlockType: 1, InGroup: 0}}

	mesh := &polyMesh{}
	addOuterBoundary(mesh, d.StatorOuterDiaMM/2*mmToCm*outerBoundaryFactor)
	prob.Labels[0] = airDefaultLabel(d.StatorOuterDiaMM / 2 * mmToCm * (1 + outerBoundaryFactor) / 2)

	for i, r := range d.Regions {
		prob.Materials = append(prob.Materials, regionMaterialFEM(r))
		matIdx := i + 2 // 1-based material index (material[i+1])
		for _, loop := range r.Loops {
			addClosedLoop(mesh, loop, 0) // interior boundary, no BC
		}
		area := regionMeshArea(r)
		mesh.Regions = append(mesh.Regions, polyRegion{
			X: r.Seed[0] * mmToCm, Y: r.Seed[1] * mmToCm,
			Label: len(prob.Labels) + 1, MaxArea: area, // 1-based label index
		})
		prob.Labels = append(prob.Labels, femLabel{
			X: r.Seed[0] * mmToCm, Y: r.Seed[1] * mmToCm, BlockType: matIdx, MaxArea: area,
		})
	}
	return prob, mesh
}

// RunMotorStudy is the Motor Designer → FEMM interop: read a motor cross-section
// descriptor (the hand-off the Motor Designer emits), build + solve its multi-region
// magnetostatic field with the vendored toolchain, and push the |B| heatmap to the
// host viewport.
func (e *Engine) RunMotorStudy(descriptorPath string) (*StudyResult, error) {
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
	return e.renderAns(ans)
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
