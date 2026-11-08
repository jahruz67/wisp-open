// desktop-entry renders the same launcher used by the running Linux app.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"wis-free-v3/internal/desktopentry"
)

func main() {
	executable := flag.String("executable", "", "absolute installed executable path")
	icon := flag.String("icon", "wis-free-v3", "icon name or absolute path")
	flag.Parse()
	if !filepath.IsAbs(*executable) {
		fmt.Fprintln(os.Stderr, "--executable must be an absolute path")
		os.Exit(1)
	}
	fmt.Print(desktopentry.Application(*executable, *icon))
}
