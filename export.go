// SPDX-License-Identifier: GPL-2.0-only

// Command oblikovati-femm-bridge is built as a c-shared library (.so/.dll) and loaded
// by the Oblikovati host at runtime. It implements the C ABI in oblikovati_addin.h
// (vendored from the oblikovati.org/api module into ./include by `make sync-header`):
// on Activate it constructs the FEMM magnetics engine (package bridge) bound to the
// host through the host-supplied callback. The host owns the model; this library owns
// the FEMM section→mesh→solve→render pipeline.
package main

/*
#cgo CFLAGS: -I${SRCDIR}/include -DOBK_BUILDING_ADDIN
#include <stdlib.h>
#include <stdint.h>
#include "oblikovati_addin.h"
*/
import "C"
import (
	"sync"
	"unsafe"

	"oblikovati.org/api"
	"oblikovati.org/femm-bridge/bridge"
)

const addInID = "com.oblikovati.femm-bridge"

var (
	idC  = C.CString(addInID)
	manC = C.CString(manifestJSON)

	mu       sync.Mutex     // guards the host callbacks and the engine
	hostCall C.ObkHostCall  // host RPC entry (set on Activate)
	hostFree C.ObkHostFree  // frees host-owned response buffers
	engine   *bridge.Engine // active magnetics engine, nil when inactive
)

//export ObkAddInId
func ObkAddInId() *C.char { return idC }

//export ObkAddInManifest
func ObkAddInManifest() *C.char { return manC }

// ObkAddInApiMajor/ObkAddInApiMinor report the oblikovati.org/api version this add-in
// was compiled against, so the host's load-time gate can refuse an incompatible build
// before activating it (see include/oblikovati_addin.h).
//
//export ObkAddInApiMajor
func ObkAddInApiMajor() C.int { return C.int(api.Major()) }

//export ObkAddInApiMinor
func ObkAddInApiMinor() C.int { return C.int(api.Minor()) }

//export ObkAddInActivate
func ObkAddInActivate(call C.ObkHostCall, freeFn C.ObkHostFree) C.int {
	mu.Lock()
	defer mu.Unlock()
	if engine != nil { // idempotent
		return C.OBK_OK
	}
	hostCall, hostFree = call, freeFn
	eng := bridge.NewEngine(cgoHostCaller{})
	engine = eng
	// Register the study command OFF the session goroutine: Activate runs on the host's session
	// goroutine before the frame loop starts, and a host call there blocks until the loop drains
	// the dispatcher — so registering inline would deadlock the head (same pattern as the MCP
	// bridge + MotorDesigner). The running frame loop drains this goroutine's host calls.
	go func() { _ = eng.Setup() }()
	return C.OBK_OK
}

//export ObkAddInDeactivate
func ObkAddInDeactivate() C.int {
	mu.Lock()
	defer mu.Unlock()
	engine = nil
	hostCall, hostFree = nil, nil
	return C.OBK_OK
}

//export ObkAddInNotify
func ObkAddInNotify(ev *C.uint8_t, n C.int) C.int {
	mu.Lock()
	eng := engine
	mu.Unlock()
	if eng == nil {
		return C.OBK_OK
	}
	eng.Notify(C.GoBytes(unsafe.Pointer(ev), n))
	return C.OBK_OK
}

//export ObkFree
func ObkFree(p *C.uint8_t) { C.free(unsafe.Pointer(p)) }

// main is required for a Go program but never runs: this binary is built with
// -buildmode=c-shared, so the host loads it as a library and calls the //export'd
// ObkAddIn* entry points directly — there is no executable entry point.
func main() {
	// Intentionally empty — see the doc comment above (c-shared has no entry point).
}
