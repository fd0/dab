package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var cmdImport = &cobra.Command{
	Use:   "import FILE|DIR MODULE",
	Short: "import existing files and directories into a given module",
	Run:   runImport,
}

func init() {
	cmdRoot.AddCommand(cmdImport)
}

func pathIsSymlink(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return isSymlink(fi)
}

func runImport(cmd *cobra.Command, args []string) {
	v("runImport\n")
	if len(args) != 2 {
		warn("usage: bundle add FILE|DIR MODULE\n")
		os.Exit(1)
	}

	src, module := args[0], args[1]

	if !exists(src) {
		warn("%s: does not exist\n", src)
		os.Exit(2)
	}

	if pathIsSymlink(src) {
		warn("%s: already a symlink, not importing\n")
		os.Exit(2)
	}

	if !isSubdir(opts.Target, filepath.Dir(src)) {
		warn("%s is not under %s\n", src, opts.Target)
		os.Exit(2)
	}

	moduleDir := filepath.Join(opts.Base, module)
	rel, err := filepath.Rel(opts.Target, src)
	ok(err)
	dst := filepath.Join(moduleDir, rel)

	if exists(dst) {
		warn("%s already exists\n", dst)
		os.Exit(2)
	}

	if !exists(moduleDir) {
		v("creating directories for new module %q\n", module)
		ok(os.MkdirAll(moduleDir, 0700))
	}

	ok(os.MkdirAll(filepath.Dir(dst), 0700))
	v("moving %q to %q\n", src, dst)
	ok(os.Rename(src, dst))
	v("creating symlink to %q in %q\n", dst, filepath.Dir(src))
	ok(os.Symlink(dst, filepath.Dir(src)))
}
