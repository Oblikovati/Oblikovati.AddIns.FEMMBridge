// PORT (Oblikovati FEMM bridge, mac/linux): headless replacement for FEMM's MFC
// application header (originally declared CFknApp : CWinApp + old_main, behind an
// __AFXWIN_H__ precompiled-header guard). The solver numerics include this only for
// the shared types and old_main; the MFC app class is dropped. Governed by
// vendor-src/femm/NOTICE.md (third-party build glue).
#ifndef FKN_SHIM_H
#define FKN_SHIM_H

#include "femmcompat.h"
#include "fknDlg.h"
#include "resource.h"

void old_main(void *inptr);

#endif // FKN_SHIM_H
