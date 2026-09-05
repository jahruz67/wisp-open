//go:build linux

package main

import "testing"

func TestLinuxInstallYdotoolCommand(t *testing.T) {
	tests := []struct {
		name string
		id   string
		like string
		want string
	}{
		{name: "Ubuntu", id: "ubuntu", like: "debian", want: "sudo apt install ydotool"},
		{name: "Mint", id: "linuxmint", like: "ubuntu debian", want: "sudo apt install ydotool"},
		{name: "Fedora", id: "fedora", want: "sudo dnf install ydotool"},
		{name: "Arch", id: "arch", want: "sudo pacman -S ydotool"},
		{name: "Manjaro", id: "manjaro", like: "arch", want: "sudo pacman -S ydotool"},
		{name: "openSUSE", id: "opensuse-tumbleweed", like: "suse opensuse", want: "sudo zypper install ydotool"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := linuxInstallYdotoolCommand(test.id, test.like); got != test.want {
				t.Fatalf("linuxInstallYdotoolCommand() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLinuxShortcutCommandQuotesExecutable(t *testing.T) {
	got := linuxShortcutCommand("/home/Test User/WIS's app")
	want := "'/home/Test User/WIS'\\''s app' --action=toggle"
	if got != want {
		t.Fatalf("linuxShortcutCommand() = %q, want %q", got, want)
	}
}
