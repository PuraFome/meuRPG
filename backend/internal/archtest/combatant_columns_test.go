package archtest

import (
	_ "embed"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"
)

// combatantColumnsFrozen lists the columns combatants has today, by their field name in the sqlc model.
// It can only shrink.
//
//go:embed combatant_columns.txt
var combatantColumnsFrozen string

// combatantColumnsAllowed lists the columns added after the freeze, as "FieldName # reason": a column that is not about
// one feature (what is about one feature goes in feature_state).
//
//go:embed combatant_columns_allowed.txt
var combatantColumnsAllowed string

// combatantModel is the sqlc model of the table, seen from this package's directory.
const combatantModel = backendRoot + "/internal/play/playdb/models.go"

// minColumnReason is the fewest characters a reason of an allowed column has.
const minColumnReason = 20

// TestCombatantsGainNoColumnPerFeature fails when the Combatant model has a column that neither combatant_columns.txt (the
// columns of the freeze) nor combatant_columns_allowed.txt (a new column with its reason) lists.
// A feature that must remember a use keeps it in combatants.feature_state (docs/architecture/play/feature-state.md).
func TestCombatantsGainNoColumnPerFeature(t *testing.T) {
	src, err := os.ReadFile(combatantModel)
	if err != nil {
		t.Fatal(err)
	}
	columns, err := combatantFields(string(src))
	if err != nil {
		t.Fatalf("%s: %v", combatantModel, err)
	}
	failProblems(t, checkCombatantColumns(columns, combatantColumnsFrozen, combatantColumnsAllowed))
	t.Logf("%d columns of combatants; %d frozen, %d allowed after the freeze", len(columns),
		len(parseExceptions(combatantColumnsFrozen)), len(parseExceptions(combatantColumnsAllowed)))
}

// combatantFields returns the field names of the Combatant struct in the source of the sqlc models, in file order.
func combatantFields(src string) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "models.go", src, 0)
	if err != nil {
		return nil, err
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "Combatant" {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return nil, fmt.Errorf("Combatant is not a struct")
			}
			var out []string
			for _, f := range st.Fields.List {
				for _, n := range f.Names {
					out = append(out, n.Name)
				}
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("no type Combatant")
}

// allowedColumns reads combatant_columns_allowed.txt: "FieldName # reason" per line, comments and blank lines skipped.
// The reason comes back empty for a line with none.
func allowedColumns(text string) (names []string, reasons map[string]string) {
	reasons = map[string]string{}
	for line := range strings.Lines(text) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, reason, _ := strings.Cut(line, "#")
		name = strings.TrimSpace(name)
		names = append(names, name)
		reasons[name] = strings.TrimSpace(reason)
	}
	return names, reasons
}

// checkCombatantColumns returns the problems of the columns against the two lists, sorted.
func checkCombatantColumns(columns []string, frozenText, allowedText string) []string {
	frozen := parseExceptions(frozenText)
	allowed, reasons := allowedColumns(allowedText)
	var problems []string
	if !slices.IsSorted(frozen) {
		problems = append(problems, "combatant_columns.txt is not sorted: keep its entries in sorted order")
	}
	if !slices.IsSorted(allowed) {
		problems = append(problems, "combatant_columns_allowed.txt is not sorted: keep its entries in sorted order")
	}
	for i := 1; i < len(frozen); i++ {
		if frozen[i] == frozen[i-1] {
			problems = append(problems, fmt.Sprintf("combatant_columns.txt lists %q twice: remove one", frozen[i]))
		}
	}
	for _, c := range columns {
		if !slices.Contains(frozen, c) && !slices.Contains(allowed, c) {
			problems = append(problems, fmt.Sprintf("combatants has a new column %s: use feature_state (docs/architecture/play/feature-state.md); "+
				"if the column is not about one feature, add it to combatant_columns_allowed.txt as `%s # reason` (at least %d characters)", c, c, minColumnReason))
		}
	}
	for _, f := range frozen {
		if !slices.Contains(columns, f) {
			problems = append(problems, fmt.Sprintf("combatant_columns.txt lists %q, which is not a column any more: remove it from combatant_columns.txt", f))
		}
	}
	for _, a := range allowed {
		switch {
		case !slices.Contains(columns, a):
			problems = append(problems, fmt.Sprintf("combatant_columns_allowed.txt lists %q, which is not a column any more: remove it from combatant_columns_allowed.txt", a))
		case slices.Contains(frozen, a):
			problems = append(problems, fmt.Sprintf("%q is in combatant_columns.txt and in combatant_columns_allowed.txt: keep it in one", a))
		case len([]rune(reasons[a])) < minColumnReason:
			problems = append(problems, fmt.Sprintf("combatant_columns_allowed.txt lists %q with no reason or a short one: say in at least %d characters why it is not about one feature", a, minColumnReason))
		}
	}
	slices.Sort(problems)
	return problems
}

const combatantModelFixture = `package playdb

type Combatant struct {
	ID    string
	Label string
	// a comment
	A, B  int32
}

type Other struct{ Nope string }
`

func TestCombatantFieldsReadsTheStruct(t *testing.T) {
	got, err := combatantFields(combatantModelFixture)
	if err != nil || !slices.Equal(got, []string{"ID", "Label", "A", "B"}) {
		t.Errorf("combatantFields = %v, %v; want ID, Label, A, B", got, err)
	}
	if _, err := combatantFields("package playdb\n"); err == nil {
		t.Error("a source with no Combatant has no error")
	}
	if _, err := combatantFields("not go"); err == nil {
		t.Error("a source that is not Go has no error")
	}
}

func TestCheckCombatantColumns(t *testing.T) {
	columns := []string{"ID", "Label", "Newer"}
	const frozen = "# frozen\nID\nLabel\n"
	long := "Newer # a column of the combat itself, no feature\n"
	cases := []struct {
		name            string
		columns         []string
		frozen, allowed string
		want            string // a part of the one problem, "" for none
	}{
		{"nothing new", []string{"ID", "Label"}, frozen, "# nothing\n", ""},
		{"a new column fails", columns, frozen, "", "new column Newer: use feature_state"},
		{"the message names the allowed list", columns, frozen, "", "combatant_columns_allowed.txt"},
		{"an allowed column with a reason passes", columns, frozen, long, ""},
		{"an allowed column without a reason fails", columns, frozen, "Newer\n", "no reason or a short one"},
		{"an allowed column with a short reason fails", columns, frozen, "Newer # too short\n", "no reason or a short one"},
		{"an entry in both lists fails", []string{"ID", "Label"}, frozen, "ID # a reason that is long enough\n", "in combatant_columns.txt and in combatant_columns_allowed.txt"},
		{"an allowed column that is gone fails", []string{"ID", "Label"}, frozen, long, `lists "Newer", which is not a column any more`},
		{"a shrinking list passes", []string{"ID"}, "ID\nLabel\n", "", `lists "Label", which is not a column any more`},
		{"a frozen list that shrank with the table passes", []string{"ID"}, "ID\n", "", ""},
		{"an unsorted frozen list fails", []string{"ID", "Label"}, "Label\nID\n", "", "not sorted"},
	}
	for _, c := range cases {
		got := checkCombatantColumns(c.columns, c.frozen, c.allowed)
		switch {
		case c.want == "" && len(got) != 0:
			t.Errorf("%s: problems %q, want none", c.name, got)
		case c.want != "" && !strings.Contains(strings.Join(got, "\n"), c.want):
			t.Errorf("%s: problems %q, want one with %q", c.name, got, c.want)
		}
	}
}
