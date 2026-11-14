package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// A spend of the master's Inspiration deletes the character's row, and the held roll remembers it (inspiration_holds.favor_spent).
// A hold dropped before it is answered is a roll that never happens, so the Inspiration is lost unless it is given back. There is
// one way to drop a hold: internal/play/hold_forget.go. These tests keep it the only one (docs/architecture/play/favor.md).

// holdForgetFile is the one file that may call the queries that delete holds.
const holdForgetFile = "internal/play/hold_forget.go"

// holdDeleteQueries are the only statements that DELETE FROM inspiration_holds, and the methods they generate.
var holdDeleteQueries = []string{"DeleteInspirationHold", "DeleteInspirationHoldsOf", "DeleteOrphanInspirationHolds"}

var deleteHolds = regexp.MustCompile(`(?i)\bDELETE\s+FROM\s+inspiration_holds\b`)

// TestOnlyTheForgetQueriesDeleteHolds fails for a query in a sqlc query file that deletes from inspiration_holds and is not one of the
// three that hold_forget.go wraps.
func TestOnlyTheForgetQueriesDeleteHolds(t *testing.T) {
	for _, f := range sqlcQueryFiles(t) {
		src, err := os.ReadFile(backendRoot + "/" + f)
		if err != nil {
			t.Fatal(err)
		}
		failProblems(t, holdDeletingQueries(f, string(src)))
	}
}

// holdDeletingQueries names the queries of the file that delete holds and are not allowed to.
func holdDeletingQueries(file, sql string) []string {
	var out []string
	heads := anyHeader.FindAllStringIndex(sql, -1)
	for i, h := range heads {
		end := len(sql)
		if i+1 < len(heads) {
			end = heads[i+1][0]
		}
		block := sql[h[0]:end]
		if !deleteHolds.MatchString(stripCommentsAndStrings(block)) {
			continue
		}
		name := queryHeader.FindStringSubmatch(block)[1]
		if !slices.Contains(holdDeleteQueries, name) {
			out = append(out, file+": "+name+" deletes from inspiration_holds: a hold dropped with no answer loses the Inspiration it spent. "+
				"Drop holds only through hold_forget.go (forgetHoldsOf, forgetHold, forgetOrphanHolds), which gives it back")
		}
	}
	return out
}

// TestHoldsAreDroppedOnlyThroughHoldForget fails for a non-test Go file of the backend, other than hold_forget.go, that calls a generated
// query that deletes holds.
func TestHoldsAreDroppedOnlyThroughHoldForget(t *testing.T) {
	var problems []string
	root := os.DirFS(backendRoot)
	err := fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != "." && (d.Name() == "node_modules" || d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".")) {
			return fs.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || path == holdForgetFile {
			return nil
		}
		src, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		problems = append(problems, callsOfHoldDeletes(path, src)...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	failProblems(t, problems)
}

// callsOfHoldDeletes lists the calls in the source of a method that deletes holds.
func callsOfHoldDeletes(file string, src []byte) []string {
	var out []string
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, 0)
	if err != nil {
		return []string{file + ": " + err.Error()}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if sel := selectorOfCall(n); sel != nil && slices.Contains(holdDeleteQueries, sel.Sel.Name) {
			out = append(out, file+":"+strconv.Itoa(fset.Position(sel.Pos()).Line)+" calls "+sel.Sel.Name+": a hold dropped with no answer loses the Inspiration it spent. "+
				"Call forgetHoldsOf, forgetHold or forgetOrphanHolds from "+holdForgetFile+" instead")
		}
		return true
	})
	return out
}

// selectorOfCall is the x.Name of a call x.Name(...), nil for any other node.
func selectorOfCall(n ast.Node) *ast.SelectorExpr {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return nil
	}
	sel, _ := call.Fun.(*ast.SelectorExpr)
	return sel
}

// TestADeletedCombatantForgetsItsHolds fails for a non-test file of internal/play that calls DeleteCombatant and never calls
// forgetHoldsOf: the foreign key of inspiration_holds cascades from combatants, and no path may rely on it (the Inspiration a held roll
// spent would go with the row).
func TestADeletedCombatantForgetsItsHolds(t *testing.T) {
	dir := backendRoot + "/internal/play"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(dir + "/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		checked++
		failProblems(t, deletesCombatantsWithoutForgetting("internal/play/"+e.Name(), src))
	}
	if checked == 0 {
		t.Fatal("no Go file found in internal/play: is backendRoot right?")
	}
}

// deletesCombatantsWithoutForgetting says the source calls DeleteCombatant and has no call of forgetHoldsOf.
func deletesCombatantsWithoutForgetting(file string, src []byte) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, 0)
	if err != nil {
		return []string{file + ": " + err.Error()}
	}
	deletes, forgets := 0, false
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			if fn.Sel.Name == "DeleteCombatant" {
				deletes++
			}
		case *ast.Ident:
			forgets = forgets || fn.Name == "forgetHoldsOf"
		}
		return true
	})
	if deletes > 0 && !forgets {
		return []string{file + " calls DeleteCombatant and never forgetHoldsOf: the foreign key would delete the combatant's held rolls and the " +
			"Inspiration they spent with them. Call forgetHoldsOf(ctx, q, holdOwnerOfCombatant(c)) before the delete"}
	}
	return nil
}

// The checks above fail on a violation: each one is shown on a source that has it, and passes on one that has not.
func TestTheHoldForgetChecksSeeAViolation(t *testing.T) {
	if got := holdDeletingQueries("q.sql", "-- name: DropHolds :exec\nDELETE FROM inspiration_holds WHERE id = $1;\n"); len(got) != 1 {
		t.Errorf("a query that deletes holds = %v, want one problem", got)
	}
	if got := holdDeletingQueries("q.sql", "-- name: DeleteInspirationHold :many\nWITH c AS (DELETE FROM inspiration_holds WHERE id = $1 RETURNING id)\nSELECT id FROM c ORDER BY id;\n"); len(got) != 0 {
		t.Errorf("the wrapped query = %v, want none", got)
	}
	if got := callsOfHoldDeletes("x.go", []byte("package x\nfunc f() { q.DeleteOrphanInspirationHolds(ctx, id) }\n")); len(got) != 1 {
		t.Errorf("a call of a hold delete = %v, want one problem", got)
	}
	if got := deletesCombatantsWithoutForgetting("x.go", []byte("package x\nfunc f() { q.DeleteCombatant(ctx, id) }\n")); len(got) != 1 {
		t.Errorf("a combatant deleted with no forget = %v, want one problem", got)
	}
	if got := deletesCombatantsWithoutForgetting("x.go", []byte("package x\nfunc f() { forgetHoldsOf(ctx, q, o); q.DeleteCombatant(ctx, id) }\n")); len(got) != 0 {
		t.Errorf("a combatant deleted after the forget = %v, want none", got)
	}
}
