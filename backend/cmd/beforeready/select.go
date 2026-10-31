package main

import (
	"path"
	"sort"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/archtest/beforereadylist"
)

const (
	// listPath is the default list, seen from the repository root.
	listPath = "backend/internal/archtest/before_ready.txt"
	// backendDir is the Go module's directory, seen from the repository root.
	backendDir = "backend"
	// genDir holds the Go code generated from proto/.
	genDir = backendDir + "/gen"
	// errorReasons is the one file under proto/ that the tests read as data, not as a schema.
	errorReasons = "proto/error-reasons.json"
)

// everythingFiles are the files whose change makes every line run: the tool that selects and
// runs the tests (the Makefile target passes it ONLY and ALL), and the module's dependencies.
var everythingFiles = []string{
	".github/scripts/before-ready.sh",
	"Makefile",
	backendDir + "/go.mod",
	backendDir + "/go.sum",
}

// everythingDirs are the directories whose change makes every line run: the tool, and the
// parser of the list with the function that decides which paths an input covers.
var everythingDirs = []string{backendDir + "/cmd/beforeready", backendDir + "/internal/archtest/beforereadylist"}

// Line is one `run` line of the list, as the shared parser reads it.
type Line = beforereadylist.Line

// Facts is what the selection knows about the change and the module.
type Facts struct {
	// Changed are the files the change touches, slash paths from the repository root
	// (modified, deleted, renamed from and to, and untracked).
	Changed []string
	// PkgDirs are the directories of all the module's packages, from the repository root.
	PkgDirs []string
	// Deps maps a package directory to the module packages its tests need, itself included
	// (go list -test, the .Deps field), as directories from the repository root.
	Deps map[string][]string
	// NewLines are the lines of the list that are not in the list of the merge base. Nil
	// when that list could not be read.
	NewLines map[string]bool
}

// Decision is a line that runs and the first reason it does.
type Decision struct {
	Line   Line
	Reason string
}

// Select returns the lines to run, each with why, and the lines left out.
func Select(lines []Line, f Facts) (run []Decision, skipped []Line) {
	owner := ownerOf(f)
	for _, l := range lines {
		if why := whyRun(l, f, owner); why != "" {
			run = append(run, Decision{Line: l, Reason: why})
		} else {
			skipped = append(skipped, l)
		}
	}
	return run, skipped
}

// All returns every line as a decision with the reason.
func All(lines []Line, reason string) []Decision {
	out := make([]Decision, 0, len(lines))
	for _, l := range lines {
		out = append(out, Decision{Line: l, Reason: reason})
	}
	return out
}

// ownerOf returns a function that maps a file to the directory of the package that owns it:
// the nearest package directory above it (a data file, a migration and a testdata file belong
// to the package that embeds or reads them), or "" when no package does.
func ownerOf(f Facts) func(string) string {
	pkgs := make(map[string]bool, len(f.PkgDirs))
	for _, d := range f.PkgDirs {
		pkgs[d] = true
	}
	return func(file string) string {
		for d := path.Dir(file); d != "." && d != "/"; d = path.Dir(d) {
			if pkgs[d] {
				return d
			}
		}
		return ""
	}
}

func whyRun(l Line, f Facts, owner func(string) string) string {
	self := backendDir + "/" + l.Pkg
	deps := f.Deps[self]
	depSet := make(map[string]bool, len(deps)+1)
	depSet[self] = true
	for _, d := range deps {
		depSet[d] = true
	}
	for _, file := range f.Changed {
		if why := whyFile(l, f, file, self, depSet, owner(file)); why != "" {
			return why
		}
	}
	return ""
}

// whyFile says why one changed file selects the line, or "".
func whyFile(l Line, f Facts, file, self string, deps map[string]bool, pkg string) string {
	if why := whyEverything(file); why != "" {
		return why
	}
	if why := whyList(l, f, file); why != "" {
		return why
	}
	switch {
	case pkg == self:
		return file + " is in the package"
	case pkg != "" && deps[pkg]:
		return file + " is in " + strings.TrimPrefix(pkg, backendDir+"/") + ", which the package imports"
	case strings.HasPrefix(file, self+"/"):
		return file + " is under the package directory"
	case isProtoSchema(file) && importsGenerated(deps):
		return file + " is a proto file, and the package imports the code generated from proto/"
	}
	if in := beforereadylist.Covering(l.Inputs, file); in != "" {
		return file + " is an input of the line (" + in + ")"
	}
	return ""
}

// whyList says why a change to the list itself selects the line: its line is new or changed.
func whyList(l Line, f Facts, file string) string {
	if file != listPath {
		return ""
	}
	if f.NewLines == nil {
		return file + " changed and the list of the merge base could not be read"
	}
	if f.NewLines[l.Raw] {
		return "its line in " + file + " is new or changed"
	}
	return ""
}

func isProtoSchema(file string) bool {
	return strings.HasPrefix(file, "proto/") && file != errorReasons
}

func importsGenerated(deps map[string]bool) bool {
	for d := range deps {
		if d == genDir || strings.HasPrefix(d, genDir+"/") {
			return true
		}
	}
	return false
}

func whyEverything(file string) string {
	for _, p := range everythingFiles {
		if file == p {
			return file + " changed: it is the tool, or the dependencies, of every line"
		}
	}
	for _, d := range everythingDirs {
		if strings.HasPrefix(file, d+"/") {
			return file + " changed: it is the tool of every line"
		}
	}
	return ""
}

// Packages returns the distinct package directories of the decisions, sorted.
func Packages(ds []Decision) []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range ds {
		if !seen[d.Line.Pkg] {
			seen[d.Line.Pkg] = true
			out = append(out, d.Line.Pkg)
		}
	}
	sort.Strings(out)
	return out
}
