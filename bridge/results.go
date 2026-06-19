// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "oblikovati.org/api/wire"

// field is a scalar field sampled at section vertices — here |B| (flux density,
// tesla). Phase 2b replaces syntheticField with the parsed .ans solution.
type field struct {
	verts   []point2
	indices []int     // triangle indices over verts
	scalars []float64 // |B| per vertex
}

func (f *field) vertexCount() int { return len(f.verts) }

// syntheticField fabricates a smooth |B| falloff across the section so the
// visualization path (color mapper + heatmap) is exercised before the solver lands.
func syntheticField(s *section) *field {
	f := &field{}
	for _, loop := range s.loops {
		for _, p := range loop {
			f.verts = append(f.verts, p)
			f.scalars = append(f.scalars, falloff(p))
		}
	}
	f.indices = fanIndices(len(f.verts))
	return f
}

// falloff is a placeholder radial |B| ~ 1/(1+r²) about the origin.
func falloff(p point2) float64 {
	r2 := p.X*p.X + p.Y*p.Y
	return 1.0 / (1.0 + r2)
}

// fanIndices triangulates n vertices as a fan (placeholder topology for the heatmap).
func fanIndices(n int) []int {
	if n < 3 {
		return nil
	}
	idx := make([]int, 0, (n-2)*3)
	for i := 1; i < n-1; i++ {
		idx = append(idx, 0, i, i+1)
	}
	return idx
}

// bFieldMapper is the |B| legend: blue (low) → red (high), tesla.
func bFieldMapper() wire.GraphicsColorMapper {
	return wire.GraphicsColorMapper{
		Values: []float64{0, 0.5, 1.0},
		Colors: []float32{0, 0, 1, 1, 0, 1, 0, 1, 1, 0, 0, 1},
	}
}

// pushFieldHeatmap renders the field into the host viewport as a client-graphics
// heatmap with a registered |B| color mapper. This is the results-OUT surface FEA
// add-ins rely on (clientGraphics AddHeatmap + RegisterColorMapper) — fully covered.
// GAP #4: the overlay is live-only; it must also persist into .obk (PBI-066) for the
// result to survive save/reload.
func (e *Engine) pushFieldHeatmap(f *field) (string, error) {
	const clientID = "femm.bfield"
	mapper := bFieldMapper()
	if err := e.api.Graphics().RegisterColorMapper("femm.bfield", mapper); err != nil {
		return "", err
	}
	coords := make([]float64, 0, len(f.verts)*3)
	for _, p := range f.verts {
		coords = append(coords, p.X, p.Y, 0)
	}
	if _, err := e.api.Graphics().AddHeatmap(clientID, coords, f.indices, f.scalars, mapper); err != nil {
		return "", err
	}
	return clientID, nil
}
