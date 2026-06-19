// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"os"
	"strings"
)

// emitFEM writes a FEMM 4.2 magnetostatics .fem for the section and returns the path
// and the number of geometry points written. The token set matches what the ported
// fkern loader (fkn/femmedoccore.cpp) reads: a planar, length-in-millimetre
// magnetostatic problem with one block label per region. Phase 2a feeds this to the
// mesher + solver; for now it only needs to be well-formed.
func emitFEM(s *section, regions []regionMaterial) (string, int, error) {
	var b strings.Builder
	writeFEMHeader(&b)
	writeFEMMaterials(&b, regions)
	pts := writeFEMGeometry(&b, s)

	f, err := os.CreateTemp("", "femm-*.fem")
	if err != nil {
		return "", 0, fmt.Errorf("create .fem: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(b.String()); err != nil {
		return "", 0, fmt.Errorf("write .fem %s: %w", f.Name(), err)
	}
	return f.Name(), pts, nil
}

// writeFEMHeader emits the problem-definition block (magnetostatic, planar, mm).
func writeFEMHeader(b *strings.Builder) {
	fmt.Fprintln(b, "[Format]      =  4.0")
	fmt.Fprintln(b, "[Frequency]   =  0") // 0 ⇒ magnetostatic
	fmt.Fprintln(b, "[Precision]   =  1e-008")
	fmt.Fprintln(b, "[MinAngle]    =  30")
	fmt.Fprintln(b, "[ProblemType] =  planar")
	fmt.Fprintln(b, "[Coordinates] =  cartesian")
	fmt.Fprintln(b, "[LengthUnits] =  millimeters")
}

// writeFEMMaterials emits one block-property record per region material.
func writeFEMMaterials(b *strings.Builder, regions []regionMaterial) {
	fmt.Fprintf(b, "[BlockProps]  = %d\n", len(regions))
	for _, m := range regions {
		fmt.Fprintln(b, "  <BeginBlock>")
		fmt.Fprintf(b, "    <BlockName> = \"%s\"\n", m.name)
		fmt.Fprintf(b, "    <Mu_x> = %g\n", m.muR)
		fmt.Fprintf(b, "    <Mu_y> = %g\n", m.muR)
		fmt.Fprintf(b, "    <H_c> = %g\n", m.coercivity)
		fmt.Fprintf(b, "    <Sigma> = %g\n", m.sigma)
		fmt.Fprintln(b, "  <EndBlock>")
	}
}

// writeFEMGeometry emits the section points and returns the point count.
// Coordinates are converted cm (host) → mm (.fem).
func writeFEMGeometry(b *strings.Builder, s *section) int {
	pts := s.pointCount()
	fmt.Fprintf(b, "[NumPoints] = %d\n", pts)
	for _, loop := range s.loops {
		for _, p := range loop {
			fmt.Fprintf(b, "%g %g\n", p.X*10, p.Y*10)
		}
	}
	return pts
}
