// Copyright 2021 The golang.design Initiative Authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.
//
// Written by Changkun Ou <changkun.de>

//go:build linux

#include <stdint.h>
#include <stdio.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/XKBlib.h>
#include <unistd.h>

extern void hotkeyDown(uintptr_t hkhandle);
extern void hotkeyUp(uintptr_t hkhandle);
extern int checkCancel(uintptr_t hkhandle);

int displayTest() {
	Display* d = NULL;
	for (int i = 0; i < 42; i++) {
		d = XOpenDisplay(0);
		if (d == NULL) continue;
		break;
	}
	if (d == NULL) {
		return -1;
	}
	return 0;
}

// waitHotkey blocks until the hotkey is triggered.
int waitHotkey(uintptr_t hkhandle, unsigned int mod, int key) {
	Display* d = NULL;
	for (int i = 0; i < 42; i++) {
		d = XOpenDisplay(0);
		if (d == NULL) continue;
		break;
	}
	if (d == NULL) {
		return -1;
	}
	
	// Optional: Ask X server to only send one release at the physical end of auto-repeat.
	Bool supported;
	XkbSetDetectableAutoRepeat(d, True, &supported);

	int keycode = XKeysymToKeycode(d, key);
	XGrabKey(d, keycode, mod, DefaultRootWindow(d), False, GrabModeAsync, GrabModeAsync);
	XSelectInput(d, DefaultRootWindow(d), KeyPressMask | KeyReleaseMask);
	XEvent ev;
	
	while(1) {
		if (checkCancel(hkhandle) == 1) {
			break;
		}
		if (XPending(d) > 0) {
			XNextEvent(d, &ev);
			switch(ev.type) {
			case KeyPress:
				hotkeyDown(hkhandle);
				continue;
			case KeyRelease:
				hotkeyUp(hkhandle);
				continue;
			}
		} else {
			usleep(10000); // Poll every 10ms for snappy responsiveness without high CPU
		}
	}
	
	XUngrabKey(d, keycode, mod, DefaultRootWindow(d));
	XCloseDisplay(d);
	return 0;
}