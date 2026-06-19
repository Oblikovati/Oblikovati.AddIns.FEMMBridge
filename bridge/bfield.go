// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "math"

// cmToMeterGrad converts a gradient taken with centimetre coordinates to per-metre:
// the .ans mesh is in cm (the .fem LengthUnits) while A is in Wb/m, so |B| in tesla
// is the cm-gradient times 100. (Absolute calibration vs FEMM's postprocessor is a
// validation item; the field SHAPE — what the heatmap shows — is exact.)
const cmToMeterGrad = 100.0

// elementB returns the magnetic flux density (Bx, By) in tesla, constant over a
// linear triangle: B = ∇×(A ẑ) = (∂A/∂y, −∂A/∂x), with A linearly interpolated from
// the three nodal potentials. Returns (0,0) for a degenerate (zero-area) element.
func elementB(s *solution, e ansElement) (float64, float64) {
	n0, n1, n2 := s.Nodes[e.P[0]], s.Nodes[e.P[1]], s.Nodes[e.P[2]]
	twoArea := (n1.X-n0.X)*(n2.Y-n0.Y) - (n2.X-n0.X)*(n1.Y-n0.Y)
	if twoArea == 0 {
		return 0, 0
	}
	// ∂A/∂x and ∂A/∂y from the linear shape-function gradients.
	dAdx := ((n1.Y-n2.Y)*n0.A + (n2.Y-n0.Y)*n1.A + (n0.Y-n1.Y)*n2.A) / twoArea
	dAdy := ((n2.X-n1.X)*n0.A + (n0.X-n2.X)*n1.A + (n1.X-n0.X)*n2.A) / twoArea
	return dAdy * cmToMeterGrad, -dAdx * cmToMeterGrad
}

// nodalBMagnitude returns |B| (tesla) at every node, averaged over the elements
// that touch it — turning the per-element constant field into the per-vertex
// scalars the client-graphics heatmap renders.
func nodalBMagnitude(s *solution) []float64 {
	sum := make([]float64, len(s.Nodes))
	count := make([]int, len(s.Nodes))
	for _, e := range s.Elements {
		bx, by := elementB(s, e)
		mag := math.Hypot(bx, by)
		for _, p := range e.P {
			sum[p] += mag
			count[p]++
		}
	}
	for i := range sum {
		if count[i] > 0 {
			sum[i] /= float64(count[i])
		}
	}
	return sum
}

// solutionField turns a parsed .ans into the heatmap field: node positions as
// vertices, the triangle connectivity as indices, and nodal |B| as the scalars.
func solutionField(s *solution) *field {
	verts := make([]point2, len(s.Nodes))
	for i, n := range s.Nodes {
		verts[i] = point2{X: n.X, Y: n.Y}
	}
	indices := make([]int, 0, len(s.Elements)*3)
	for _, e := range s.Elements {
		indices = append(indices, e.P[0], e.P[1], e.P[2])
	}
	return &field{verts: verts, indices: indices, scalars: nodalBMagnitude(s)}
}
