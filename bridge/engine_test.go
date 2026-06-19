// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"oblikovati.org/api/wire"
)

// requireSolver points the engine at the in-repo CMake build output and skips the
// test when the fkern/triangle binaries have not been built (e.g. a Go-only CI job).
// Tests that actually run the solver call this first.
func requireSolver(t *testing.T) {
	t.Helper()
	dir, err := filepath.Abs("../vendor-src/femm/build")
	if err != nil {
		t.Fatalf("resolve solver dir: %v", err)
	}
	for _, b := range []string{"triangle", "fkern"} {
		if _, err := os.Stat(filepath.Join(dir, b)); err != nil {
			t.Skipf("solver not built (%s): run `cmake --build vendor-src/femm/build`", b)
		}
	}
	t.Setenv("OBK_FEMM_BIN", dir)
}

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
	requireSolver(t)
	h := squareHost()
	res, err := NewEngine(h).RunStudy(0)
	if err != nil {
		t.Fatalf("RunStudy: %v", err)
	}
	if fi, err := os.Stat(res.AnsPath); err != nil || fi.Size() == 0 {
		t.Errorf("expected a non-empty .ans at %s (err=%v)", res.AnsPath, err)
	}
	if res.RegionCount != 1 {
		t.Errorf("RegionCount = %d, want 1 (one face)", res.RegionCount)
	}
	if res.GraphicsClientID != "femm.bfield" {
		t.Errorf("GraphicsClientID = %q, want femm.bfield", res.GraphicsClientID)
	}
	// The study must touch the geometry, material, and graphics surfaces.
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
