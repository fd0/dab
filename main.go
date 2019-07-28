package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

type GlobalOptions struct {
	DryRun  bool
	Verbose bool
	Debug   bool
	Target  string
	Base    string
}

var globalOpts GlobalOptions

func init() {
	defaultTarget := os.Getenv("TARGET")
	if defaultTarget == "" {
		defaultTarget = os.Getenv("HOME")
	}

	fs := cmdRoot.PersistentFlags()
	fs.StringVar(&globalOpts.Target, "target", defaultTarget, "set target directory")
	fs.StringVar(&globalOpts.Base, "base", "", "set base directory")
	fs.BoolVar(&globalOpts.DryRun, "dry-run", false, "only print actions, do not execute them")
	fs.BoolVar(&globalOpts.Verbose, "verbose", false, "be verbose")
	fs.BoolVar(&globalOpts.Debug, "debug", false, "print debugging information")
}

var cmdRoot = &cobra.Command{
	Use:           "dab",
	Short:         "manage dotfiles and bundles",
	SilenceErrors: true,
	SilenceUsage:  true,

	PreRunE: func(cmd *cobra.Command, args []string) error {
		if globalOpts.Base == "" {
			globalOpts.Base = findBasedir()
		}

		if globalOpts.Base == "" {
			return fmt.Errorf("unable to find basedir")
		}

		return nil
	},
}

func findBasedir() string {
	exe := os.Args[0]

	dir, err := filepath.Abs(filepath.Dir(exe))
	ok(err)

	fi, err := os.Lstat(exe)
	ok(err)

	if isSymlink(fi) {
		dir = readlink(exe)
	}

	v("looking for basedir in %v\n", dir)

	for {
		if filepath.Dir(dir) == dir {
			warn("unable to find bundles.json, pass base directory with `--base`\n")
			os.Exit(1)
		}

		filename := filepath.Join(dir, "bundles.json")
		fi, err = os.Stat(filename)
		if err == nil {
			v("found basedir: %v\n", dir)
			return dir
		}

		if err != nil && os.IsNotExist(err) {
			dir = filepath.Dir(dir)
			continue
		}

		ok(err)
	}
}

func ok(err error) {
	if err == nil {
		return
	}

	fmt.Fprintf(os.Stderr, "error: %+v\n", err)
	os.Exit(1)
}

func v(s string, args ...interface{}) {
	if !globalOpts.Verbose {
		return
	}
	fmt.Printf(s, args...)
}

func d(s string, args ...interface{}) {
	if !globalOpts.Debug {
		return
	}
	fmt.Printf(s, args...)
}

func msg(s string, args ...interface{}) {
	fmt.Printf(s, args...)
}

func warn(s string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, s, args...)
}

func main() {
	err := cmdRoot.Execute()
	var exitCode int
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		exitCode = 1
	}

	os.Exit(exitCode)
}
