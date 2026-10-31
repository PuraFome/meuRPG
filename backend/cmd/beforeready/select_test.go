package main

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

const mod = "github.com/PuraFome/meuRPG/backend"

// rows mimic `go list -test`: play imports combat through rules and its tests import campaigns;
// cmd/api imports play; leaktest imports play only in its tests; gen is the generated code.
const rows = `
github.com/PuraFome/meuRPG/backend/internal/rules/combat	github.com/PuraFome/meuRPG/backend/internal/platform/dice fmt
github.com/PuraFome/meuRPG/backend/internal/rules/combat.test	github.com/PuraFome/meuRPG/backend/internal/rules/combat github.com/PuraFome/meuRPG/backend/internal/platform/dice testing
github.com/PuraFome/meuRPG/backend/internal/rules	github.com/PuraFome/meuRPG/backend/internal/rules/combat
github.com/PuraFome/meuRPG/backend/internal/play	github.com/PuraFome/meuRPG/backend/internal/rules github.com/PuraFome/meuRPG/backend/internal/rules/combat github.com/PuraFome/meuRPG/backend/gen/play/v1
github.com/PuraFome/meuRPG/backend/internal/play [github.com/PuraFome/meuRPG/backend/internal/play.test]	github.com/PuraFome/meuRPG/backend/internal/rules github.com/PuraFome/meuRPG/backend/internal/rules/combat
github.com/PuraFome/meuRPG/backend/internal/play_test [github.com/PuraFome/meuRPG/backend/internal/play.test]	github.com/PuraFome/meuRPG/backend/internal/campaigns
github.com/PuraFome/meuRPG/backend/internal/play.test	github.com/PuraFome/meuRPG/backend/internal/campaigns github.com/PuraFome/meuRPG/backend/internal/rules github.com/PuraFome/meuRPG/backend/internal/rules/combat github.com/PuraFome/meuRPG/backend/gen/play/v1
github.com/PuraFome/meuRPG/backend/internal/campaigns	github.com/PuraFome/meuRPG/backend/internal/rules
github.com/PuraFome/meuRPG/backend/cmd/api	github.com/PuraFome/meuRPG/backend/internal/play github.com/PuraFome/meuRPG/backend/internal/rules github.com/PuraFome/meuRPG/backend/internal/rules/combat
github.com/PuraFome/meuRPG/backend/internal/leaktest	github.com/PuraFome/meuRPG/backend/internal/play github.com/PuraFome/meuRPG/backend/internal/rules github.com/PuraFome/meuRPG/backend/internal/rules/combat
github.com/PuraFome/meuRPG/backend/internal/archtest	fmt
github.com/PuraFome/meuRPG/backend/migrations	embed
github.com/PuraFome/meuRPG/backend/gen/play/v1	google.golang.org/protobuf/proto
`

const list = `# comment
run	cmd/api	^TestRun$	regen api	-
run	internal/archtest	^Test	no generator	backend,proto/error-reasons.json
helper	internal/play	runGolden	why
run	internal/leaktest	^TestLeak$	regen leak	-
run	internal/play	^TestGolden$	regen play	-
run	internal/play	^TestStored$	regen stored	backend/migrations
run	internal/rules	^TestSnapshot$	regen snapshot	NOTICE,web/src/app/pages/credits/credits.ts
run	internal/rules/combat	Characterization$	regen combat	-
run	migrations	^TestKinds$	regen kinds	backend/internal
`

func setup(t *testing.T) ([]Line, Facts) {
	t.Helper()
	lines, err := ParseList(strings.NewReader(list))
	if err != nil {
		t.Fatal(err)
	}
	dirs, deps, err := parseGraph(mod, rows)
	if err != nil {
		t.Fatal(err)
	}
	return lines, Facts{PkgDirs: dirs, Deps: deps, NewLines: map[string]bool{}}
}

// run returns "pkg regex" of the selected lines.
func run(t *testing.T, changed ...string) []string {
	t.Helper()
	lines, f := setup(t)
	f.Changed = changed
	sel, skipped := Select(lines, f)
	if len(sel)+len(skipped) != len(lines) {
		t.Fatalf("selected %d + skipped %d != %d lines", len(sel), len(skipped), len(lines))
	}
	var out []string
	for _, d := range sel {
		out = append(out, d.Line.Pkg+" "+d.Line.Regex)
	}
	return out
}

func TestSelect(t *testing.T) {
	t.Parallel()
	play := []string{"internal/play ^TestGolden$", "internal/play ^TestStored$"}
	cases := []struct {
		name    string
		changed []string
		want    []string
	}{
		{"web only", []string{"web/src/app/app.ts", "web/package.json"}, nil},
		{"docs only", []string{"docs/testing.md", "README.md", "Makefile"}, nil},
		{"nothing changed", nil, nil},
		{
			"a file in the play package selects its lines and the importers",
			[]string{"backend/internal/play/combat.go"},
			// archtest and migrations: any file of backend/internal is an input
			append([]string{"cmd/api ^TestRun$", "internal/archtest ^Test", "internal/leaktest ^TestLeak$", "migrations ^TestKinds$"}, play...),
		},
		{
			"a data file in a subdirectory of play belongs to play",
			[]string{"backend/internal/play/cases/x.json"},
			append([]string{"cmd/api ^TestRun$", "internal/archtest ^Test", "internal/leaktest ^TestLeak$", "migrations ^TestKinds$"}, play...),
		},
		{
			"combat selects the play goldens through the import",
			[]string{"backend/internal/rules/combat/attack.go"},
			[]string{"cmd/api ^TestRun$", "internal/archtest ^Test", "internal/leaktest ^TestLeak$", "internal/play ^TestGolden$", "internal/play ^TestStored$", "internal/rules ^TestSnapshot$", "internal/rules/combat Characterization$", "migrations ^TestKinds$"},
		},
		{
			"the error reasons file selects the archtest line only",
			[]string{"proto/error-reasons.json"},
			[]string{"internal/archtest ^Test"},
		},
		{
			"a migration selects the lines that name it as input, and the ones that import its package",
			[]string{"backend/migrations/00300_x.sql"},
			[]string{"internal/archtest ^Test", "internal/play ^TestStored$", "migrations ^TestKinds$"},
		},
		{
			"a proto file selects the packages that import generated code",
			[]string{"proto/play/v1/play.proto"},
			play,
		},
		{
			"the credits page is an input of the snapshot test",
			[]string{"web/src/app/pages/credits/credits.ts"},
			[]string{"internal/rules ^TestSnapshot$"},
		},
		{
			"NOTICE is an input of the snapshot test",
			[]string{"NOTICE"},
			[]string{"internal/rules ^TestSnapshot$"},
		},
		{
			"a rename counts for both paths",
			[]string{"web/a.ts", "backend/internal/rules/combat/old.go"},
			[]string{"cmd/api ^TestRun$", "internal/archtest ^Test", "internal/leaktest ^TestLeak$", "internal/play ^TestGolden$", "internal/play ^TestStored$", "internal/rules ^TestSnapshot$", "internal/rules/combat Characterization$", "migrations ^TestKinds$"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := run(t, c.changed...)
			slices.Sort(got)
			want := slices.Clone(c.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("selected\n  %q\nwant\n  %q", got, want)
			}
		})
	}
}

func TestSelectTheToolOrTheDependenciesSelectEverything(t *testing.T) {
	t.Parallel()
	lines, f := setup(t)
	for _, file := range []string{".github/scripts/before-ready.sh", "backend/go.mod", "backend/go.sum", "backend/cmd/beforeready/select.go"} {
		f.Changed = []string{file}
		sel, skipped := Select(lines, f)
		if len(sel) != len(lines) || len(skipped) != 0 {
			t.Errorf("%s selected %d of %d lines", file, len(sel), len(lines))
		}
	}
}

func TestSelectAChangedLineOfTheList(t *testing.T) {
	t.Parallel()
	lines, f := setup(t)
	f.Changed = []string{listPath}
	f.NewLines = map[string]bool{lines[2].Raw: true} // the leaktest line
	sel, _ := Select(lines, f)
	var got []string
	for _, d := range sel {
		got = append(got, d.Line.Pkg)
	}
	// the new line, the archtest package (the list is in its directory) and the migrations line (it reads backend/internal)
	slices.Sort(got)
	if want := []string{"internal/archtest", "internal/leaktest", "migrations"}; !slices.Equal(got, want) {
		t.Errorf("selected %v, want %v", got, want)
	}
	f.NewLines = nil // the list of the merge base could not be read
	if sel, _ := Select(lines, f); len(sel) != len(lines) {
		t.Errorf("an unreadable old list selected %d of %d lines", len(sel), len(lines))
	}
}

func TestReasonsNameTheFile(t *testing.T) {
	t.Parallel()
	lines, f := setup(t)
	f.Changed = []string{"backend/internal/rules/combat/attack.go"}
	sel, _ := Select(lines, f)
	for _, d := range sel {
		if d.Line.Pkg == "internal/play" && !strings.Contains(d.Reason, "backend/internal/rules/combat/attack.go") {
			t.Errorf("reason %q does not name the changed file", d.Reason)
		}
	}
}

func TestAllNamesTheReason(t *testing.T) {
	t.Parallel()
	lines, _ := setup(t)
	got := All(lines, "because")
	if len(got) != len(lines) || got[0].Reason != "because" {
		t.Errorf("All = %v", got)
	}
}

func TestParseListRefusesAShortLine(t *testing.T) {
	t.Parallel()
	if _, err := ParseList(strings.NewReader("run\tinternal/x\t^T$\tregen\n")); err == nil {
		t.Error("a run line with no inputs column was accepted")
	}
}

func TestParseGraphKeepsTheTestImports(t *testing.T) {
	t.Parallel()
	dirs, deps, err := parseGraph(mod, rows)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(dirs, "backend/internal/play") || slices.Contains(dirs, "backend/internal/play_test") {
		t.Errorf("dirs = %v", dirs)
	}
	if !slices.Contains(deps["backend/internal/play"], "backend/internal/campaigns") {
		t.Errorf("play deps lack the package its tests import: %v", deps["backend/internal/play"])
	}
	if _, _, err := parseGraph(mod, "other/pkg\tfmt\n"); err == nil {
		t.Error("a graph with no package of the module was accepted")
	}
}

func TestFiltered(t *testing.T) {
	t.Parallel()
	lines, _ := setup(t)
	got := filtered(list, All(lines[:1], "x"))
	if !strings.Contains(got, "regen api") || strings.Contains(got, "regen leak") || !strings.Contains(got, "helper\t") || !strings.Contains(got, "# comment") {
		t.Errorf("filtered = %q", got)
	}
}

func TestPlanRunsEverythingWhenItCannotTell(t *testing.T) {
	t.Parallel()
	lines, _ := setup(t)
	run, skipped, note := plan(lines, false, Facts{}, errors.New("no merge base with origin/main"))
	if len(run) != len(lines) || len(skipped) != 0 || !strings.Contains(note, "running every line") {
		t.Errorf("run %d of %d, skipped %d, note %q", len(run), len(lines), len(skipped), note)
	}
	run, _, note = plan(lines, true, Facts{}, nil)
	if len(run) != len(lines) || note != "" {
		t.Errorf("ALL: run %d of %d, note %q", len(run), len(lines), note)
	}
}

func TestComputeFactsOutsideAGitTreeFails(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", "/")
	if _, err := computeFacts("none", ""); err == nil {
		t.Error("computeFacts succeeded outside a git work tree")
	}
}
