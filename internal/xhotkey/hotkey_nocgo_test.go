// Copyright 2021 The golang.design Initiative Authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.
//
// Written by Changkun Ou <changkun.de>

//go:build darwin && !cgo

package hotkey_test

import (
	"testing"

	"wis-free-v3/internal/xhotkey"
)

// Without CGO on Darwin, registration is unsupported (panic). Linux without CGO uses the portal backend instead.
func TestHotkey(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			return
		}
		t.Fatalf("expect to fail when CGO_ENABLED=0")
	}()

	hk := hotkey.New([]hotkey.Modifier{}, hotkey.Key(0))
	err := hk.Register()
	if err != nil {
		t.Fatal(err)
	}
	hk.Unregister()
}
