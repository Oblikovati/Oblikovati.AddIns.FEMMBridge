// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"io"
)

// femMaterial is one [BlockProps] record (a magnetics material). Jr is applied
// current density in MA/m^2; Mu* are relative permeabilities; Hc is coercivity
// (A/m) for permanent magnets; Sigma is conductivity (MS/m).
type femMaterial struct {
	MuX, MuY float64
	Hc       float64
	Jr       float64
	Sigma    float64
}

// femBoundary is one [BdryProps] record. BdryType 0 = prescribed vector potential
// A = A0 + A1*x + A2*y (Dirichlet; A0=A1=A2=0 is the common A=0 far boundary).
type femBoundary struct {
	BdryType   int
	A0, A1, A2 float64
}

// femLabel is one [NumBlockLabels] record: a seed point whose 1-based BlockType
// selects the material; the bridge's .poly gives triangle the matching region
// attribute so every element in the region resolves to this material.
type femLabel struct {
	X, Y      float64
	BlockType int
	MaxArea   float64
	InGroup   int
}

// femProblem is the complete .fem problem definition fkern's OnOpenDocument reads.
// Geometry (points/segments) is NOT here — fkern takes it from the triangle mesh;
// the .fem carries only the physics (materials, boundary conditions, block labels).
type femProblem struct {
	Frequency  float64 // 0 = magnetostatic
	Precision  float64
	Planar     bool // false = axisymmetric
	Materials  []femMaterial
	Boundaries []femBoundary
	Labels     []femLabel
}

// writeFEM emits the .fem in fkern's exact key/value + tagged-record format
// (decoded in bridge/PIPELINE.md). LengthUnits is centimeters — the host DB unit —
// so no coordinate scaling is needed anywhere in the pipeline.
func writeFEM(w io.Writer, p *femProblem) error {
	probType := "axisymmetric"
	if p.Planar {
		probType = "planar"
	}
	fmt.Fprintf(w, "[Format]      = 4.0\n")
	fmt.Fprintf(w, "[Frequency]   = %g\n", p.Frequency)
	fmt.Fprintf(w, "[Precision]   = %g\n", p.Precision)
	fmt.Fprintf(w, "[LengthUnits] = centimeters\n")
	fmt.Fprintf(w, "[ProblemType] = %s\n", probType)
	fmt.Fprintf(w, "[Coordinates] = cartesian\n")
	fmt.Fprintf(w, "[PointProps]  = 0\n")
	writeFEMBoundaries(w, p.Boundaries)
	writeFEMMaterials(w, p.Materials)
	fmt.Fprintf(w, "[CircuitProps] = 0\n")
	writeFEMLabels(w, p.Labels)
	return nil
}

// writeFEMBoundaries emits the [BdryProps] tagged records.
func writeFEMBoundaries(w io.Writer, bs []femBoundary) {
	fmt.Fprintf(w, "[BdryProps]   = %d\n", len(bs))
	for _, b := range bs {
		fmt.Fprintf(w, "  <BeginBdry>\n")
		fmt.Fprintf(w, "    <BdryType> = %d\n", b.BdryType)
		fmt.Fprintf(w, "    <A_0> = %g\n", b.A0)
		fmt.Fprintf(w, "    <A_1> = %g\n", b.A1)
		fmt.Fprintf(w, "    <A_2> = %g\n", b.A2)
		fmt.Fprintf(w, "    <phi> = 0\n")
		fmt.Fprintf(w, "    <mu_ssd> = 0\n")
		fmt.Fprintf(w, "    <sigma_ssd> = 0\n")
		fmt.Fprintf(w, "  <EndBdry>\n")
	}
}

// writeFEMMaterials emits the [BlockProps] tagged records (one per material).
func writeFEMMaterials(w io.Writer, ms []femMaterial) {
	fmt.Fprintf(w, "[BlockProps]  = %d\n", len(ms))
	for _, m := range ms {
		fmt.Fprintf(w, "  <BeginBlock>\n")
		fmt.Fprintf(w, "    <Mu_x> = %g\n", m.MuX)
		fmt.Fprintf(w, "    <Mu_y> = %g\n", m.MuY)
		fmt.Fprintf(w, "    <H_c> = %g\n", m.Hc)
		fmt.Fprintf(w, "    <H_cAngle> = 0\n")
		fmt.Fprintf(w, "    <J_re> = %g\n", m.Jr)
		fmt.Fprintf(w, "    <J_im> = 0\n")
		fmt.Fprintf(w, "    <sigma> = %g\n", m.Sigma)
		fmt.Fprintf(w, "    <d_lam> = 0\n")
		fmt.Fprintf(w, "    <LamFill> = 1\n")
		fmt.Fprintf(w, "    <LamType> = 0\n")
		fmt.Fprintf(w, "    <NStrands> = 0\n")
		fmt.Fprintf(w, "    <WireD> = 0\n")
		fmt.Fprintf(w, "    <BHPoints> = 0\n")
		fmt.Fprintf(w, "  <EndBlock>\n")
	}
}

// writeFEMLabels emits [NumBlockLabels] and one record per label. Field order is
// x y BlockType MaxArea InCircuit MagDir InGroup Turns IsExternal (fkern parses it
// positionally).
func writeFEMLabels(w io.Writer, ls []femLabel) {
	fmt.Fprintf(w, "[NumBlockLabels] = %d\n", len(ls))
	for _, l := range ls {
		fmt.Fprintf(w, "%.17g %.17g %d %.17g 0 0 %d 1 0\n",
			l.X, l.Y, l.BlockType, l.MaxArea, l.InGroup)
	}
}
