package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

type InstallOptions struct {
	global       GlobalOptions
	DisableCheck bool
}

var installOpts InstallOptions

func init() {
	cmdRoot.AddCommand(cmdInstall)
	cmdInstall.Flags().BoolVar(&installOpts.DisableCheck, "disable-check", false, "don't check for broken symlinks on install")
}

var cmdInstall = &cobra.Command{
	Use:   "install",
	Short: "install modules",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := installOpts
		opts.global = globalOpts
		return runInstall(cmd, opts, args)
	},
}

func readdir(dir string) []os.FileInfo {
	f, err := os.Open(dir)
	ok(err)
	fis, err := f.Readdir(-1)
	ok(err)
	ok(f.Close())
	return fis
}

func subdirs(dir string) (dirs []string) {
	for _, fi := range readdir(dir) {
		if !fi.IsDir() {
			continue
		}

		name := fi.Name()
		if name == ".git" || name == "manage" || name == "old" {
			continue
		}

		dirs = append(dirs, name)
	}

	return dirs
}

func isSymlink(fi os.FileInfo) bool {
	return (fi.Mode() & os.ModeSymlink) != 0
}

func isDir(name string) bool {
	fi, err := os.Stat(name)
	ok(err)
	return fi.Mode().IsDir()
}

// exists checks if the target name points to exists.
func exists(name string) bool {
	_, err := os.Stat(name)
	return !os.IsNotExist(err)
}

// linkDirContents links all entries in srcdir to dstdir.
func linkDirContents(opts InstallOptions, srcdir, dstdir string) {
	for _, entry := range readdir(srcdir) {
		link(opts, filepath.Join(srcdir, entry.Name()), dstdir)
	}
}

// resolveConflict resolves a conflict where two sources (src1 exists, src2 not
// yet) are to be linked to the name dst.
func resolveConflict(opts InstallOptions, src1, src2, dst string) {
	v("resolve conflict for %v:\n  %v\n  %v\n", dst, src1, src2)
	if !isDir(src1) {
		warn("unable to resolve conflict for %v: source %v is not a directory\n", dst, src1)
		return
	}
	if !isDir(src2) {
		warn("unable to resolve conflict for %v: source %v is not a directory\n", dst, src2)
	}

	// first, remove the symlink
	ok(os.Remove(dst))

	// create a new directory
	ok(os.Mkdir(dst, 0755))

	// then link the file separately
	linkDirContents(opts, src1, dst)
	linkDirContents(opts, src2, dst)

	v("conflict for %v resolved\n", dst)
}

// link creates a symlink to the file/dir src in targetdir.
func link(opts InstallOptions, src, targetdir string) {
	base := filepath.Base(src)
	if len(base) == 0 {
		warn("invalid item name %v, skipping\n", src)
		return
	}

	dst := filepath.Join(targetdir, base)

	fi, err := os.Lstat(dst)
	if err == nil {
		if !isSymlink(fi) && !fi.IsDir() {
			warn("skipping already existing item %v (neither symlink nor directory)\n", dst)
			return
		}

		if fi.IsDir() {
			v("%v exists, descending into %v\n", dst, src)
			linkDirContents(opts, src, dst)
			return
		}

		linkTarget := readlink(dst)
		switch {
		case linkTarget == src:
			// nothing to do, dst already points to src
			return

		case isSubdir(opts.global.Base, readlink(dst)):
			resolveConflict(opts, linkTarget, src, dst)
			return

		case exists(dst) && isDir(dst):
			v("descending into %v\n", linkTarget)
			linkDirContents(opts, src, dst)
			return

		case exists(dst):
			v("symlink %v already exists, skipping\n", dst)
			return

		default:
			v("removing dangling symlink\n")
			if !opts.global.DryRun {
				ok(os.Remove(dst))
			}
		}
	}

	v("link %v -> %v\n", src, dst)
	if !opts.global.DryRun {
		ok(os.Symlink(src, dst))
	}
}

func install(opts InstallOptions, moduleName, targetdir string) {
	if moduleName == "" {
		return
	}

	modulePath := filepath.Join(opts.global.Base, moduleName)

	v("install %v\n", moduleName)
	if !exists(modulePath) {
		warn("module %v does not exist, skipping\n", moduleName)
		return
	}

	linkDirContents(opts, modulePath, targetdir)
}

func cleanupBrokenLinks(opts InstallOptions) {
	v("looking for broken symlinks (disable with `--disable-check`)...")
	defer v(" done\n")
	walkOurSymlinks(opts.global.Base, opts.global.Target, func(filename, target string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		d("check symlink %v\n", target)

		// if the target exists, do nothing
		if exists(target) {
			return nil
		}

		v("removing broken symlink %v\n", filename)
		return os.Remove(filename)
	})
}

func runInstall(cmd *cobra.Command, opts InstallOptions, args []string) error {
	v("run install\n")
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	if !opts.DisableCheck {
		cleanupBrokenLinks(opts)
	}

	if len(args) == 0 {
		// install only the base module
		install(opts, "base", opts.global.Target)

		if exists(filepath.Join(opts.global.Base, "base_"+hostname)) {
			install(opts, "base_"+hostname, opts.global.Target)
		}
		return nil
	}

	if args[0] == "all" {
		// install all modules
		for _, module := range subdirs(opts.global.Base) {
			install(opts, module, opts.global.Target)
			if exists(filepath.Join(opts.global.Base, module+"_"+hostname)) {
				install(opts, module+"_"+hostname, opts.global.Target)
			}
		}
		return nil
	}

	// otherwise install the listed modules
	for _, module := range args {
		// install all modules
		install(opts, module, opts.global.Target)
		if exists(filepath.Join(opts.global.Base, module+"_"+hostname)) {
			install(opts, module+"_"+hostname, opts.global.Target)
		}
	}

	return nil
}

func readlink(file string) string {
	target, err := os.Readlink(file)
	ok(err)
	return target
}

func isSubdir(dir, subdir string) bool {
	for {
		if len(subdir) <= len(dir) {
			return dir == subdir
		}

		subdir = filepath.Dir(subdir)
		if subdir == "." {
			return false
		}
	}
}
