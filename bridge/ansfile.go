// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ansNode is a solved mesh node: position (problem length units = cm) and the
// magnetic vector potential A (Wb/m) fkern solved for there.
type ansNode struct {
	X, Y float64
	A    float64
}

// ansElement is a linear-triangle element by its three node indices.
type ansElement struct {
	P [3]int
}

// solution is the parsed [Solution] section of a .ans: the nodal vector potential
// field on the triangle mesh. |B| is derived from it (see bField).
type solution struct {
	Nodes    []ansNode
	Elements []ansElement
}

// parseAns reads fkern's .ans file. The file echoes the input .fem, then a
// [Solution] section: NumNodes lines "x y A bc", then NumElements lines
// "p0 p1 p2 lbl e0 e1 e2 Jprev" (see prob1big.cpp WriteStatic2D / PIPELINE.md).
func parseAns(path string) (*solution, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open .ans %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	if err := scanToSolution(sc); err != nil {
		return nil, err
	}
	nodes, err := parseAnsNodes(sc)
	if err != nil {
		return nil, err
	}
	elems, err := parseAnsElements(sc)
	if err != nil {
		return nil, err
	}
	return &solution{Nodes: nodes, Elements: elems}, nil
}

// scanToSolution advances the scanner past the echoed .fem to the [Solution] line.
func scanToSolution(sc *bufio.Scanner) error {
	for sc.Scan() {
		if strings.EqualFold(strings.TrimSpace(sc.Text()), "[Solution]") {
			return nil
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("scan .ans: %w", err)
	}
	return fmt.Errorf("parse .ans: no [Solution] section found")
}

// parseAnsNodes reads the node count then that many "x y A bc" rows.
func parseAnsNodes(sc *bufio.Scanner) ([]ansNode, error) {
	n, err := scanCount(sc, "node count")
	if err != nil {
		return nil, err
	}
	nodes := make([]ansNode, n)
	for i := 0; i < n; i++ {
		f, err := scanFields(sc, 3, "node", i)
		if err != nil {
			return nil, err
		}
		nodes[i] = ansNode{X: f[0], Y: f[1], A: f[2]}
	}
	return nodes, nil
}

// parseAnsElements reads the element count then that many rows; only the first
// three fields (the triangle node indices) are needed to compute B.
func parseAnsElements(sc *bufio.Scanner) ([]ansElement, error) {
	n, err := scanCount(sc, "element count")
	if err != nil {
		return nil, err
	}
	elems := make([]ansElement, n)
	for i := 0; i < n; i++ {
		f, err := scanFields(sc, 3, "element", i)
		if err != nil {
			return nil, err
		}
		elems[i] = ansElement{P: [3]int{int(f[0]), int(f[1]), int(f[2])}}
	}
	return elems, nil
}

// scanCount reads the next non-empty line as an integer count.
func scanCount(sc *bufio.Scanner, what string) (int, error) {
	for sc.Scan() {
		t := strings.TrimSpace(sc.Text())
		if t == "" {
			continue
		}
		n, err := strconv.Atoi(strings.Fields(t)[0])
		if err != nil {
			return 0, fmt.Errorf("parse .ans %s %q: %w", what, t, err)
		}
		return n, nil
	}
	return 0, fmt.Errorf("parse .ans: unexpected EOF reading %s", what)
}

// scanFields reads the next line and returns its first min whitespace-separated
// fields as floats, erroring with the offending row if it is short or malformed.
func scanFields(sc *bufio.Scanner, min int, what string, i int) ([]float64, error) {
	if !sc.Scan() {
		return nil, fmt.Errorf("parse .ans: unexpected EOF at %s %d", what, i)
	}
	fields := strings.Fields(sc.Text())
	if len(fields) < min {
		return nil, fmt.Errorf("parse .ans %s %d: have %d fields, need %d: %q", what, i, len(fields), min, sc.Text())
	}
	out := make([]float64, min)
	for k := 0; k < min; k++ {
		v, err := strconv.ParseFloat(fields[k], 64)
		if err != nil {
			return nil, fmt.Errorf("parse .ans %s %d field %d %q: %w", what, i, k, fields[k], err)
		}
		out[k] = v
	}
	return out, nil
}
