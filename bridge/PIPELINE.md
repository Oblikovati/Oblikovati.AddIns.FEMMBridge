# FEMM magnetostatics pipeline (reverse-engineered spec)

How the add-in drives a 2D magnetostatic solve end to end using our ported `fkern`
solver + `triangle` mesher (built from `vendor-src/femm/`), with **the bridge itself
acting as FEMM's preprocessor** — so we never port FEMM's MFC GUI/preprocessor.

```
host body ──▶ bridge ──▶ <name>.fem        (problem: materials, BCs, geometry)
                  │   └─▶ <name>.poly       (triangle PSLG with FEMM markers)
                  ├─▶ triangle  ──▶ <name>.node/.ele/.edge   (mesh + markers/attrs)
                  ├─▶ write     ──▶ <name>.pbc                (periodic/air-gap: "0\n0\n")
                  ├─▶ fkern <name> ──▶ <name>.ans             (per-node vector potential A)
                  └─▶ parse .ans ──▶ |B| field ──▶ clientGraphics heatmap in host
```

All three stages are subprocess/file based — the GPL bridge never links the AFPL
solver (see `vendor-src/femm/NOTICE.md`).

## Stage formats (decoded from `fkn/femmedoccore.cpp` + `femm/bd_writepoly.cpp`)

### `<name>.poly` — triangle PSLG input (FEMM marker conventions)
- **Nodes**: header `N 2 0 1` (N nodes, 2D, 0 attrs, **1 boundary marker**), then
  `i x y marker` per node. `marker = pointPropIndex + 2` if the node carries a point
  boundary condition, else `0`.
- **Segments**: header `M 1` (M segments, 1 marker), then `i n0 n1 marker`.
  `marker = -(linePropIndex + 2)` if the segment carries a boundary condition, else
  `0`. **Segment markers are NEGATIVE** (distinguishes them from node markers).
- **Holes**: `H` then `k hx hy` per hole point (interior of a void).
- **Regions**: `R` then `k rx ry (k+1) maxArea` per region — the region attribute
  `k+1` is the **block-label index** triangle propagates to every element via `-A`.
- Trailing `0`.

### triangle invocation
`triangle -p -P -q<minAngle> -e -A -a -z -Q -I <name>`
(`-p` PSLG, `-P` no .poly out, `-q` quality angle, `-e` emit .edge, `-A` regional
attrs = block labels, `-a` area constraint, `-z` zero-indexed, `-Q` quiet, `-I` no
iteration). Produces `<name>.node`, `<name>.ele`, `<name>.edge`.

### `<name>.node` / `<name>.ele` — read by `fkern` LoadMesh
- `.node`: `NumNodes ...`, then `idx x y marker`. fkern decodes BC:
  `bc = (marker>1) ? marker-2 : -1`. Lengths scaled to cm by `LengthUnits`.
- `.ele`: `NumEls ...`, then `idx p0 p1 p2 label`. fkern: `lbl = label-1`
  (`<0` → the default block label). This is the triangle `-A` region attribute.

### `<name>.pbc` — FEMM-specific (NOT triangle output); the bridge writes it
Periodic boundary conditions + air-gap elements. For a plain problem with neither:
```
0      <- NumPBCs
0      <- NumAGEs
```

### `<name>.fem` — problem definition, read by `fkern` OnOpenDocument
Key/value header then list sections. Header keys (case-insensitive, value after `=`):
`[Format]`, `[Frequency]` (0 = magnetostatic), `[Precision]`, `[LengthUnits]`
(inches|millimeters|centimeters|mils|microns|meters), `[ProblemType]`
(planar|axisymmetric), `[Coordinates]` (cartesian|polar). Then list sections:
`[PointProps]`, `[BdryProps]` (boundary conditions), `[BlockProps]` (materials:
Mu_x/Mu_y, H_c, J current density, Sigma, B-H curve), `[NumPoints]`, `[NumSegments]`,
`[NumHoles]`, `[NumBlockLabels]`. A block-label record is `x y BlockType MaxArea
InCircuit MagDir InGroup Turns IsExternal` (BlockType = 1-based material index).

### `<name>.ans` — solution (written by WriteStatic2D)
Same layout as `.fem` plus the mesh and the **per-node vector potential A**. |B| is
recovered from A across each element: B = ∇×(A ẑ), i.e. Bx = ∂A/∂y, By = −∂A/∂x —
constant per linear triangle. (Exact `.ans` record layout: decode `WriteStatic2D` in
`fkn/femmedoccore.cpp` when writing the parser.)

## Implementation plan (Go, in `bridge/`)
1. `femfile.go` — extend the emitter to the full `.fem` (materials + BCs + geometry).
2. `poly.go` — write `<name>.poly` with the marker conventions above.
3. `mesh.go` — run the vendored `triangle`; write `.pbc`.
4. `solve.go` — run the vendored `fkern`; locate binaries (build output / packaged).
5. `ansfile.go` — parse `.ans` → per-node A → per-element B → |B| at vertices.
6. `results.go` — replace the synthetic field with the parsed |B| (already wired to
   clientGraphics AddHeatmap + RegisterColorMapper).
7. Validate: a current-loop or solenoid with a known analytic |B|, then live end-to-end
   against the Oblikovati host before any push.
