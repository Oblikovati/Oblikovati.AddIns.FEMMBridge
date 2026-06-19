// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"math"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// A result is unreadable without its scale, so the add-in draws its own |B| legend: a vertical
// gradient bar (sampled through the same mapper as the tint) just outside the field, with the
// scale's min/max value labels and the true peak |B|. It is built as on-top client graphics so
// it stays legible over the geometry; the labels are world-anchored text the head projects.

const (
	legendBarWidth = 0.7  // cm
	legendSegments = 24   // gradient quads up the bar
	legendGapCm    = 0.8  // gap between the field edge and the bar
	legendLabelPad = 0.25 // gap between the bar and its value labels
)

// fieldBounds returns the field's xy extent (cm).
func fieldBounds(f *field) (minX, minY, maxX, maxY float64) {
	minX, minY, maxX, maxY = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range f.verts {
		minX, maxX = math.Min(minX, p.X), math.Max(maxX, p.X)
		minY, maxY = math.Min(minY, p.Y), math.Max(maxY, p.Y)
	}
	return minX, minY, maxX, maxY
}

// peakOf returns the largest finite |B| in the field (tesla).
func peakOf(f *field) float64 {
	var peak float64
	for _, v := range f.scalars {
		if v > peak && !math.IsInf(v, 0) && !math.IsNaN(v) {
			peak = v
		}
	}
	return peak
}

// legendNodes builds the |B| color-scale legend for a field: a gradient bar spanning
// [scaleMin, scaleMax] (the mapper's range) placed just to the right of the field, labelled
// with the scale endpoints and the actual peak |B|. The bar is sized to the field height so it
// reads at the same scale as the result.
func legendNodes(mapper wire.GraphicsColorMapper, f *field, scaleMin, scaleMax float64) []wire.GraphicsNode {
	_, minY, maxX, maxY := fieldBounds(f)
	x0 := maxX + legendGapCm
	yLo, yHi := minY, maxY

	bar := legendBar(mapper, x0, yLo, yHi, scaleMin, scaleMax)

	labelX := x0 + legendBarWidth + legendLabelPad
	nodes := []wire.GraphicsNode{
		bar,
		textNode("legend.title", "|B| (T)", x0, yHi+0.7),
		textNode("legend.max", fmt.Sprintf("%.1f", scaleMax), labelX, yHi),
		textNode("legend.mid", fmt.Sprintf("%.1f", (scaleMin+scaleMax)/2), labelX, (yLo+yHi)/2),
		textNode("legend.min", fmt.Sprintf("%.1f", scaleMin), labelX, yLo),
		textNode("legend.peak", fmt.Sprintf("peak %.2f T", peakOf(f)), x0, yLo-0.7),
	}
	return nodes
}

// legendBar is the vertical gradient quad strip from scaleMin (bottom) to scaleMax (top),
// colored per vertex through the mapper, drawn on top of the model.
func legendBar(mapper wire.GraphicsColorMapper, x0, yLo, yHi, scaleMin, scaleMax float64) wire.GraphicsNode {
	var coords, scalars []float64
	var indices []int
	for i := 0; i < legendSegments; i++ {
		t0, t1 := float64(i)/legendSegments, float64(i+1)/legendSegments
		y0, y1 := yLo+(yHi-yLo)*t0, yLo+(yHi-yLo)*t1
		base := len(coords) / 3
		coords = append(coords,
			x0, y0, 0, x0+legendBarWidth, y0, 0,
			x0+legendBarWidth, y1, 0, x0, y1, 0)
		v0, v1 := scaleMin+(scaleMax-scaleMin)*t0, scaleMin+(scaleMax-scaleMin)*t1
		scalars = append(scalars, v0, v0, v1, v1)
		indices = append(indices, base, base+1, base+2, base, base+2, base+3)
	}
	return wire.GraphicsNode{Id: "legend.bar", Primitives: []wire.GraphicsPrimitive{{
		Kind: string(types.GraphicsTriangles), Coordinates: coords, Indices: indices,
		Scalars: scalars, ColorMapper: &mapper, ColorBinding: string(types.GraphicsColorPerVertex),
		OnTop: true,
	}}}
}

// textNode is a single world-anchored legend label.
func textNode(id, text string, x, y float64) wire.GraphicsNode {
	return wire.GraphicsNode{Id: id, Primitives: []wire.GraphicsPrimitive{{
		Kind: string(types.GraphicsText), Text: text, Anchor: []float64{x, y, 0},
		Color: []float32{0.95, 0.95, 0.95, 1},
	}}}
}
