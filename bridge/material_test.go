// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// TestMagneticFromHostReadsPermanentMagnet pins the GAP #3 closure: a host material that
// carries a hard-magnetic group yields a FEMM block material with the magnet's recoil μr
// and coercivity (converted kA/m → A/m), not the air fallback.
func TestMagneticFromHostReadsPermanentMagnet(t *testing.T) {
	m := wire.MaterialInfo{
		DisplayName: "Magnet (NdFeB, N42 Sintered)",
		Magnetic: types.Magnetic{
			Class: types.HardMagnetic, RelativePermeability: 1.05,
			Remanence: 1.30, Coercivity: 915, // kA/m
		},
	}
	got := magneticFromHost(m)
	if got.muR != 1.05 {
		t.Errorf("muR = %v, want 1.05", got.muR)
	}
	if got.coercivity != 915000 {
		t.Errorf("coercivity = %v A/m, want 915000 (915 kA/m)", got.coercivity)
	}
	if got.name != m.DisplayName {
		t.Errorf("name = %q, want %q", got.name, m.DisplayName)
	}
}

// TestMagneticFromHostReadsSoftCore pins that a soft-magnetic lamination yields its high μr.
func TestMagneticFromHostReadsSoftCore(t *testing.T) {
	m := wire.MaterialInfo{
		DisplayName: "Electrical Steel (M270-35A, NO)",
		Magnetic:    types.Magnetic{Class: types.SoftMagnetic, RelativePermeability: 4000, SaturationFluxDensity: 1.8},
	}
	got := magneticFromHost(m)
	if got.muR != 4000 {
		t.Errorf("muR = %v, want 4000", got.muR)
	}
	if got.coercivity != 0 {
		t.Errorf("soft core coercivity = %v, want 0", got.coercivity)
	}
}

// TestMagneticFromHostNonMagneticIsAir pins the fallback: a material with no magnetic group
// (the zero value) stays air-like (μr = 1, no coercivity) so an unassigned region solves.
func TestMagneticFromHostNonMagneticIsAir(t *testing.T) {
	got := magneticFromHost(wire.MaterialInfo{DisplayName: "Aluminum 6061"})
	if got.muR != 1 {
		t.Errorf("non-magnetic muR = %v, want 1 (air)", got.muR)
	}
	if got.coercivity != 0 {
		t.Errorf("non-magnetic coercivity = %v, want 0", got.coercivity)
	}
}

// TestMagneticFromHostRecoversConductivity pins that σ is still read from resistivity.
func TestMagneticFromHostRecoversConductivity(t *testing.T) {
	m := wire.MaterialInfo{Electrical: types.Electrical{Resistivity: 1.4e-6}} // Ω·m (NdFeB)
	got := magneticFromHost(m)
	want := 1.0 / 1.4e-6 / 1e6 // MS/m
	if got.sigma != want {
		t.Errorf("sigma = %v MS/m, want %v", got.sigma, want)
	}
}
