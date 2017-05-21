package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

type BundleConfig struct {
	Bundles []Bundle
}

type Bundle struct {
	Source string
	Ref    string
	Dir    string
}

func loadBundleConfig() BundleConfig {
	buf, err := ioutil.ReadFile(filepath.Join(opts.basedir, "bundles.json"))
	if os.IsNotExist(err) {
		return BundleConfig{}
	}
	ok(err)

	var cfg BundleConfig
	ok(json.Unmarshal(buf, &cfg))
	return cfg
}

func saveBundleConfig(cfg BundleConfig) {
	buf, err := json.MarshalIndent(cfg, "", "  ")
	ok(err)
	ok(ioutil.WriteFile(filepath.Join(opts.basedir, "bundles.json"), buf, 0644))
}

func cmdBundle(args []string) {
	if len(args) == 0 {
		warn("usage: bundle [add|update]\n")
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		cmdBundleAdd(args[1:])
	case "update":
		cmdBundleUpdate(args[1:])
	}
}

// run executes cmd in opts.basedir
func run(cmd string, args ...string) {
	v("run %q %q\n", cmd, args)
	c := exec.Command(cmd, args...)
	c.Dir = opts.basedir
	c.Stderr = os.Stderr
	c.Stdout = os.Stdout
	ok(c.Run())
}

func addBundle(b Bundle) {
	run("git", "-c", "fetch.fsckObjects=false",
		"subtree", "add", "--squash", "--prefix",
		b.Dir, b.Source, b.Ref)
}

func updateBundle(b Bundle) {
	run("git", "-c", "fetch.fsckObjects=false",
		"subtree", "pull", "-q", "--squash",
		"--prefix", b.Dir, b.Source, b.Ref)
}

func cmdBundleAdd(args []string) {
	if len(args) < 2 || len(args) > 3 {
		warn("usage: bundle add DIR SRC REF\n")
		os.Exit(1)
	}

	// use the master branch by default
	if len(args) == 2 {
		args = append(args, "master")
	}

	dir, src, ref := args[0], args[1], args[2]

	cfg := loadBundleConfig()
	bundle := Bundle{Dir: dir, Source: src, Ref: ref}

	addBundle(bundle)

	cfg.Bundles = append(cfg.Bundles, bundle)

	saveBundleConfig(cfg)

	msg := fmt.Sprintf("Add bundle as %v\n\nSourced from %v (%v)\n", dir, src, ref)
	run("git", "add", "bundles.json")
	run("git", "commit", "--message", msg, "bundles.json")
}

func cmdBundleUpdate(args []string) {
	updateModules := make(map[string]bool)
	if len(args) > 0 {
		for _, dir := range args {
			updateModules[dir] = true
		}
	} else {
		updateModules[""] = true
	}

	cfg := loadBundleConfig()
	for _, bundle := range cfg.Bundles {
		allowed, ok := updateModules[bundle.Dir]
		if !ok {
			allowed = updateModules[""]
		}

		if !allowed {
			continue
		}

		if !exists(bundle.Dir) {
			addBundle(bundle)
			continue
		}

		updateBundle(bundle)
	}
}
