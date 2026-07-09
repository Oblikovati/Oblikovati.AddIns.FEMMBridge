// The oblikovati-femm-bridge add-in: a c-shared library (.so/.dll) loaded by the
// host at runtime, integrating FEMM (Finite Element Method Magnetics) as a 2D
// magnetostatics simulation provider. It pulls a section + materials from the host
// over the Apache-2.0 API, meshes + solves with a vendored headless FEMM solver, and
// renders the |B| field back as client graphics. Its own module so the solver-bridge
// deps stay independent of the host — the runtime boundary is the C ABI, not Go (see
// ../include/oblikovati_addin.h).
//
// The SHIPPED library links only the Apache-2.0 contract (oblikovati.org/api). The
// require on the GPL application module (oblikovati) is TEST-SCOPE ONLY — the
// bridge↔real-host integration tests drive the live router/model. Both modules are
// sibling repos resolved by the go.work workspace at this repo's root (no committed
// replace); CI injects the equivalent replaces via .github/actions/siblings.
module oblikovati.org/femm-bridge

go 1.24.0

require oblikovati.org/api v0.119.0
