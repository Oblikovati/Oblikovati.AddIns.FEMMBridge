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
// GAP #3 (the headline finding): wire.MaterialInfo carries Mechanical, Thermal and
// Electrical groups but NO Magnetic group — there is no relative permeability,
// coercivity, or B-H curve on a host material. So magnetostatics cannot read its
// governing property from the host; the add-in falls back to its own library until a
// types.Magnetic group exists. Electrical conductivity is recoverable (σ = 1/ρ) and
// IS read from the host.
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

// magneticFromHost shows the gap concretely: only σ (from resistivity) survives the
// trip from the host; μ_r and H_c have no source field on MaterialInfo and fall back
// to an air-like default until a types.Magnetic group exists.
func magneticFromHost(m wire.MaterialInfo) regionMaterial {
	r := air
	r.name = m.DisplayName
	if m.Electrical.Resistivity > 0 {
		r.sigma = 1.0 / m.Electrical.Resistivity / 1e6 // Ω·m → MS/m
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
