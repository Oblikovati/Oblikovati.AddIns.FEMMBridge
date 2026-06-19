// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"fmt"
	"os"
	"path/filepath"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// motorDescriptorHandoffPath is the cross-add-in hand-off: MotorDesigner writes the FEMM motor
// descriptor here on Generate, and the FEMM add-in reads it here on RunStudy. Both add-ins run
// inside the host process, so a well-known file is the simplest contract until a
// document-attribute channel (AttributeSets) lands.
func motorDescriptorHandoffPath() string {
	return filepath.Join(os.TempDir(), "oblikovati-femm-motor-descriptor.json")
}

// fieldScaleMax is the |B| color-scale ceiling (tesla): the iron runs ~1–2 T while tooth tips
// and re-entrant corners spike higher, so the scale is clamped here (matching bFieldMapper) to
// keep the yoke/tooth contrast instead of washing out to red.
const fieldScaleMax = 2.0

// tintClientID is the single client-graphics group holding the tinted surfaces + legend.
const tintClientID = "femm.bfield"

// RunMotorStudyOnHost is the whole add-in study: solve the |B| field for the motor descriptor
// MotorDesigner handed off, export each host part's surface mesh, tint every vertex by the
// field, and push the tinted surfaces plus a |B| legend back into the viewport — all through
// the api/client, so the host is reached only as its public surface allows.
func (e *Engine) RunMotorStudyOnHost() (*StudyResult, error) {
	field, err := e.solveMotorField(motorDescriptorHandoffPath())
	if err != nil {
		return nil, err
	}
	sampler, err := newFieldSampler(field)
	if err != nil {
		return nil, err
	}
	nodes, err := e.tintHostParts(sampler)
	if err != nil {
		return nil, err
	}
	scaleMax := e.scaleMax
	if scaleMax <= 0 {
		scaleMax = fieldScaleMax
	}
	nodes = append(nodes, legendNodes(bFieldMapper(), field, 0, scaleMax)...)
	if err := e.pushTint(nodes); err != nil {
		return nil, err
	}
	return &StudyResult{FieldVertices: field.vertexCount(), GraphicsClientID: tintClientID}, nil
}

// tintHostParts exports every part document's surface mesh and returns one tinted graphics node
// per part.
func (e *Engine) tintHostParts(sampler *fieldSampler) ([]wire.GraphicsNode, error) {
	docs, err := e.api.Documents().List()
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	var nodes []wire.GraphicsNode
	for _, d := range docs.Documents {
		if d.Type != "part" {
			continue
		}
		mesh, err := e.exportPartMesh(d.ID)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, tintedNode(d.Name, mesh, sampler, bFieldMapper()))
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no part documents to tint")
	}
	return nodes, nil
}

// exportPartMesh activates a part and writes its surface mesh to a temp OBJ the add-in reads
// back — exercising the public mesh-export surface FEA add-ins rely on.
func (e *Engine) exportPartMesh(id uint64) (*objMesh, error) {
	if _, err := e.api.Documents().Activate(id); err != nil {
		return nil, fmt.Errorf("activate doc %d: %w", id, err)
	}
	path := filepath.Join(os.TempDir(), fmt.Sprintf("femm-tint-%d.obj", id))
	if _, err := e.api.Documents().Export(wire.ExportRequest{
		Path: path, Format: "obj", Resolution: string(types.ResolutionHigh),
	}); err != nil {
		return nil, fmt.Errorf("export doc %d: %w", id, err)
	}
	return parseOBJ(path)
}

// pushTint activates the assembly (so the overlay is owned by the assembly document, per the
// doc-scoped client-graphics contract) and replaces the FEMM group with the tinted surfaces +
// legend.
func (e *Engine) pushTint(nodes []wire.GraphicsNode) error {
	if id, ok := e.assemblyDocID(); ok {
		if _, err := e.api.Documents().Activate(id); err != nil {
			return fmt.Errorf("activate assembly: %w", err)
		}
	}
	_, err := e.api.Graphics().Set(wire.SetClientGraphicsArgs{
		ClientId: tintClientID, Lane: string(types.GraphicsLanePersistent), Nodes: nodes,
	})
	return err
}

// assemblyDocID returns the open assembly document's id, if any.
func (e *Engine) assemblyDocID() (uint64, bool) {
	docs, err := e.api.Documents().List()
	if err != nil {
		return 0, false
	}
	for _, d := range docs.Documents {
		if d.Type == "assembly" {
			return d.ID, true
		}
	}
	return 0, false
}
