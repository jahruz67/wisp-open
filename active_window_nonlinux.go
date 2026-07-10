//go:build !linux

package main

import "github.com/go-vgo/robotgo"

func activeWindowTitle() string {
	return robotgo.GetTitle()
}
