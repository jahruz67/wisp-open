//go:build darwin

package darwin

/*
#include <stdbool.h>
void *wis_duck_output(void);
bool wis_restore_output(void *state);
*/
import "C"

import "wis-free-v3/internal/logger"

// DuckMedia lowers output to 1% of its current volume and returns its restoration.
func DuckMedia() func() {
	state := C.wis_duck_output()
	if state == nil {
		logger.Error("Failed to lower output volume")
		return nil
	}
	return func() {
		if !bool(C.wis_restore_output(state)) {
			logger.Error("Failed to restore output volume")
		}
	}
}
