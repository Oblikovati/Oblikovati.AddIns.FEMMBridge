# Third-party code — NOT under this repository's GPL license

**Everything in this directory (`vendor-src/femm/`) and all of its subdirectories is
third-party software governed by the upstream licenses named below. None of it is
covered by the `GPL-2.0-only` license that applies to the rest of this repository.**

This includes both the vendored solver source code **and** any build tooling added
here to compile it on macOS/Linux (CMake files, shell/Make scripts, patches). Those
build files exist solely to build this third-party code and do not place it — or
themselves — under the GPL. Do not add GPL-licensed or repository-original product
code under this directory; keep it strictly to vendored upstream sources and the
glue needed to build them.

## Licenses that apply here

| Component | Author / © | License | Text |
|-----------|------------|---------|------|
| **FEMM** (Finite Element Method Magnetics) — the `fkern` magnetostatics solver and supporting sources | David C. Meeker, dmeeker@ieee.org | **Aladdin Free Public License (AFPL), v8 (1999)** | [`LICENSE.txt`](./LICENSE.txt) |
| **Triangle** — 2D Delaunay mesh generator | Jonathan Shewchuk | Its own license (NOT AFPL) | appended in [`LICENSE.txt`](./LICENSE.txt) |
| **Lua** — embedded scripting (if vendored) | Lua authors / PUC-Rio | Lua/MIT license | appended in [`LICENSE.txt`](./LICENSE.txt) |

The full, verbatim upstream license text is preserved in [`LICENSE.txt`](./LICENSE.txt)
exactly as distributed with FEMM 4.2 (21 Apr 2019); upstream release notes are in
[`UPSTREAM-README.txt`](./UPSTREAM-README.txt). The AFPL itself states, in its own
words, that it *"is not the same as any of the GNU Licenses … Its terms are
substantially different."*

## Why this is segregated, and how the add-in stays license-clean

The AFPL is **incompatible with the GPL** and carries **non-commercial use/distribution
restrictions** (read `LICENSE.txt` in full before redistributing). To keep the GPL
add-in and the AFPL solver legally separate, the solver is built and shipped as a
**standalone executable** that the bridge invokes **at arm's length as a subprocess**,
exchanging data only through FEMM's own `.fem` / `.ans` files. The GPL bridge does
**NOT** statically or dynamically link the FEMM/AFPL code into its own address space —
that arms-length, mere-aggregation boundary is what keeps each component under its own
license.

**Distribution caveat:** because of the AFPL's non-commercial terms, the built FEMM
solver binary must not be bundled into a commercial distribution of the product
without separate clearance. Treat it as an optional, separately-obtained component
(the end user supplies/builds FEMM) unless and until its licensing is resolved.
