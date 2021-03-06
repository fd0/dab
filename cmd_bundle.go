package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var cmdBundle = &cobra.Command{
	Use:   "bundle",
	Short: "manage bundles",
}

var cmdBundleAdd = &cobra.Command{
	Use:   "add DIR SOURCE REF",
	Short: "add a new bundle",
	Run: func(cmd *cobra.Command, args []string) {
		runBundleAdd(cmd, globalOpts, args)
	},
}

func init() {
	cmdRoot.AddCommand(cmdBundle)
	cmdBundle.AddCommand(cmdBundleAdd)
	cmdBundle.AddCommand(cmdBundleUpdate)
	cmdBundle.AddCommand(cmdBundleRemove)
}

// BundleConfig is a list of bundles to manage.
type BundleConfig struct {
	Bundles []Bundle
}

// Bundle configures a bundle.
type Bundle struct {
	Source string
	Ref    string
	Commit string
	Dir    string
}

func loadBundleConfig(base string) BundleConfig {
	buf, err := ioutil.ReadFile(filepath.Join(base, "bundles.json"))
	if os.IsNotExist(err) {
		return BundleConfig{}
	}
	ok(err)

	var cfg BundleConfig
	ok(json.Unmarshal(buf, &cfg))
	return cfg
}

func saveBundleConfig(base string, cfg BundleConfig) {
	buf, err := json.MarshalIndent(cfg, "", "  ")
	ok(err)
	ok(ioutil.WriteFile(filepath.Join(base, "bundles.json"), buf, 0644))
}

// run executes cmd in dir.
func run(dir, cmd string, args ...string) {
	v("run %q %q\n", cmd, args)
	c := exec.Command(cmd, args...)
	c.Dir = dir
	c.Stderr = os.Stderr
	c.Stdout = os.Stdout
	ok(c.Run())
}

// run executes cmd in dir and returns what's printed on stdout.
func runOutput(dir, cmd string, args ...string) string {
	v("run %q %q\n", cmd, args)
	c := exec.Command(cmd, args...)
	c.Dir = dir
	c.Stderr = os.Stderr
	buf, err := c.Output()
	ok(err)
	return string(buf)
}

var cmdBundleUpdate = &cobra.Command{
	Use:   "update [bundle] [...]",
	Short: "update bundles",
	Run: func(cmd *cobra.Command, args []string) {
		runBundleUpdate(cmd, globalOpts, args)
	},
}

func addBundle(base string, b *Bundle) {
	run(base, "git", "-c", "fetch.fsckObjects=false", "clone", b.Source, b.Dir)
	bundleDir := filepath.Join(base, b.Dir)
	run(bundleDir, "git", "checkout", b.Ref)
	b.Commit = strings.TrimSpace(runOutput(bundleDir, "git", "rev-parse", "HEAD"))
	v("bundle %v is at %v\n", b.Dir, b.Commit)
	ok(os.RemoveAll(filepath.Join(bundleDir, ".git")))
}

func updateBundle(base string, b *Bundle) {
	v("update bundle %v\n", b.Dir)
	bundleDir := filepath.Join(base, b.Dir)
	ok(os.RemoveAll(bundleDir))
	addBundle(base, b)
}

func runBundleAdd(cmd *cobra.Command, opts GlobalOptions, args []string) {
	if len(args) < 2 || len(args) > 3 {
		warn("usage: bundle add DIR SRC REF\n")
		os.Exit(1)
	}

	// use the master branch by default
	if len(args) == 2 {
		args = append(args, "master")
	}

	dir, src, ref := args[0], args[1], args[2]

	cfg := loadBundleConfig(opts.Base)
	bundle := Bundle{Dir: dir, Source: src, Ref: ref}

	addBundle(opts.Base, &bundle)

	cfg.Bundles = append(cfg.Bundles, bundle)

	saveBundleConfig(opts.Base, cfg)
}

func runBundleUpdate(cmd *cobra.Command, opts GlobalOptions, args []string) {
	v("runUpdateBundle\n")
	updateModules := make(map[string]bool)
	if len(args) > 0 {
		for _, dir := range args {
			updateModules[dir] = true
		}
	} else {
		updateModules[""] = true
	}

	cfg := loadBundleConfig(opts.Base)
	for i, bundle := range cfg.Bundles {
		allowed, ok := updateModules[bundle.Dir]
		if !ok {
			allowed = updateModules[""]
		}

		if !allowed {
			continue
		}

		v("testing if %v exists\n", bundle.Dir)
		if !exists(filepath.Join(opts.Base, bundle.Dir)) {
			addBundle(opts.Base, &bundle)
			continue
		}

		updateBundle(opts.Base, &bundle)
		cfg.Bundles[i] = bundle
	}

	saveBundleConfig(opts.Base, cfg)
}

var cmdBundleRemove = &cobra.Command{
	Use:   "remove bundle [bundle] [...]",
	Short: "remove bundles",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBundleRemove(cmd, globalOpts, args)
	},
}

func runBundleRemove(cmd *cobra.Command, opts GlobalOptions, args []string) error {
	if len(args) == 0 {
		return errors.Errorf("specify at least one bundle directory\n")
	}

	cfg := loadBundleConfig(opts.Base)

	for _, dir := range args {
		err := os.RemoveAll(filepath.Join(opts.Base, dir))
		if err != nil {
			return errors.Wrap(err, "RemoveAll")
		}

		for i, b := range cfg.Bundles {
			if b.Dir == dir {
				cfg.Bundles = append(cfg.Bundles[:i], cfg.Bundles[i+1:]...)
				break
			}
		}
	}

	saveBundleConfig(opts.Base, cfg)

	return nil
}
