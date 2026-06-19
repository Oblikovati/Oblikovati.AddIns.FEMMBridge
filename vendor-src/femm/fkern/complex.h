// PORT (mac/linux): FEMM ships CComplex twice (solver + liblua); liblua's is a
// superset (adds comparison operators + atan2). Unify on it to avoid duplicate
// symbols at link, and let liblua/COMPLEX.CPP be the single definition.
#include "../liblua/femmcomplex.h"
