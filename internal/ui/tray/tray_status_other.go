//go:build !linux

package tray

func LinuxHostStatus() HostStatus { return HostStatus{} }
