// PORT (Oblikovati FEMM bridge, mac/linux): MFC compatibility shim.
//
// FEMM 4.2's fkern solver was written against MFC (Visual C++). This header
// supplies just enough of the MFC surface the SOLVER numerics actually use so the
// code builds headless with clang/gcc on macOS and Linux, with NO Win32/MFC.
//
// It is part of the build glue for the AFPL-licensed FEMM sources and is governed by
// vendor-src/femm/NOTICE.md (third-party-licensed area, not this repo's GPL). The
// upstream solver files are unmodified except for this shim being reachable via the
// redirect headers (stdafx.h / afx*.h in this directory).
#ifndef FEMM_COMPAT_H
#define FEMM_COMPAT_H

#include <cstdio>
#include <cstdarg>
#include <cstring>
#include <cstdlib>
#include <string>
#include <unistd.h>

// --- Win32 scalar types the numerics use --------------------------------------
typedef int BOOL;
#ifndef TRUE
#define TRUE 1
#endif
#ifndef FALSE
#define FALSE 0
#endif
#ifndef NULL
#define NULL 0
#endif
typedef unsigned char BYTE;
typedef unsigned int UINT;
typedef unsigned long DWORD;

// --- Win32 CRT/API name aliases the solver uses -------------------------------
#define _strnicmp strncasecmp
#define _stricmp strcasecmp
#define DeleteFile(f) remove(f)
#ifndef __min
#define __min(a, b) ((a) < (b) ? (a) : (b))
#endif
#ifndef __max
#define __max(a, b) ((a) > (b) ? (a) : (b))
#endif

// --- CString: minimal std::string-backed replacement --------------------------
// Implements exactly the surface fkern uses: construction/assignment from char*,
// implicit const char* (for strcpy/printf), GetLength, Left, and printf-style Format.
class CString {
	std::string s;

public:
	CString() {}
	CString(const char *p) : s(p ? p : "") {}
	CString(const CString &o) : s(o.s) {}
	CString &operator=(const char *p) { s = p ? p : ""; return *this; }
	CString &operator=(const CString &o) { s = o.s; return *this; }
	CString &operator+=(const char *p) { if (p) s += p; return *this; }
	CString &operator+=(const CString &o) { s += o.s; return *this; }
	operator const char *() const { return s.c_str(); }
	int GetLength() const { return (int)s.size(); }
	void Empty() { s.clear(); }
	CString Left(int n) const { return CString(s.substr(0, n < 0 ? 0 : n).c_str()); }
	CString Mid(int start) const {
		if (start < 0 || start >= (int)s.size()) return CString();
		return CString(s.substr(start).c_str());
	}
	CString Mid(int start, int n) const {
		if (start < 0 || start >= (int)s.size()) return CString();
		return CString(s.substr(start, n < 0 ? 0 : n).c_str());
	}
	bool operator==(const char *p) const { return s == (p ? p : ""); }

	// printf-style formatting (fkern uses simple %i/%g/%e specifiers).
	void Format(const char *fmt, ...) {
		char buf[1024];
		va_list ap;
		va_start(ap, fmt);
		vsnprintf(buf, sizeof(buf), fmt, ap);
		va_end(ap);
		s = buf;
	}
};

// --- Diagnostics: AfxMessageBox / MsgBox go to stderr (no GUI) -----------------
// AfxMessageBox takes a plain string; FEMM's MsgBox is printf-style (fkern calls it
// as MsgBox("Bandwidth = %i", n)), so it is variadic.
inline int AfxMessageBox(const char *msg) {
	std::fprintf(stderr, "fkern: %s\n", msg ? msg : "");
	return 0;
}
inline void MsgBoxf(const char *fmt, ...) {
	char buf[1024];
	va_list ap;
	va_start(ap, fmt);
	vsnprintf(buf, sizeof(buf), fmt, ap);
	va_end(ap);
	std::fprintf(stderr, "fkern: %s\n", buf);
}
#ifndef MsgBox
#define MsgBox(...) MsgBoxf(__VA_ARGS__)
#endif

// --- Sleep (ms): the solver yields cooperatively; map to usleep ---------------
inline void Sleep(unsigned long ms) { usleep(ms * 1000); }

// --- CProgressCtrl: the dialog's progress bars become no-ops -------------------
class CProgressCtrl {
public:
	void SetPos(int) {}
	void SetRange(short, short) {}
	void SetStep(int) {}
	void StepIt() {}
};

#endif // FEMM_COMPAT_H
