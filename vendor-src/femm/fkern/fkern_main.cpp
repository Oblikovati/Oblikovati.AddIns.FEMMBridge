// PORT (Oblikovati FEMM bridge, mac/linux): headless console entry point for the
// FEMM magnetostatics solver, replacing FEMM's MFC application + file-dialog
// old_main. Reads a problem path from argv[1], loads the .fem + mesh, renumbers,
// solves (static or harmonic, planar or axisymmetric), and writes the .ans
// solution — exactly the dispatch FEMM's old_main performed, minus the GUI.
//
// Governed by vendor-src/femm/NOTICE.md: this glue builds the AFPL-licensed FEMM
// solver headless; it is not under this repository's GPL.
#include <stdafx.h>
#include "fkn.h"
#include "complex.h"
#include "spars.h"
#include "mesh.h"
#include "femmedoccore.h"
#include "lua.h"

// The solver's nonlinear/circuit expression evaluator uses one global Lua state
// (declared `extern lua_State *lua;` in prob1big/prob3big). FEMM's MFC app created
// it on startup; we do the same here.
lua_State *lua;
extern void lua_baselibopen(lua_State *L);
extern void lua_iolibopen(lua_State *L);
extern void lua_strlibopen(lua_State *L);
extern void lua_mathlibopen(lua_State *L);
extern void lua_dblibopen(lua_State *L);

static void initLua() {
	lua = lua_open(4096);
	lua_baselibopen(lua);
	lua_strlibopen(lua);
	lua_mathlibopen(lua);
	lua_iolibopen(lua);
	lua_dblibopen(lua);
}

// solveStatic runs the magnetostatic (Frequency == 0) path.
static int solveStatic(CFemmeDocCore &Doc) {
	CBigLinProb L;
	L.Precision = Doc.Precision;
	if (L.Create(Doc.NumNodes, Doc.BandWidth) == FALSE) {
		fprintf(stderr, "fkern: couldn't allocate matrices\n");
		return 4;
	}
	BOOL ok = (Doc.ProblemType == FALSE) ? Doc.Static2D(L) : Doc.StaticAxisymmetric(L);
	if (ok == FALSE) {
		fprintf(stderr, "fkern: couldn't solve the problem\n");
		return 5;
	}
	if (Doc.WriteStatic2D(L) == FALSE) {
		fprintf(stderr, "fkern: couldn't write results\n");
		return 6;
	}
	return 0;
}

// solveHarmonic runs the time-harmonic (Frequency > 0) path.
static int solveHarmonic(CFemmeDocCore &Doc) {
	CBigComplexLinProb L;
	L.Precision = Doc.Precision;
	if (L.Create(Doc.NumNodes + Doc.NumCircProps, Doc.BandWidth, Doc.NumNodes) == FALSE) {
		fprintf(stderr, "fkern: couldn't allocate matrices\n");
		return 4;
	}
	BOOL ok = (Doc.ProblemType == FALSE) ? Doc.Harmonic2D(L) : Doc.HarmonicAxisymmetric(L);
	if (ok == FALSE) {
		fprintf(stderr, "fkern: couldn't solve the problem\n");
		return 5;
	}
	if (Doc.WriteHarmonic2D(L) == FALSE) {
		fprintf(stderr, "fkern: couldn't write results\n");
		return 6;
	}
	return 0;
}

int main(int argc, char **argv) {
	if (argc < 2) {
		fprintf(stderr, "usage: fkern <problem>   (problem.fem + mesh in the same dir)\n");
		return 1;
	}
	initLua();

	CFknDlg view;
	CFemmeDocCore Doc;
	char pathName[2048];
	strncpy(pathName, argv[1], sizeof(pathName) - 1);
	pathName[sizeof(pathName) - 1] = 0;
	Doc.PathName = pathName;
	Doc.TheView = &view;

	if (Doc.OnOpenDocument() != TRUE) {
		fprintf(stderr, "fkern: problem loading .fem file '%s'\n", pathName);
		return 7;
	}
	if (Doc.LoadMesh() != TRUE) {
		fprintf(stderr, "fkern: problem loading mesh\n");
		return 2;
	}
	if (Doc.PrevType == 0 && Doc.Cuthill() != TRUE) {
		fprintf(stderr, "fkern: problem renumbering nodes\n");
		return 3;
	}

	int rc = (Doc.Frequency == 0) ? solveStatic(Doc) : solveHarmonic(Doc);
	if (rc == 0)
		printf("fkern: solved %s (%d nodes, %d elements)\n", pathName, Doc.NumNodes, Doc.NumEls);
	return rc;
}
