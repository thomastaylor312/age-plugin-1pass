// Command age-plugin-1pass is an age identity plugin that stores private keys
// in 1Password and resolves them at decrypt time via the 1Password desktop app.
//
// When invoked by age with --age-plugin=identity-v1, it runs the plugin state
// machine. Otherwise it dispatches to subcommands (currently: generate).
package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"

	"github.com/thomastaylor312/age-plugin-1pass/internal/cmd"
)

// version is stamped at build time via -ldflags. Defaults to "dev" for local
// builds.
var version = "dev"

func main() {
	os.Exit(mainRun(os.Args[1:]))
}

func mainRun(args []string) int {
	// age spawns the plugin binary with --age-plugin=<state-machine>. Detect
	// that before any subcommand/flag parsing so it takes precedence over
	// normal CLI dispatch. plugin.Main handles flag parsing itself.
	for _, a := range args {
		if strings.HasPrefix(a, "--age-plugin=") || strings.HasPrefix(a, "-age-plugin=") {
			return cmd.RunPlugin(version)
		}
	}

	// Subcommand dispatch: the first positional arg is the subcommand. Any
	// top-level flags (e.g. --version) are handled before that.
	fs := pflag.NewFlagSet("age-plugin-1pass", pflag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	showVersion := fs.BoolP("version", "V", false, "print version and exit")
	fs.Usage = usage
	// Stop parsing at the first non-flag arg so `generate --pq` etc. reach
	// the subcommand untouched.
	fs.SetInterspersed(false)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *showVersion {
		fmt.Println(version)
		return 0
	}

	rest := fs.Args()
	if len(rest) == 0 {
		usage()
		return 2
	}

	switch rest[0] {
	case "generate":
		return cmd.RunGenerate(rest[1:], version)
	case "help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "age-plugin-1pass: unknown command %q\n", rest[0])
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `age-plugin-1pass — age identity plugin backed by 1Password

Usage:
    age-plugin-1pass generate --account <acct> --vault <vault> [--name <name>] [-o FILE] [--pq]
    age-plugin-1pass --version

When invoked by age as "age-plugin-1pass --age-plugin=identity-v1", it runs
the age plugin protocol and resolves the wrapped private key from 1Password.
`)
}
