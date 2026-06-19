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

// bFieldMapper is the |B| legend: a bright 5-stop jet (blue→cyan→green→yellow→red) ranged
// 0..2 T. Most of the iron runs 1–2 T while the tooth tips saturate past it, so spanning 2 T
// (rather than 1 T) keeps the yoke/tooth contrast instead of clamping the whole core to red;
// the field can spike higher at re-entrant corners, which simply pins to the top color.
func bFieldMapper() wire.GraphicsColorMapper {
	return wire.GraphicsColorMapper{
		Values: []float64{0, 0.5, 1.0, 1.5, 2.0},
		Colors: []float32{
			0.10, 0.20, 0.90, 1, // 0.0 T  blue
			0.00, 0.80, 1.00, 1, // 0.5 T  cyan
			0.20, 0.90, 0.20, 1, // 1.0 T  green
			1.00, 0.90, 0.00, 1, // 1.5 T  yellow
			1.00, 0.10, 0.00, 1, // 2.0 T  red
		},
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
	// Draw the |B| field as a translucent flood plot ON TOP of the analyzed motor geometry so
	// it projects over the stator/rotor/magnets rather than being occluded — the result must be
	// read against the parts it was solved on. 0.6 lets the part edges show through the field.
	if _, err := e.api.Graphics().AddFloodPlot(clientID, coords, f.indices, f.scalars, mapper, floodPlotOpacity); err != nil {
		return "", err
	}
	return clientID, nil
}

// floodPlotOpacity is the |B| overlay's translucency: opaque enough to read the field, sheer
// enough that the motor parts underneath stay visible.
const floodPlotOpacity = 0.6
