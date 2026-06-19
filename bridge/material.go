// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "oblikovati.org/api/wire"

// regionMaterial is one FEMM block material: the magnetic properties a
// magnetostatics solve needs, resolved per section region.
type regionMaterial struct {
	name       string
	muR        float64 // relative permeability (linear region)
	coercivity float64 // H_c, A/m (permanent magnets)
	sigma      float64 // electrical conductivity, MS/m (eddy currents)
}

// air is the default fill — μ_r = 1, no source, no conductivity.
var air = regionMaterial{name: "Air", muR: 1}

// resolveRegionMaterials maps each section region to a FEMM block material.
//
// GAP #3 (now CLOSED, API 0.69.0): wire.MaterialInfo carries a types.Magnetic group —
// relative permeability, remanence Br and coercivity Hc — so magnetostatics reads its
// governing property straight off the host material. Electrical conductivity is still
// recovered from resistivity (σ = 1/ρ). A non-magnetic (or unset) host material falls
// back to air so an unassigned region still solves.
func (e *Engine) resolveRegionMaterials(s *section) ([]regionMaterial, error) {
	list, err := e.api.Materials().List()
	if err != nil {
		return nil, err
	}
	host := firstMaterial(list)
	mats := make([]regionMaterial, 0, len(s.regions))
	for range s.regions {
		mats = append(mats, magneticFromHost(host))
	}
	return mats, nil
}

// magneticFromHost reads a FEMM block material from a host material's Magnetic group
// (API 0.69.0): μ_r for both soft cores and the recoil line of a permanent magnet, plus
// H_c [A/m] for a magnet. σ comes from electrical resistivity. A non-magnetic material
// keeps the air default (μ_r = 1). Coercivity is converted kA/m → A/m for the solver.
func magneticFromHost(m wire.MaterialInfo) regionMaterial {
	r := air
	r.name = m.DisplayName
	if m.Electrical.Resistivity > 0 {
		r.sigma = 1.0 / m.Electrical.Resistivity / 1e6 // Ω·m → MS/m
	}
	if m.Magnetic.IsMagnetic() {
		r.muR = m.Magnetic.RelativePermeability
		r.coercivity = m.Magnetic.Coercivity * 1000 // kA/m → A/m
	}
	return r
}

// firstMaterial returns the document's first material, or an air placeholder.
func firstMaterial(list wire.ListMaterialsResult) wire.MaterialInfo {
	if len(list.Materials) == 0 {
		return wire.MaterialInfo{DisplayName: "Air"}
	}
	return list.Materials[0]
}
