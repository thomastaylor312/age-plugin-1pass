// Command age-plugin-1pass is an age identity plugin that stores private keys
// in 1Password and resolves them at decrypt time via the 1Password desktop app.
//
// When invoked by age with --age-plugin=identity-v1, it runs the plugin state
// machine. Otherwise it dispatches to subcommands (currently: generate).
package main

import (
	"fmt"
	"os"
)

// version is stamped at build time via -ldflags. Defaults to "dev" for local
// builds.
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// age spawns the plugin binary with --age-plugin=<state-machine>. Detect
	// that first so it takes precedence over normal subcommand dispatch.
	for _, a := range args {
		if a == "--age-plugin=identity-v1" || a == "-age-plugin=identity-v1" {
			// Plugin mode wiring lands in a later commit.
			fmt.Fprintln(os.Stderr, "age-plugin-1pass: plugin mode not yet implemented")
			return 1
		}
	}

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
