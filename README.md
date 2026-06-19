# Oblikovati FEMM Bridge

A host add-in that integrates **FEMM** (Finite Element Method Magnetics — David
Meeker's 2D/axisymmetric magnetostatics solver) as a **simulation provider** for
Oblikovati. It links **only** the Apache-2.0 public API (`oblikovati.org/api`) and
reaches the running host over the C ABI (ADR-0016) — never the GPL application
internals. This is the first proof that FEA/simulation can ship entirely as add-ins
against the v1 API.

> Built/versioned/shipped exactly like the [MCP bridge](../Oblikovati.AddIns.MCPBridge):
> a cgo `c-shared` library, its own Go module pinned to a published `oblikovati.org/api`
> release, sibling repos wired by `.github/actions/siblings`, and an hourly
> API-tracking release pipeline.

## Pipeline

1. **Section** — pull a body's 2D boundary loops + per-face partition
   (`Body.CalculateStrokes` / `Body.CalculateFacets`).
2. **Materials** — resolve each region's material (`Materials.List`/`Get`).
3. **Emit `.fem`** — geometry + block labels + boundary conditions (FEMM 4.2 format).
4. **Mesh + solve** — Triangle meshes; the vendored headless `fkern` solves → `.ans`
   *(Phase 2; the current build pushes a synthetic field so the render path is live)*.
5. **Render** — push the |B| field back as a `clientGraphics` heatmap + color-mapper
   legend.

## API-gap findings (this add-in is also the gap audit for the v1 API)

Driving a real solver surfaced four v1 API gaps (tracked upstream in the app repo):

1. **No analytic surface/curve evaluator over the wire** — meshers only get facets.
2. **No edge identity in stroke results** — can't bind a boundary condition to a
   specific B-rep edge (faces are fine).
3. **No magnetic material properties** — `types.Material` has Mechanical/Thermal/
   Electrical but no μ_r / coercivity / B-H curve, so the add-in carries its own
   magnetic library. *(σ is recoverable from electrical resistivity.)*
4. **Client-graphics results don't persist in `.obk`** — overlays are live-only.

See `bridge/` for the inline `GAP #n` markers at each call site.

## Build

```sh
make build      # cgo c-shared library into build/
make install    # build + copy library + manifest into the host's add-ins dir
make test       # cgo-free bridge unit tests + bridge<->host integration
```

Local dev resolves the sibling `Oblikovati.API` + `Oblikovati` checkouts via a
git-ignored `go.work`; CI injects the equivalent replaces (`.github/actions/siblings`).

## Layout

```
export.go / hostcaller.go / manifest.go   C-ABI c-shared shell (the only cgo)
manifest.json                             add-in manifest (capabilities)
bridge/                                    cgo-free magnetics engine + pipeline
  engine.go      section→materials→.fem→render orchestration + HostCaller
  section.go     host geometry → 2D loops
  material.go    host materials → FEMM block materials (μ_r gap)
  femfile.go     FEMM 4.2 .fem emitter
  results.go     |B| field → clientGraphics heatmap
vendor-src/                                Phase 2: ported headless fkern + Triangle
```

## Licensing (read before vendoring or shipping the solver)

This repository is **GPL-2.0-only** — the add-in's own code is GPL; the only thing it
links is the Apache-2.0 public `oblikovati.org/api`.

**FEMM is NOT GPL.** It is distributed under the **Aladdin Free Public License (AFPL)**
— a non-GPL, **non-commercial**, GPL-incompatible license. All vendored FEMM source and
the macOS/Linux build tooling for it live under [`vendor-src/femm/`](./vendor-src/femm/)
and are governed **entirely** by the upstream licenses documented in
[`vendor-src/femm/NOTICE.md`](./vendor-src/femm/NOTICE.md) (AFPL for FEMM; separate
licenses for Triangle and Lua) — **never** by this repo's GPL.

To keep the two legally separate, the AFPL solver is built as a **standalone binary**
the bridge runs **as a subprocess** (arm's-length, file-based `.fem`/`.ans` exchange);
the GPL bridge never links FEMM into its own process. See the NOTICE for the
non-commercial distribution caveat.
