// PORT (Oblikovati FEMM bridge, mac/linux): headless replacement for FEMM's MFC
// progress dialog (CFknDlg). The solver holds a `TheView` pointer and reports
// progress/status through it; here those calls become no-ops so the numerics run
// without a GUI. Governed by vendor-src/femm/NOTICE.md (third-party build glue).
#ifndef FKN_DLG_SHIM_H
#define FKN_DLG_SHIM_H

#include "femmcompat.h"

class CFknDlg {
public:
	CProgressCtrl m_prog1;
	CProgressCtrl m_prog2;
	void *m_hWnd; // never dereferenced; presence satisfies IsWindow(TheView->m_hWnd)

	CFknDlg() : m_hWnd(0) {}

	void SetDlgItemText(int, const char *) {}
	void SetWindowText(const char *) {}
	void InvalidateRect(void *, BOOL) {}
	void UpdateWindow() {}
};

// IsWindow is called as IsWindow(TheView->m_hWnd) in the solver's status loop; with
// a headless view there is no window to wait on, so report "ready" immediately.
inline BOOL IsWindow(void *) { return TRUE; }

#endif // FKN_DLG_SHIM_H
