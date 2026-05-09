//go:build unix && !windows

package main

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

const instanceSockName = "instance.sock"

// secondInstanceWake is closed when a second process asks the running app to show its window.
var secondInstanceWake = make(chan struct{}, 8)

func instanceSocketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDir, instanceSockName), nil
}

func tryNotifyRunningInstanceToShow() {
	path, err := instanceSocketPath()
	if err != nil {
		return
	}
	c, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err != nil {
		return
	}
	defer c.Close()
	_, _ = c.Write([]byte{1})
}

func runSecondInstanceListener() {
	path, err := instanceSocketPath()
	if err != nil {
		return
	}
	_ = os.Remove(path)
	l, err := net.Listen("unix", path)
	if err != nil {
		return
	}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_, _ = io.Copy(io.Discard, conn)
				select {
				case secondInstanceWake <- struct{}{}:
				default:
				}
			}(c)
		}
	}()
}
