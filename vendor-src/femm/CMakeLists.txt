# PORT (Oblikovati FEMM bridge): cross-platform build for the headless FEMM
# magnetics toolchain on macOS and Linux (clang/gcc). Produces:
#   * fkern    — the magnetostatics solver (reads .fem + mesh, writes .ans)
#   * triangle — Shewchuk's 2D mesh generator
#
# This builds the third-party, separately-licensed sources under vendor-src/femm/
# (Aladdin Free Public License for FEMM; Triangle's and Lua's own terms). See
# vendor-src/femm/NOTICE.md — none of this is under the repository's GPL. The
# add-in invokes these as standalone binaries (subprocess), never linking them.
cmake_minimum_required(VERSION 3.16)
project(femm_headless CXX C)

set(CMAKE_CXX_STANDARD 11)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

if(NOT CMAKE_BUILD_TYPE)
  set(CMAKE_BUILD_TYPE Release)
endif()

# LINUX/APPLE both take the POSIX paths in the ported sources (the Win32-only code
# is guarded behind _WIN32).
add_compile_definitions(LINUX)

# --- liblua (FEMM's bundled Lua 4, compiled as C++; Number type is CComplex) ----
# liblua's COMPLEX.CPP is the SINGLE CComplex definition (its header is a superset
# of the solver's — adds comparison operators + atan2). The solver's complex.h is
# redirected here and its complex.cpp is dropped from the fkern target, so CComplex
# is defined exactly once and there are no duplicate-symbol link errors.
file(GLOB LUA_SRC "${CMAKE_CURRENT_SOURCE_DIR}/liblua/*.cpp")
list(APPEND LUA_SRC "${CMAKE_CURRENT_SOURCE_DIR}/liblua/COMPLEX.CPP")
add_library(femmlua STATIC ${LUA_SRC})
target_include_directories(femmlua PUBLIC "${CMAKE_CURRENT_SOURCE_DIR}/liblua")

# --- triangle (Shewchuk mesher) -------------------------------------------------
# Standalone mesher: do NOT define TRILIBRARY (that #ifdef strips main()).
add_executable(triangle "${CMAKE_CURRENT_SOURCE_DIR}/triangle/triangle.c")
if(UNIX)
  target_link_libraries(triangle m)
endif()

# --- fkern (magnetostatics solver) ---------------------------------------------
set(FKERN_SRC
  spars.cpp cspars.cpp cuthill.cpp fullmatrix.cpp matprop.cpp
  prob1big.cpp prob2big.cpp prob3big.cpp prob4big.cpp femmedoccore.cpp
  fkern_main.cpp)
list(TRANSFORM FKERN_SRC PREPEND "${CMAKE_CURRENT_SOURCE_DIR}/fkern/")
add_executable(fkern ${FKERN_SRC})
target_include_directories(fkern PRIVATE
  "${CMAKE_CURRENT_SOURCE_DIR}/fkern"
  "${CMAKE_CURRENT_SOURCE_DIR}/liblua")
target_link_libraries(fkern femmlua)
if(UNIX)
  target_link_libraries(fkern m)
endif()
