//go:build linux

// ============================================================
// LINUX-ONLY FILE — Sets the GTK program name for the system tray
// on Linux. This is required for proper desktop integration.
// ============================================================

package tray

/*
#cgo pkg-config: glib-2.0
#include <glib.h>
*/
import "C"
import "unsafe"

func init() {
	cstr := C.CString("wis-free-v3")
	defer C.free(unsafe.Pointer(cstr))
	C.g_set_prgname(cstr)
}
