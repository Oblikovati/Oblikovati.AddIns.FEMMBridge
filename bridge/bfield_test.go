// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestElementBLinearField checks B = ∇×(A ẑ) on an exactly-known linear potential
// A(x,y) = 2x + 3y over a unit triangle: ∂A/∂x=2, ∂A/∂y=3, so B = (∂A/∂y, −∂A/∂x) =
// (3, −2), scaled cm→m (×100).
func TestElementBLinearField(t *testing.T) {
	s := &solution{
		Nodes: []ansNode{
			{X: 0, Y: 0, A: 0}, {X: 1, Y: 0, A: 2}, {X: 0, Y: 1, A: 3},
		},
		Elements: []ansElement{{P: [3]int{0, 1, 2}}},
	}
	bx, by := elementB(s, s.Elements[0])
	if math.Abs(bx-300) > 1e-9 || math.Abs(by-(-200)) > 1e-9 {
		t.Errorf("elementB = (%g, %g), want (300, -200)", bx, by)
	}
}

// TestElementBUniformPotential: a constant A has zero field.
func TestElementBUniformPotential(t *testing.T) {
	s := &solution{
		Nodes:    []ansNode{{X: 0, Y: 0, A: 5}, {X: 2, Y: 0, A: 5}, {X: 0, Y: 2, A: 5}},
		Elements: []ansElement{{P: [3]int{0, 1, 2}}},
	}
	if bx, by := elementB(s, s.Elements[0]); bx != 0 || by != 0 {
		t.Errorf("uniform A must give B=0, got (%g, %g)", bx, by)
	}
}

// TestSolutionFieldShape verifies the heatmap field carries one scalar per node,
// triangle connectivity, and nodal |B| equal to the (single element's) magnitude.
func TestSolutionFieldShape(t *testing.T) {
	s := &solution{
		Nodes:    []ansNode{{X: 0, Y: 0, A: 0}, {X: 1, Y: 0, A: 2}, {X: 0, Y: 1, A: 3}},
		Elements: []ansElement{{P: [3]int{0, 1, 2}}},
	}
	f := solutionField(s)
	if len(f.verts) != 3 || len(f.scalars) != 3 {
		t.Fatalf("verts=%d scalars=%d, want 3 each", len(f.verts), len(f.scalars))
	}
	if len(f.indices) != 3 {
		t.Errorf("indices=%d, want 3 (one triangle)", len(f.indices))
	}
	want := math.Hypot(300, 200)
	for i, v := range f.scalars {
		if math.Abs(v-want) > 1e-6 {
			t.Errorf("nodal |B|[%d] = %g, want %g", i, v, want)
		}
	}
}
