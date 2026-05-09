//go:build linux
package tray

/*
#include <glib.h>
*/
import "C"
import "unsafe"

func init() {
	cstr := C.CString("wis-free-v3")
	defer C.free(unsafe.Pointer(cstr))
	C.g_set_prgname(cstr)
}
