// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

// sampleAns is a minimal .ans: an echoed-.fem preamble (which must be skipped),
// then a [Solution] section with 3 nodes and 1 element.
const sampleAns = `[Format] = 4.0
[Frequency] = 0
[NumBlockLabels] = 1
0.5 0.5 1 0.25 0 0 0 1 0
[Solution]
3
0	0	0	-2
1	0	1.5	-2
0	1	2.5	-2
1
0	1	2	0	-2	-2	-2	0
`

func TestParseAns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.ans")
	if err := os.WriteFile(path, []byte(sampleAns), 0o600); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	sol, err := parseAns(path)
	if err != nil {
		t.Fatalf("parseAns: %v", err)
	}
	if len(sol.Nodes) != 3 {
		t.Fatalf("nodes = %d, want 3", len(sol.Nodes))
	}
	if sol.Nodes[2].X != 0 || sol.Nodes[2].Y != 1 || sol.Nodes[2].A != 2.5 {
		t.Errorf("node[2] = %+v, want {0 1 2.5}", sol.Nodes[2])
	}
	if len(sol.Elements) != 1 || sol.Elements[0].P != [3]int{0, 1, 2} {
		t.Errorf("elements = %+v, want one {0 1 2}", sol.Elements)
	}
}

func TestParseAnsMissingSolution(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.ans")
	if err := os.WriteFile(path, []byte("[Format] = 4.0\nno solution here\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := parseAns(path); err == nil {
		t.Fatal("parseAns should fail when there is no [Solution] section")
	}
}
