// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// Tinting the analyzed parts (rather than floating a flood-plot disk) means coloring each
// body's real surface by |B|. The 2D magnetostatic field is invariant along the motor axis, so
// a surface vertex at (x,y,z) takes |B|(x,y): we export each part's mesh, sample the solved
// field at every vertex, and push it back as a per-vertex-colored surface that sits on the
// geometry. This keeps the whole pipeline inside the add-in — the host is reached only through
// the api/client (mesh export + client graphics), never bespoke driver code.

// fieldSampler answers |B|(x,y) (tesla) over the solved cross-section by bucketing the field
// mesh vertices into a regular grid — a nearest-value lookup dense enough for the field's
// sub-millimetre node spacing.
type fieldSampler struct {
	grid             []float64 // |B| per cell, row-major; NaN = empty
	nx, ny           int
	minX, minY, cell float64
}

const samplerGridCells = 220 // ~0.5 mm cells over a ~10 cm field disk

// newFieldSampler builds the lookup grid from a solved field (vertices in cm, scalars = |B|).
// Each field vertex is splatted into its cell and the 8 neighbors so a part-surface vertex that
// falls between field nodes still resolves.
func newFieldSampler(f *field) (*fieldSampler, error) {
	if len(f.verts) == 0 || len(f.scalars) != len(f.verts) {
		return nil, fmt.Errorf("field unusable: %d verts, %d scalars", len(f.verts), len(f.scalars))
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range f.verts {
		minX, maxX = math.Min(minX, p.X), math.Max(maxX, p.X)
		minY, maxY = math.Min(minY, p.Y), math.Max(maxY, p.Y)
	}
	cell := math.Max(maxX-minX, maxY-minY) / samplerGridCells
	s := &fieldSampler{
		nx: samplerGridCells + 1, ny: samplerGridCells + 1,
		minX: minX, minY: minY, cell: cell,
	}
	s.grid = make([]float64, s.nx*s.ny)
	for i := range s.grid {
		s.grid[i] = math.NaN()
	}
	for i, p := range f.verts {
		s.splat(p.X, p.Y, f.scalars[i])
	}
	return s, nil
}

// splat writes value into the (x,y) cell and its 8 neighbors, keeping the max so a high-field
// node is not overwritten by an adjacent low one.
func (s *fieldSampler) splat(x, y, value float64) {
	cx := int((x - s.minX) / s.cell)
	cy := int((y - s.minY) / s.cell)
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			ix, iy := cx+dx, cy+dy
			if ix < 0 || iy < 0 || ix >= s.nx || iy >= s.ny {
				continue
			}
			at := &s.grid[iy*s.nx+ix]
			if math.IsNaN(*at) || value > *at {
				*at = value
			}
		}
	}
}

// at returns |B| at (x,y) cm; cells outside the solved domain read 0.
func (s *fieldSampler) at(x, y float64) float64 {
	cx := int((x - s.minX) / s.cell)
	cy := int((y - s.minY) / s.cell)
	if cx < 0 || cy < 0 || cx >= s.nx || cy >= s.ny {
		return 0
	}
	if v := s.grid[cy*s.nx+cx]; !math.IsNaN(v) {
		return v
	}
	return 0
}

// objMesh is a flattened triangle mesh: one (position, normal) per triangle corner, so it maps
// 1:1 to a per-vertex client-graphics primitive with sequential indices.
type objMesh struct {
	coords  []float64 // xyz triples (model units, unscaled)
	normals []float64 // xyz triples
}

// parseOBJ reads a Wavefront OBJ (the host's mesh export) into a flattened corner mesh,
// expanding polygon faces into a triangle fan and resolving each corner's "v/vt/vn" reference.
func parseOBJ(path string) (*objMesh, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m := &objMesh{}
	if err := scanOBJ(f, m); err != nil {
		return nil, err
	}
	if len(m.coords) == 0 {
		return nil, fmt.Errorf("OBJ %s has no triangles", path)
	}
	return m, nil
}

// scanOBJ reads vertex/normal/face lines from r into m, flattening faces as it goes.
func scanOBJ(r io.Reader, m *objMesh) error {
	var verts, norms [][3]float64
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "v":
			verts = append(verts, parse3(fields[1:]))
		case "vn":
			norms = append(norms, parse3(fields[1:]))
		case "f":
			appendFace(m, fields[1:], verts, norms)
		}
	}
	return sc.Err()
}

// appendFace fans an OBJ face (3+ corners) into triangles, emitting flattened corners.
func appendFace(m *objMesh, corners []string, verts, norms [][3]float64) {
	for i := 1; i+1 < len(corners); i++ {
		for _, c := range []string{corners[0], corners[i], corners[i+1]} {
			vi, ni := faceRef(c)
			p := vertAt(verts, vi)
			m.coords = append(m.coords, p[0], p[1], p[2])
			n := vertAt(norms, ni)
			m.normals = append(m.normals, n[0], n[1], n[2])
		}
	}
}

// faceRef parses a "v", "v/vt", "v//vn", or "v/vt/vn" corner into 1-based vertex + normal
// indices (0 when absent).
func faceRef(c string) (vi, ni int) {
	parts := strings.Split(c, "/")
	vi, _ = strconv.Atoi(parts[0])
	if len(parts) == 3 {
		ni, _ = strconv.Atoi(parts[2])
	}
	return vi, ni
}

// vertAt resolves a 1-based (OBJ) index into a slice, returning the zero vector when out of range.
func vertAt(s [][3]float64, idx int) [3]float64 {
	if idx >= 1 && idx <= len(s) {
		return s[idx-1]
	}
	return [3]float64{}
}

func parse3(f []string) [3]float64 {
	var v [3]float64
	for i := 0; i < 3 && i < len(f); i++ {
		v[i], _ = strconv.ParseFloat(f[i], 64)
	}
	return v
}

// detectScaleToCm infers the OBJ→cm factor from the mesh radius: a motor exported in millimetres
// reads tens of units, in centimetres a few — the host exports in the document's display unit.
func detectScaleToCm(m *objMesh) float64 {
	var maxR float64
	for i := 0; i+1 < len(m.coords); i += 3 {
		if r := math.Hypot(m.coords[i], m.coords[i+1]); r > maxR {
			maxR = r
		}
	}
	if maxR > 15 {
		return 0.1 // millimetres → centimetres
	}
	return 1.0
}

// tintedNode builds one per-vertex |B|-colored client-graphics node for a part's exported
// surface mesh: every corner keeps its real normal (so the surface shades as geometry, not a
// flat overlay) and samples |B| at its (x,y). scaleToCm maps the OBJ's units to the host cm world.
func tintedNode(id string, m *objMesh, s *fieldSampler, mapper wire.GraphicsColorMapper) wire.GraphicsNode {
	scaleToCm := detectScaleToCm(m)
	n := len(m.coords) / 3
	coords := make([]float64, len(m.coords))
	scalars := make([]float64, n)
	indices := make([]int, n)
	for i := 0; i < n; i++ {
		x, y := m.coords[i*3]*scaleToCm, m.coords[i*3+1]*scaleToCm
		coords[i*3], coords[i*3+1], coords[i*3+2] = x, y, m.coords[i*3+2]*scaleToCm
		scalars[i] = s.at(x, y)
		indices[i] = i
	}
	return wire.GraphicsNode{
		Id: id,
		Primitives: []wire.GraphicsPrimitive{{
			Kind: string(types.GraphicsTriangles), Coordinates: coords, Indices: indices,
			Normals: m.normals, Scalars: scalars, ColorMapper: &mapper,
			ColorBinding: string(types.GraphicsColorPerVertex),
		}},
	}
}
