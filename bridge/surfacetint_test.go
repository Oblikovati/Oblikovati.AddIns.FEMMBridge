// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"os"
	"path/filepath"
	"testing"

	"oblikovati.org/api/types"
)

// rampField is a 2 cm square whose |B| ramps with x+y, so a sampler/tint can be checked against
// known values at the corners.
func rampField() *field {
	return &field{
		verts:   []point2{{X: 0, Y: 0}, {X: 2, Y: 0}, {X: 2, Y: 2}, {X: 0, Y: 2}},
		indices: []int{0, 1, 2, 0, 2, 3},
		scalars: []float64{0, 2, 4, 2},
	}
}

func TestFieldSamplerReturnsValueAtVertices(t *testing.T) {
	s, err := newFieldSampler(rampField())
	if err != nil {
		t.Fatalf("newFieldSampler: %v", err)
	}
	for _, c := range []struct {
		x, y, want float64
	}{{0, 0, 0}, {2, 0, 2}, {2, 2, 4}, {0, 2, 2}} {
		if got := s.at(c.x, c.y); got != c.want {
			t.Errorf("at(%g,%g) = %g, want %g", c.x, c.y, got, c.want)
		}
	}
	if got := s.at(-10, -10); got != 0 {
		t.Errorf("at(outside) = %g, want 0", got)
	}
}

// writeOBJ writes a one-triangle OBJ over the ramp square's corners and returns its path.
func writeOBJ(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tri.obj")
	const body = "v 0 0 0\nv 2 0 0\nv 2 2 0\nvn 0 0 1\nvn 0 0 1\nvn 0 0 1\nf 1//1 2//2 3//3\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write obj: %v", err)
	}
	return path
}

func TestParseOBJFlattensCorners(t *testing.T) {
	m, err := parseOBJ(writeOBJ(t))
	if err != nil {
		t.Fatalf("parseOBJ: %v", err)
	}
	if len(m.coords) != 9 || len(m.normals) != 9 { // 3 corners × xyz
		t.Fatalf("coords=%d normals=%d, want 9 each", len(m.coords), len(m.normals))
	}
	if m.normals[2] != 1 { // first corner normal +z
		t.Errorf("normal z = %g, want 1", m.normals[2])
	}
}

func TestTintedNodeSamplesFieldPerVertex(t *testing.T) {
	s, _ := newFieldSampler(rampField())
	m, _ := parseOBJ(writeOBJ(t))
	node := tintedNode("Stator", m, s, bFieldMapper())
	if len(node.Primitives) != 1 {
		t.Fatalf("primitives = %d, want 1", len(node.Primitives))
	}
	p := node.Primitives[0]
	if p.ColorBinding != string(types.GraphicsColorPerVertex) || p.ColorMapper == nil {
		t.Errorf("primitive = %+v, want per-vertex mapper", p)
	}
	// Corners (0,0)→0, (2,0)→2, (2,2)→4 of the ramp field.
	want := []float64{0, 2, 4}
	for i, w := range want {
		if p.Scalars[i] != w {
			t.Errorf("scalar[%d] = %g, want %g (field not sampled onto the surface)", i, p.Scalars[i], w)
		}
	}
}

func TestDetectScaleToCm(t *testing.T) {
	mm := &objMesh{coords: []float64{30, 0, 0}} // tens of units ⇒ millimetres
	cm := &objMesh{coords: []float64{3, 0, 0}}  // a few units ⇒ centimetres
	if got := detectScaleToCm(mm); got != 0.1 {
		t.Errorf("mm scale = %g, want 0.1", got)
	}
	if got := detectScaleToCm(cm); got != 1.0 {
		t.Errorf("cm scale = %g, want 1.0", got)
	}
}

func TestLegendNodesCarryScaleEndpointsAndPeak(t *testing.T) {
	nodes := legendNodes(bFieldMapper(), rampField(), 0, 2.0)
	texts := map[string]bool{}
	hasBar := false
	for _, n := range nodes {
		for _, p := range n.Primitives {
			switch p.Kind {
			case string(types.GraphicsTriangles):
				if p.OnTop && p.ColorMapper != nil {
					hasBar = true
				}
			case string(types.GraphicsText):
				texts[p.Text] = true
			}
		}
	}
	if !hasBar {
		t.Error("legend missing an on-top gradient bar")
	}
	if !texts["0.0"] || !texts["2.0"] {
		t.Errorf("legend value labels = %v, want min 0.0 and max 2.0", texts)
	}
	if !texts["peak 4.00 T"] {
		t.Errorf("legend labels = %v, want the true peak 'peak 4.00 T'", texts)
	}
}
