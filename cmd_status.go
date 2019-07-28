package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var cmdStatus = &cobra.Command{
	Use:   "status",
	Short: "displays the status of all modules",
	Run: func(*cobra.Command, []string) {
		state := currentState(globalOpts.Base, globalOpts.Target)
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

// State contains the state whether a module is installed or not.
type State map[string]bool

func firstdir(dir string) string {
	dirs := strings.Split(dir, string(filepath.Separator))
	return dirs[0]
}

func installedModules(base, target string) State {
	state := make(State)
	walkOurSymlinks(base, target, func(filename, dir string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(base, dir)
		if err != nil {
			return err
		}

		module := firstdir(rel)

		state[module] = true
		return nil
	})

	return state
}

func allModules(base string) (modules []string) {
	for _, dir := range subdirs(base) {
		module := filepath.Base(dir)
		modules = append(modules, module)
	}
	return modules
}

func currentState(base, target string) State {
	state := installedModules(base, target)
	for _, module := range allModules(base) {
		if _, ok := state[module]; !ok {
			state[module] = false
		}
	}
	return state
}
