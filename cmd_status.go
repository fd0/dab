package main

import (
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

var cmdStatus = &cobra.Command{
	Use:   "status",
	Short: "displays the status of all bundles",
	Run: func(*cobra.Command, []string) {
		state := currentState()
		keys := make([]string, 0, len(state))
		for name := range state {
			keys = append(keys, name)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i] < keys[j]
		})

		for _, name := range keys {
			installed := state[name]
			if !installed {
				msg("[ ---- ] %v\n", name)
			} else {
				msg("[ INST ] %v\n", name)
			}
		}
	},
}

func init() {
	cmdRoot.AddCommand(cmdStatus)
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
		if !isSubdir(opts.Base, target) {
			continue
		}

		module := filepath.Base(filepath.Dir(target))
		v("module %v, target %v\n", module, target)
		state[module] = true
	}

	return state
}

func allModules() (modules []string) {
	for _, dir := range subdirs(opts.Base) {
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
