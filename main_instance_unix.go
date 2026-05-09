//go:build unix && !windows

package main

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const instanceSockName = "instance.sock"

// secondInstanceWake is closed when a second process asks the running app to show its window.
var secondInstanceWake = make(chan struct{}, 8)

// secondInstanceCommand receives commands from a spawned second process (e.g. desktop shortcut).
// Commands are 1-byte values written to the unix socket.
var secondInstanceCommand = make(chan byte, 16)

const (
	instanceCmdShow   byte = 1
	instanceCmdStart  byte = 2
	instanceCmdStop   byte = 3
	instanceCmdToggle byte = 4
)

func instanceSocketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDir, instanceSockName), nil
}

func tryNotifyRunningInstance(cmd byte) bool {
	path, err := instanceSocketPath()
	if err != nil {
		return false
	}
	c, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err != nil {
		return false
	}
	defer c.Close()
	if cmd == 0 {
		cmd = instanceCmdShow
	}
	_, _ = c.Write([]byte{cmd})
	return true
}

func tryNotifyRunningInstanceToShow() bool {
	return tryNotifyRunningInstance(instanceCmdShow)
}

func tryNotifyRunningInstanceAction(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "show":
		return tryNotifyRunningInstance(instanceCmdShow)
	case "start":
		return tryNotifyRunningInstance(instanceCmdStart)
	case "stop":
		return tryNotifyRunningInstance(instanceCmdStop)
	case "toggle":
		return tryNotifyRunningInstance(instanceCmdToggle)
	default:
		return false
	}
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
				buf := make([]byte, 1)
				n, _ := conn.Read(buf)
				if n == 1 {
					select {
					case secondInstanceCommand <- buf[0]:
					default:
					}
				} else {
					// Backwards-compatible: any connection with no payload = show window.
					select {
					case secondInstanceCommand <- instanceCmdShow:
					default:
					}
				}
				_, _ = io.Copy(io.Discard, conn)
				select {
				case secondInstanceWake <- struct{}{}:
				default:
				}
			}(c)
		}
	}()
}
