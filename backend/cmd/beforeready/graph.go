package main

import (
	"errors"
	"sort"
	"strings"
)

// parseGraph reads `go list -test` rows (`import path<TAB>deps separated by spaces`) of the
// module mod into the package directories from the repository root and, for each, the module
// packages its tests need. The rows of a package's test variants (`p [p.test]`, `p_test [p.test]`,
// `p.test`) count for the package itself.
func parseGraph(mod, text string) (dirs []string, deps map[string][]string, err error) {
	sets := map[string]map[string]bool{}
	isPkg := map[string]bool{}
	for _, row := range strings.Split(text, "\n") {
		if strings.TrimSpace(row) == "" {
			continue
		}
		path, rest, _ := strings.Cut(row, "\t")
		dir, ok := moduleDir(mod, path)
		if !ok {
			continue
		}
		if !strings.Contains(path, " [") && !strings.HasSuffix(path, ".test") && !strings.HasSuffix(path, "_test") {
			isPkg[dir] = true
		}
		if sets[dir] == nil {
			sets[dir] = map[string]bool{dir: true}
		}
		for _, d := range strings.Fields(rest) {
			if dd, ok := moduleDir(mod, d); ok {
				sets[dir][dd] = true
			}
		}
	}
	if len(isPkg) == 0 {
		return nil, nil, errors.New("go list found no package of the module " + mod)
	}
	deps = map[string][]string{}
	for dir := range isPkg {
		dirs = append(dirs, backendDir+"/"+dir)
		var ds []string
		for d := range sets[dir] {
			ds = append(ds, backendDir+"/"+d)
		}
		sort.Strings(ds)
		deps[backendDir+"/"+dir] = ds
	}
	sort.Strings(dirs)
	return dirs, deps, nil
}

// moduleDir turns an import path of the module (or one of its test variants) into its
// directory under the module root.
func moduleDir(mod, importPath string) (string, bool) {
	p, _, _ := strings.Cut(importPath, " [")
	p = strings.TrimSuffix(strings.TrimSuffix(p, ".test"), "_test")
	if p == mod || !strings.HasPrefix(p, mod+"/") {
		return "", false
	}
	return strings.TrimPrefix(p, mod+"/"), true
}
