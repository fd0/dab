package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

var opts = struct {
	DryRun  bool
	Verbose bool
	Target  string
	basedir string
}{}

func init() {
	defaultTarget := os.Getenv("TARGET")
	if defaultTarget == "" {
		defaultTarget = os.Getenv("HOME")
	}

	flag.BoolVar(&opts.DryRun, "dry-run", false, "only print actions, do not execute them")
	flag.BoolVar(&opts.Verbose, "verbose", false, "be verbose")
	flag.StringVar(&opts.Target, "target", defaultTarget, "set target directory")
	flag.Parse()
}

func ok(err error) {
	if err == nil {
		return
	}

	fmt.Fprintf(os.Stderr, "error: %v\n\nstack trace:\n", err)

	buf := make([]byte, 1<<20)
	l := runtime.Stack(buf, false)
	buf = buf[:l]
	fmt.Fprintf(os.Stderr, "%s", buf)

	os.Exit(1)
}

func v(s string, args ...interface{}) {
	if !opts.Verbose {
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
func linkDirContents(srcdir, dstdir string) {
	for _, entry := range readdir(srcdir) {
		link(filepath.Join(srcdir, entry.Name()), dstdir)
	}
}

// resolveConflict resolves a conflict where two sources (src1 exists, src2 not
// yet) are to be linked to the name dst.
func resolveConflict(src1, src2, dst string) {
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
	linkDirContents(src1, dst)
	linkDirContents(src2, dst)

	v("conflict for %v resolved\n", dst)
}

// link creates a symlink to the file/dir src in targetdir.
func link(src, targetdir string) {
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
			linkDirContents(src, dst)
			return
		}

		linkTarget := readlink(dst)
		switch {
		case linkTarget == src:
			// nothing to do, dst already points to src
			return

		case isSubdir(opts.basedir, readlink(dst)):
			resolveConflict(linkTarget, src, dst)
			return

		case exists(dst) && isDir(dst):
			v("descending into %v\n", linkTarget)
			linkDirContents(src, dst)
			return

		case exists(dst):
			v("symlink %v already exists, skipping\n", dst)
			return

		default:
			v("removing dangling symlink\n")
			if !opts.DryRun {
				ok(os.Remove(dst))
			}
		}
	}

	v("link %v -> %v\n", src, dst)
	if !opts.DryRun {
		ok(os.Symlink(src, dst))
	}
}

func install(moduleName, targetdir string) {
	if moduleName == "" {
		return
	}

	modulePath := filepath.Join(opts.basedir, moduleName)

	v("install %v\n", moduleName)
	if !exists(modulePath) {
		warn("module %v does not exist, skipping\n", moduleName)
		return
	}

	linkDirContents(modulePath, targetdir)
}

func cleanupBrokenLinks() {
	for _, fi := range readdir(opts.Target) {
		if !isSymlink(fi) {
			continue
		}

		filename := filepath.Join(opts.Target, fi.Name())
		target := readlink(filename)

		// make sure we only act on symlinks that we manage
		if !isSubdir(opts.basedir, target) {
			continue
		}

		// if the target exists, do nothing
		if exists(target) {
			continue
		}

		v("removing broken symlink %v\n", filename)
		ok(os.Remove(filename))
	}
}

func cmdInstall(args []string) {
	hostname, err := os.Hostname()
	ok(err)

	cleanupBrokenLinks()

	if len(args) == 0 {
		// install only the base module
		install("base", opts.Target)

		if exists(filepath.Join(opts.basedir, "base_"+hostname)) {
			install("base_"+hostname, opts.Target)
		}
		return
	}

	if args[0] == "all" {
		// install all modules
		for _, module := range subdirs(opts.basedir) {
			install(module, opts.Target)
			install(module+"_"+hostname, opts.Target)
			if exists(filepath.Join(opts.basedir, module+"_"+hostname)) {
				install(module+"_"+hostname, opts.Target)
			}
		}
		return
	}

	// otherwise install the listed modules
	for _, module := range args {
		// install all modules
		install(module, opts.Target)
	}
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
	}
}

// State contains the state whether a bundle is installed or not.
type State map[string]bool

func installedModules() State {
	state := make(State)
	for _, fi := range readdir(opts.Target) {
		if !isSymlink(fi) {
			continue
		}

		target := readlink(filepath.Join(opts.Target, fi.Name()))
		if !isSubdir(opts.basedir, target) {
			continue
		}

		module := filepath.Base(filepath.Dir(target))
		v("module %v, target %v\n", module, target)
		state[module] = true
	}

	return state
}

func allModules() (modules []string) {
	for _, dir := range subdirs(opts.basedir) {
		module := filepath.Base(dir)
		modules = append(modules, module)
	}
	return modules
}

func currentState() State {
	state := installedModules()
	for _, module := range allModules() {
		if _, ok := state[module]; !ok {
			state[module] = false
		}
	}
	return state
}

func cmdStatus(args []string) {
	state := currentState()
	for m, installed := range state {
		if !installed {
			msg("[ ---- ] %v\n", m)
		} else {
			msg("[ INST ] %v\n", m)
		}
	}
}

func walkOurSymlinks(dir string, fn filepath.WalkFunc) {
	ok(filepath.Walk(opts.Target, func(filename string, fi os.FileInfo, err error) error {
		if err != nil {
			v("error checking %v: %v\n", filename, err)
			return nil
		}

		if !isSymlink(fi) {
			return err
		}

		target := readlink(filename)
		if !isSubdir(opts.basedir, target) {
			return err
		}

		return fn(filename, fi, err)
	}))
}

func cmdRemove(args []string) {
	walkOurSymlinks(opts.Target, func(filename string, fi os.FileInfo, err error) error {
		ok(os.Remove(filename))
		v("removed %v\n", filename)
		return err
	})
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

	for {
		if filepath.Dir(dir) == dir {
			warn("unable to find bundles.json\n")
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

func main() {
	var cmd string
	var args []string

	if len(flag.Args()) == 0 {
		cmd = "install"
	} else {
		cmd = flag.Args()[0]
		args = flag.Args()[1:]
	}

	opts.basedir = findBasedir()

	switch cmd {
	case "install":
		cmdInstall(args)
	case "remove":
		cmdRemove(args)
	case "status":
		cmdStatus(args)
	case "bundle":
		cmdBundle(args)
	default:
		warn("unknown command: %v\n", cmd)
		os.Exit(1)
	}
}
