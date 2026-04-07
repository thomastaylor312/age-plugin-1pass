// Command age-plugin-1pass is an age identity plugin that stores private keys
// in 1Password and resolves them at decrypt time via the 1Password desktop app.
//
// When invoked by age with --age-plugin=identity-v1, it runs the plugin state
// machine. Otherwise it dispatches to subcommands (currently: generate).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/thomastaylor312/age-plugin-1pass/internal/cmd"
)

// version is stamped at build time via -ldflags. Defaults to "dev" for local
// builds.
var version = "dev"

func main() {
	// age spawns the plugin binary with --age-plugin=<state-machine>. Detect
	// that first so it takes precedence over normal subcommand dispatch.
	// We don't need the exact value (identity-v1 vs recipient-v1) here —
	// plugin.Main handles dispatch internally.
	for _, a := range os.Args[1:] {
		if strings.HasPrefix(a, "--age-plugin=") || strings.HasPrefix(a, "-age-plugin=") {
			os.Exit(cmd.RunPlugin(version))
		}
	}

	os.Exit(runCLI(os.Args[1:]))
}

func runCLI(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}

	switch args[0] {
	case "generate":
		fmt.Fprintln(os.Stderr, "age-plugin-1pass: generate not yet implemented")
		return 1
	case "-h", "--help", "help":
		usage()
		return 0
	case "-V", "--version", "version":
		fmt.Println(version)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "age-plugin-1pass: unknown command %q\n", args[0])
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `age-plugin-1pass — age identity plugin backed by 1Password

Usage:
    age-plugin-1pass generate --account <acct> --vault <vault> [--name <name>] [-o OUTPUT] [-pq]
    age-plugin-1pass --version

When invoked by age as "age-plugin-1pass --age-plugin=identity-v1", it runs
the age plugin protocol and resolves the wrapped private key from 1Password.
`)
}
