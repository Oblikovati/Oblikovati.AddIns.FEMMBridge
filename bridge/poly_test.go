// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"strings"
	"testing"
)

// TestWritePolyFormat checks the .poly text for a unit square with a Dirichlet
// boundary (segment marker -2) and one air region (block label 1), against the
// FEMM/Triangle conventions in PIPELINE.md.
func TestWritePolyFormat(t *testing.T) {
	m := &polyMesh{
		Nodes: []polyNode{
			{X: 0, Y: 0, Marker: 0}, {X: 1, Y: 0, Marker: 0},
			{X: 1, Y: 1, Marker: 0}, {X: 0, Y: 1, Marker: 0},
		},
		Segments: []polySegment{
			{N0: 0, N1: 1, Marker: -2}, {N0: 1, N1: 2, Marker: -2},
			{N0: 2, N1: 3, Marker: -2}, {N0: 3, N1: 0, Marker: -2},
		},
		Regions: []polyRegion{{X: 0.5, Y: 0.5, Label: 1, MaxArea: 0.01}},
	}
	var b strings.Builder
	if err := writePoly(&b, m); err != nil {
		t.Fatalf("writePoly: %v", err)
	}
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")

	if lines[0] != "4 2 0 1" {
		t.Errorf("node header = %q, want \"4 2 0 1\"", lines[0])
	}
	// Segment header follows 4 node lines (index 5).
	if lines[5] != "4 1" {
		t.Errorf("segment header = %q, want \"4 1\"", lines[5])
	}
	// A boundary segment must carry the negative line-BC marker.
	if !strings.HasSuffix(lines[6], " -2") {
		t.Errorf("first segment = %q, want trailing marker -2", lines[6])
	}
	// Holes header (0) then regions header (1) then the region line.
	if !strings.Contains(b.String(), "\n0\n1\n") {
		t.Errorf("expected 0 holes then 1 region header in:\n%s", b.String())
	}
	if !strings.Contains(b.String(), "0.5 0.5 1 0.01") {
		t.Errorf("region line missing label/area in:\n%s", b.String())
	}
}

// TestWritePolyPointMarker verifies a point boundary condition encodes as index+2.
func TestWritePolyPointMarker(t *testing.T) {
	m := &polyMesh{Nodes: []polyNode{{X: 0, Y: 0, Marker: 3}}} // pointProp index 1 -> 1+2
	var b strings.Builder
	if err := writePoly(&b, m); err != nil {
		t.Fatalf("writePoly: %v", err)
	}
	if !strings.Contains(b.String(), "0 0 0 3\n") {
		t.Errorf("point marker not encoded as 3 in:\n%s", b.String())
	}
}
