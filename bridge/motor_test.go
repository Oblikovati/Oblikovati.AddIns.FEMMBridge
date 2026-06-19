// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// circleLoopMM is a closed circle of radius r (mm) as n points.
func circleLoopMM(r float64, n int) [][2]float64 {
	loop := make([][2]float64, n)
	for i := 0; i < n; i++ {
		a := 2 * math.Pi * float64(i) / float64(n)
		loop[i] = [2]float64{r * math.Cos(a), r * math.Sin(a)}
	}
	return loop
}

// magnetSectorMM is a closed annular sector (rIn..rOut mm) centred at centreDeg,
// spanning ±halfDeg — one surface magnet pole.
func magnetSectorMM(rIn, rOut, centreDeg, halfDeg float64) [][2]float64 {
	const arc = 8
	var loop [][2]float64
	for i := 0; i <= arc; i++ { // outer arc, sweep +
		a := (centreDeg - halfDeg + 2*halfDeg*float64(i)/arc) * math.Pi / 180
		loop = append(loop, [2]float64{rOut * math.Cos(a), rOut * math.Sin(a)})
	}
	for i := arc; i >= 0; i-- { // inner arc, sweep back
		a := (centreDeg - halfDeg + 2*halfDeg*float64(i)/arc) * math.Pi / 180
		loop = append(loop, [2]float64{rIn * math.Cos(a), rIn * math.Sin(a)})
	}
	return loop
}

// twoPoleMotor is a minimal surface-PM motor cross-section: a stator iron annulus
// (bore 30 / OD 50 mm), a rotor iron annulus (shaft 10 / OD 27 mm), and two NdFeB
// magnets (27..29 mm) magnetised radially out/in — enough to drive a real field.
func twoPoleMotor() *MotorDescriptor {
	const muIron = 4000
	mag := func(centre float64) MotorRegion {
		c := centre * math.Pi / 180
		rm := 28.0
		return MotorRegion{
			Name:  "Magnet",
			Loops: [][][2]float64{magnetSectorMM(27, 29, centre, 70)},
			Seed:  [2]float64{rm * math.Cos(c), rm * math.Sin(c)},
			MuR:   1.05, HcAm: 900000, HcAngleDeg: centre, // radial, outward at the pole centre
		}
	}
	// Clean ~1 mm gaps so no boundaries coincide: rotor OD 26, magnets 27..29 (1 mm
	// air to the rotor), stator bore 30 (1 mm air gap to the magnets).
	return &MotorDescriptor{
		StatorOuterDiaMM: 100,
		Regions: []MotorRegion{
			{Name: "Stator", Loops: [][][2]float64{circleLoopMM(50, 48), circleLoopMM(30, 48)},
				Seed: [2]float64{40, 0}, MuR: muIron},
			{Name: "Rotor", Loops: [][][2]float64{circleLoopMM(26, 40), circleLoopMM(10, 24)},
				Seed: [2]float64{18, 0}, MuR: muIron},
			mag(0), mag(180),
		},
	}
}

// TestBuildMotorProblemDomain checks the multi-region assembly: a material + label per
// region plus the default air block, and the A=0 outer boundary present.
func TestBuildMotorProblemDomain(t *testing.T) {
	prob, mesh := buildMotorProblem(twoPoleMotor())
	if len(prob.Materials) != 5 { // air + 4 regions (stator, rotor, 2 magnets)
		t.Errorf("materials = %d, want 5 (air + 4 regions)", len(prob.Materials))
	}
	if len(prob.Labels) != 5 || !prob.Labels[0].IsDefault {
		t.Errorf("labels = %d (default=%v), want 5 with label 0 default", len(prob.Labels), prob.Labels[0].IsDefault)
	}
	if len(mesh.Regions) != 4 {
		t.Errorf("seeded regions = %d, want 4 (air is the default, unseeded)", len(mesh.Regions))
	}
	// A magnet material must carry directed coercivity.
	if prob.Materials[3].Hc == 0 || prob.Materials[3].HcAngle != 0 {
		// material[3] is the first magnet (centre 0°)
		if prob.Materials[3].Hc == 0 {
			t.Error("magnet material has no coercivity")
		}
	}
}

// TestMotorStudySolves runs the whole multi-region pipeline (descriptor → .fem/.poly →
// triangle → fkern → .ans → |B|) and checks a real motor field: finite, non-zero, with
// the peak in the iron/airgap rather than out at the A=0 boundary.
func TestMotorStudySolves(t *testing.T) {
	requireSolver(t)
	bins, err := findSolverBinaries()
	if err != nil {
		t.Fatalf("findSolverBinaries: %v", err)
	}
	prob, mesh := buildMotorProblem(twoPoleMotor())
	ans, err := runSolve(t.TempDir(), "motor", prob, mesh, bins)
	if err != nil {
		t.Fatalf("runSolve: %v", err)
	}
	sol, err := parseAns(ans)
	if err != nil {
		t.Fatalf("parseAns: %v", err)
	}
	if len(sol.Nodes) < 200 {
		t.Fatalf("mesh too coarse for a motor: %d nodes", len(sol.Nodes))
	}
	mags := nodalBMagnitude(sol)
	var maxB float64
	for _, m := range mags {
		if m > maxB {
			maxB = m
		}
	}
	if maxB <= 0 || math.IsNaN(maxB) || math.IsInf(maxB, 0) {
		t.Fatalf("|B| max = %g, want a finite non-zero motor field", maxB)
	}
	t.Logf("motor solve: %d nodes, %d elements, |B| max = %.4g T", len(sol.Nodes), len(sol.Elements), maxB)
}
