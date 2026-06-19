// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// currentSquareProblem is a 1 cm square of a μ_r=1 material carrying a uniform
// current density, with A=0 on all four edges. The vector potential bulges to a
// maximum at the centre, so |B| = |∇A| is zero at the centre and rises toward the
// edges — a non-trivial field with a known qualitative shape.
func currentSquareProblem() (*femProblem, *polyMesh) {
	prob := &femProblem{
		Frequency:  0,
		Precision:  1e-8,
		Planar:     true,
		Boundaries: []femBoundary{{BdryType: 0}}, // A = 0
		Materials:  []femMaterial{{MuX: 1, MuY: 1, Jr: 1}},
		Labels:     []femLabel{{X: 0.5, Y: 0.5, BlockType: 1, MaxArea: 0.002}},
	}
	mesh := &polyMesh{
		Nodes: []polyNode{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}},
		Segments: []polySegment{
			{N0: 0, N1: 1, Marker: -2}, {N0: 1, N1: 2, Marker: -2},
			{N0: 2, N1: 3, Marker: -2}, {N0: 3, N1: 0, Marker: -2},
		},
		Regions: []polyRegion{{X: 0.5, Y: 0.5, Label: 1, MaxArea: 0.002}},
	}
	return prob, mesh
}

// TestFkernSolvesCurrentSquare runs the whole real pipeline (write .fem/.poly/.pbc →
// triangle → fkern → parse .ans → |B|) and checks the physics: a finite, non-zero
// field that vanishes near the centre and peaks toward the boundary.
func TestFkernSolvesCurrentSquare(t *testing.T) {
	requireSolver(t)
	bins, err := findSolverBinaries()
	if err != nil {
		t.Fatalf("findSolverBinaries: %v", err)
	}
	prob, mesh := currentSquareProblem()
	ans, err := runSolve(t.TempDir(), "coil", prob, mesh, bins)
	if err != nil {
		t.Fatalf("runSolve: %v", err)
	}
	sol, err := parseAns(ans)
	if err != nil {
		t.Fatalf("parseAns: %v", err)
	}
	if len(sol.Nodes) < 50 {
		t.Fatalf("mesh too coarse: %d nodes", len(sol.Nodes))
	}

	mags := nodalBMagnitude(sol)
	maxB, centreB := fieldStats(sol, mags)
	if maxB <= 0 || math.IsNaN(maxB) || math.IsInf(maxB, 0) {
		t.Fatalf("|B| max = %g, want a finite positive field", maxB)
	}
	// The centre |B| should be well below the peak (A is flat at its centre maximum).
	if centreB >= maxB {
		t.Errorf("centre |B| (%g) should be below the peak |B| (%g)", centreB, maxB)
	}
	t.Logf("current-square solve: %d nodes, |B| max=%.4g T, centre=%.4g T", len(sol.Nodes), maxB, centreB)
}

// fieldStats returns the peak |B| and the |B| at the node nearest the square centre.
func fieldStats(s *solution, mags []float64) (maxB, centreB float64) {
	bestDist := math.Inf(1)
	for i, n := range s.Nodes {
		if mags[i] > maxB {
			maxB = mags[i]
		}
		if d := math.Hypot(n.X-0.5, n.Y-0.5); d < bestDist {
			bestDist, centreB = d, mags[i]
		}
	}
	return maxB, centreB
}
