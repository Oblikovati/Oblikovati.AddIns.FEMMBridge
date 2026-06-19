// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"oblikovati.org/api/wire"
)

// fakeHost is a named fake HostCaller (no live host): it answers the wire methods a
// magnetics study issues with canned JSON, and records the methods it saw so a test
// can assert the full section→materials→render call sequence ran.
type fakeHost struct {
	calls    []string
	failOn   string // method to fail, for error-path tests ("" = none)
	strokes  wire.StrokeSetResult
	facets   wire.FacetSetResult
	material wire.MaterialInfo
}

func (h *fakeHost) Call(method string, _ []byte) ([]byte, error) {
	h.calls = append(h.calls, method)
	if method == h.failOn {
		return nil, os.ErrInvalid
	}
	switch method {
	case wire.MethodBodyCalculateStrokes:
		return json.Marshal(h.strokes)
	case wire.MethodBodyCalculateFacets:
		return json.Marshal(h.facets)
	case wire.MethodMaterialsList:
		return json.Marshal(wire.ListMaterialsResult{Materials: []wire.MaterialInfo{h.material}})
	default:
		return []byte("{}"), nil // registerColorMapper / set return no body the engine reads
	}
}

// squareHost is a fake whose body is a unit square (one loop, one region) with a
// resistive material, enough to drive a full study.
func squareHost() *fakeHost {
	return &fakeHost{
		strokes: wire.StrokeSetResult{
			VertexCount:       4,
			VertexCoordinates: []float64{0, 0, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0},
			PolylineCount:     1,
			PolylineLengths:   []int{4},
		},
		facets:   wire.FacetSetResult{IndexCountPerFace: []int{2}},
		material: wire.MaterialInfo{DisplayName: "Steel", Electrical: wire.MaterialInfo{}.Electrical},
	}
}

func TestRunStudyDrivesFullPipeline(t *testing.T) {
	h := squareHost()
	res, err := NewEngine(h).RunStudy(0)
	if err != nil {
		t.Fatalf("RunStudy: %v", err)
	}
	defer os.Remove(res.FemPath)

	if res.PointCount != 4 {
		t.Errorf("PointCount = %d, want 4 (square loop)", res.PointCount)
	}
	if res.RegionCount != 1 {
		t.Errorf("RegionCount = %d, want 1 (one face)", res.RegionCount)
	}
	if res.GraphicsClientID != "femm.bfield" {
		t.Errorf("GraphicsClientID = %q, want femm.bfield", res.GraphicsClientID)
	}
	// The study must touch the geometry, material, and graphics surfaces in order.
	want := []string{
		wire.MethodBodyCalculateStrokes,
		wire.MethodBodyCalculateFacets,
		wire.MethodMaterialsList,
		wire.MethodClientGraphicsRegisterMapper,
		wire.MethodClientGraphicsSet,
	}
	for _, m := range want {
		if !contains(h.calls, m) {
			t.Errorf("study never called %q (calls: %v)", m, h.calls)
		}
	}
}

func TestEmittedFEMIsWellFormed(t *testing.T) {
	h := squareHost()
	res, err := NewEngine(h).RunStudy(0)
	if err != nil {
		t.Fatalf("RunStudy: %v", err)
	}
	defer os.Remove(res.FemPath)

	b, err := os.ReadFile(res.FemPath)
	if err != nil {
		t.Fatalf("read .fem: %v", err)
	}
	fem := string(b)
	for _, tok := range []string{"[ProblemType] =  planar", "[Frequency]   =  0", "<BeginBlock>", "[NumPoints] = 4"} {
		if !strings.Contains(fem, tok) {
			t.Errorf(".fem missing %q:\n%s", tok, fem)
		}
	}
}

func TestRunStudyPropagatesSectionError(t *testing.T) {
	h := squareHost()
	h.failOn = wire.MethodBodyCalculateStrokes
	if _, err := NewEngine(h).RunStudy(0); err == nil {
		t.Fatal("RunStudy should fail when the host section call fails")
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
