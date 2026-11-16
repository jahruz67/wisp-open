//go:build darwin

package darwin

/*
#include <stdbool.h>
void *wis_mute_output(void);
bool wis_restore_output_mute(void *state);
*/
import "C"

import "wis-free-v3/internal/logger"

// MuteOutput mutes output and returns a function to restore its previous mute state.
func MuteOutput() func() {
	state := C.wis_mute_output()
	if state == nil {
		logger.Error("Failed to mute output")
		return nil
	}
	return func() {
		if !bool(C.wis_restore_output_mute(state)) {
			logger.Error("Failed to restore output mute state")
		}
	}
}
