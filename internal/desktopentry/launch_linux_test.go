package desktopentry

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exercise an actual desktop launcher parser, including field-code expansion,
// rather than reproducing its quoting rules in the test. No display is needed.
func TestGLibLaunchesLiteralExecutable(t *testing.T) {
	gio, err := exec.LookPath("gio")
	if err != nil {
		t.Skip("libglib2.0-bin is required for launcher integration testing")
	}
	directory := t.TempDir()
	executable := filepath.Join(directory, "WIS's $HOME `app` %f ; \" \\")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nprintf '%s\\n' \"$0\" \"$@\" > \"$WIS_LAUNCH_RESULT\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	result := filepath.Join(directory, "result")
	t.Setenv("WIS_LAUNCH_RESULT", result)
	for _, test := range []struct {
		name, content, want string
	}{
		{"application", Application(executable, "wis-free-v3"), executable},
		{"autostart", Autostart(executable, "wis-free-v3"), executable + "\n--background"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_ = os.Remove(result)
			path := filepath.Join(directory, "wis-free-v3.desktop")
			if err := os.WriteFile(path, []byte(test.content), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if out, err := exec.CommandContext(ctx, gio, "launch", path).CombinedOutput(); err != nil {
				t.Fatalf("gio launch: %v\n%s", err, out)
			}
			for ctx.Err() == nil {
				if content, err := os.ReadFile(result); err == nil && len(content) > 0 {
					if got := strings.TrimSuffix(string(content), "\n"); got != test.want {
						t.Fatalf("launcher ran %q, want %q", got, test.want)
					}
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Fatal("launcher did not execute the literal path")
		})
	}
}
