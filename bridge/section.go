// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"

	"oblikovati.org/api/wire"
)

// meshTolerance is the chord tolerance (cm, the host DB unit) for the section
// tessellation — fine enough that polyline boundaries track curved edges.
const meshTolerance = 0.05

// point2 is a vertex in the section plane (cm, host DB units).
type point2 struct{ X, Y float64 }

// section is the 2D magnetics domain extracted from a host body: closed boundary
// loops plus the per-region face partition the FEMM block labels attach to.
type section struct {
	bodyIndex int
	loops     [][]point2 // boundary polylines (from Body.CalculateStrokes)
	regions   []int      // facet count per B-rep face (FacetSetResult.IndexCountPerFace)
}

// extractSection pulls the body's boundary loops and face partition over api/client.
//
// It leans on two host capabilities an FEA add-in always needs:
//   - Body.CalculateStrokes → boundary polylines (GAP #2: no per-edge key comes
//     back, so a boundary condition can only bind to the whole loop, not one edge);
//   - Body.CalculateFacets → IndexCountPerFace, the per-B-rep-face partition used to
//     map each region to its material.
func (e *Engine) extractSection(bodyIndex int) (*section, error) {
	strokes, err := e.api.Body().CalculateStrokes(bodyIndex, meshTolerance)
	if err != nil {
		return nil, fmt.Errorf("calculate strokes: %w", err)
	}
	facets, err := e.api.Body().CalculateFacets(wire.CalculateFacetsArgs{BodyIndex: bodyIndex, Tolerance: meshTolerance})
	if err != nil {
		return nil, fmt.Errorf("calculate facets: %w", err)
	}
	return &section{
		bodyIndex: bodyIndex,
		loops:     loopsFromStrokes(strokes),
		regions:   facets.IndexCountPerFace,
	}, nil
}

// loopsFromStrokes unflattens StrokeSetResult (flat XY[Z] coords + per-polyline
// lengths) into closed 2D loops, dropping Z (the section is planar).
func loopsFromStrokes(s wire.StrokeSetResult) [][]point2 {
	loops := make([][]point2, 0, s.PolylineCount)
	at := 0
	for _, n := range s.PolylineLengths {
		loop := make([]point2, 0, n)
		for i := 0; i < n; i++ {
			base := (at + i) * 3
			loop = append(loop, point2{X: s.VertexCoordinates[base], Y: s.VertexCoordinates[base+1]})
		}
		loops = append(loops, loop)
		at += n
	}
	return loops
}

// pointCount totals the section's boundary vertices, for the run summary.
func (s *section) pointCount() int {
	n := 0
	for _, loop := range s.loops {
		n += len(loop)
	}
	return n
}
