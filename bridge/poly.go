// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"io"
)

// polyNode is a PSLG vertex. Marker encodes a point boundary condition as
// pointPropIndex+2 (0 = none), matching FEMM's bd_writepoly + fkern LoadMesh.
type polyNode struct {
	X, Y   float64
	Marker int
}

// polySegment is a boundary edge between two node indices. Marker encodes a line
// boundary condition as -(linePropIndex+2) (0 = none) — negative by FEMM convention.
type polySegment struct {
	N0, N1 int
	Marker int
}

// polyRegion seeds a meshed area: a point strictly inside it, the 1-based block-label
// attribute triangle propagates to every element there (-A), and a local max area.
type polyRegion struct {
	X, Y    float64
	Label   int
	MaxArea float64
}

// polyMesh is the complete PSLG the bridge hands to triangle: the FEMM-marked
// geometry for one magnetostatics problem. Holes are interior points of voids.
type polyMesh struct {
	Nodes    []polyNode
	Segments []polySegment
	Holes    [][2]float64
	Regions  []polyRegion
}

// writePoly emits the .poly in Triangle's PSLG format with FEMM's marker conventions
// (see bridge/PIPELINE.md). Triangle is then run as:
//
//	triangle -p -P -q<angle> -e -A -a -z -Q -I <name>
//
// producing zero-indexed .node/.ele/.edge that fkern's LoadMesh reads directly.
func writePoly(w io.Writer, m *polyMesh) error {
	if err := writePolyNodes(w, m.Nodes); err != nil {
		return err
	}
	if err := writePolySegments(w, m.Segments); err != nil {
		return err
	}
	if err := writePolyHoles(w, m.Holes); err != nil {
		return err
	}
	return writePolyRegions(w, m.Regions)
}

// writePolyNodes writes the node section: "N 2 0 1" then "i x y marker".
func writePolyNodes(w io.Writer, nodes []polyNode) error {
	if _, err := fmt.Fprintf(w, "%d 2 0 1\n", len(nodes)); err != nil {
		return err
	}
	for i, n := range nodes {
		if _, err := fmt.Fprintf(w, "%d %.17g %.17g %d\n", i, n.X, n.Y, n.Marker); err != nil {
			return err
		}
	}
	return nil
}

// writePolySegments writes the segment section: "M 1" then "i n0 n1 marker".
func writePolySegments(w io.Writer, segs []polySegment) error {
	if _, err := fmt.Fprintf(w, "%d 1\n", len(segs)); err != nil {
		return err
	}
	for i, s := range segs {
		if _, err := fmt.Fprintf(w, "%d %d %d %d\n", i, s.N0, s.N1, s.Marker); err != nil {
			return err
		}
	}
	return nil
}

// writePolyHoles writes the hole section: "H" then "k hx hy".
func writePolyHoles(w io.Writer, holes [][2]float64) error {
	if _, err := fmt.Fprintf(w, "%d\n", len(holes)); err != nil {
		return err
	}
	for i, h := range holes {
		if _, err := fmt.Fprintf(w, "%d %.17g %.17g\n", i, h[0], h[1]); err != nil {
			return err
		}
	}
	return nil
}

// writePolyRegions writes the region section: "R" then "k rx ry attr maxArea",
// where attr (= Label) is the 1-based block-label index triangle assigns via -A.
func writePolyRegions(w io.Writer, regions []polyRegion) error {
	if _, err := fmt.Fprintf(w, "%d\n", len(regions)); err != nil {
		return err
	}
	for i, r := range regions {
		if _, err := fmt.Fprintf(w, "%d %.17g %.17g %d %.17g\n", i, r.X, r.Y, r.Label, r.MaxArea); err != nil {
			return err
		}
	}
	return nil
}
