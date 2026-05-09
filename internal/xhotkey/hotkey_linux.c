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

// Effective modifier masks for XGrabKey when NumLock / CapsLock are on.
// See: https://stackoverflow.com/questions/4037230/how-to-handle-global-hotkeys-with-x11-xlib
static unsigned int mod_masks[4];

static void init_mod_masks(unsigned int mod) {
	mod_masks[0] = mod;
	mod_masks[1] = mod | Mod2Mask;
	mod_masks[2] = mod | LockMask;
	mod_masks[3] = mod | Mod2Mask | LockMask;
}

static void grab_all(Display* d, int keycode, unsigned int mod, Window w) {
	init_mod_masks(mod);
	for (int i = 0; i < 4; i++) {
		XGrabKey(d, keycode, mod_masks[i], w, False, GrabModeAsync, GrabModeAsync);
	}
}

static void ungrab_all(Display* d, int keycode, unsigned int mod, Window w) {
	init_mod_masks(mod);
	for (int i = 0; i < 4; i++) {
		XUngrabKey(d, keycode, mod_masks[i], w);
	}
}

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
	XCloseDisplay(d);
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
	if (keycode == 0) {
		XCloseDisplay(d);
		return -1;
	}

	Window root = DefaultRootWindow(d);
	grab_all(d, keycode, mod, root);
	XEvent ev;
	
	while(1) {
		if (checkCancel(hkhandle) == 1) {
			break;
		}
		if (XPending(d) > 0) {
			XNextEvent(d, &ev);
			switch(ev.type) {
			case KeyPress:
				if (ev.xkey.keycode == (unsigned)keycode) {
					hotkeyDown(hkhandle);
				}
				continue;
			case KeyRelease:
				if (ev.xkey.keycode == (unsigned)keycode) {
					hotkeyUp(hkhandle);
				}
				continue;
			}
		} else {
			usleep(10000); // Poll every 10ms for snappy responsiveness without high CPU
		}
	}
	
	ungrab_all(d, keycode, mod, root);
	XCloseDisplay(d);
	return 0;
}