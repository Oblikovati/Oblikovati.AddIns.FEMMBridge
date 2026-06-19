// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// minMeshAngle is triangle's quality bound (min angle, degrees). 30 is FEMM's
// default and keeps elements well-conditioned for the solver.
const minMeshAngle = 30.0

// solverBinaries locates the vendored FEMM toolchain executables. Both are AFPL
// third-party binaries the bridge runs at arm's length (vendor-src/femm/NOTICE.md);
// it never links them. Resolve from OBK_FEMM_BIN, else the in-repo CMake build dir.
type solverBinaries struct {
	triangle string
	fkern    string
}

// findSolverBinaries returns the toolchain paths, erroring if either is missing so
// the failure names what to build rather than surfacing a cryptic exec error.
func findSolverBinaries() (solverBinaries, error) {
	dir := os.Getenv("OBK_FEMM_BIN")
	if dir == "" {
		dir = "vendor-src/femm/build"
	}
	b := solverBinaries{
		triangle: filepath.Join(dir, "triangle"),
		fkern:    filepath.Join(dir, "fkern"),
	}
	for _, p := range []string{b.triangle, b.fkern} {
		if _, err := os.Stat(p); err != nil {
			return b, fmt.Errorf("FEMM solver binary missing: %s (build vendor-src/femm or set OBK_FEMM_BIN): %w", p, err)
		}
	}
	return b, nil
}

// runSolve executes the full magnetostatics pipeline in dir for problem base: write
// <base>.fem + <base>.poly + <base>.pbc, mesh with triangle, solve with fkern, and
// return the path to the <base>.ans solution. dir holds all intermediate files.
func runSolve(dir, base string, p *femProblem, m *polyMesh, bins solverBinaries) (string, error) {
	stem := filepath.Join(dir, base)
	if err := writeFile(stem+".fem", func(f *os.File) error { return writeFEM(f, p) }); err != nil {
		return "", err
	}
	if err := writeFile(stem+".poly", func(f *os.File) error { return writePoly(f, m) }); err != nil {
		return "", err
	}
	if err := writeFile(stem+".pbc", writePBC); err != nil {
		return "", err
	}
	if err := mesh(stem, bins.triangle); err != nil {
		return "", err
	}
	if err := solve(stem, bins.fkern); err != nil {
		return "", err
	}
	return stem + ".ans", nil
}

// writePBC writes the periodic-boundary / air-gap file. The bridge generates no
// periodic BCs or air-gap elements, so both counts are zero.
func writePBC(f *os.File) error {
	_, err := fmt.Fprint(f, "0\n0\n")
	return err
}

// mesh runs triangle on <stem>.poly with FEMM's flags, producing the zero-indexed
// <stem>.node/.ele/.edge mesh fkern reads.
func mesh(stem, triangle string) error {
	args := []string{"-p", "-P", fmt.Sprintf("-q%g", minMeshAngle), "-e", "-A", "-a", "-z", "-Q", "-I", stem}
	if out, err := exec.Command(triangle, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("triangle mesh %s: %w: %s", stem, err, out)
	}
	return nil
}

// solve runs fkern on the problem stem, writing <stem>.ans.
func solve(stem, fkern string) error {
	if out, err := exec.Command(fkern, stem).CombinedOutput(); err != nil {
		return fmt.Errorf("fkern solve %s: %w: %s", stem, err, out)
	}
	return nil
}

// writeFile creates path and hands the open file to write, ensuring it is closed.
func writeFile(path string, write func(*os.File) error) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	if err := write(f); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
