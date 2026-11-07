package archtest

import (
	_ "embed"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// sessionWriteExceptions lists the functions of internal/play and internal/maps that write (INSERT, UPDATE or DELETE of the
// package's own queries) in a transaction of their own, outside the funnel (Service.tx and Service.txOn in
// internal/play/hints.go). An entry names the function, not each site in it. The rule does not know which table a query
// writes: an entry whose reason starts with "permanent:" is accepted forever (the function never touches what a live
// session shows) and the rest is the work queue of #735 part 2, which can only shrink. Both kinds are listed, so a new
// writer is noticed, and a function that is no longer an offender fails the test ("remove it").
// An entry sits on the callee: converting it can need the calling module (characters, progression) to change too.
//
//go:embed sessionwrite_exceptions.txt
var sessionWriteExceptions string

// sessionWriteDirs are the packages whose transactions are read, with the module that owns the sqlc queries they call.
// Only the files directly in each directory are read (their subpackages are other packages), and the *.sql files of the
// directory name the queries.
var sessionWriteDirs = []string{"internal/play", "internal/maps"}

// sessionFunnel holds the functions that are the funnel: the test does not read into them, and they are not offenders.
var sessionFunnel = map[string]bool{
	"internal/play/hints.go:(*Service).tx":   true,
	"internal/play/hints.go:(*Service).txOn": true,
}

// sessionSentinels are the names whose call is a write or a tell of one in any package: the number of the session's event
// (NextSessionEventSeq), the hint number (BumpHintSeq), the streams (send, tellMembers in play, publish in maps) and the
// touch of the session (touch). They are matched by bare name.
var sessionSentinels = []string{"send", "tellMembers", "publish", "touch", "NextSessionEventSeq", "BumpHintSeq"}

// TestWritesGoThroughTheFunnel fails for a transaction that writes and is not Service.tx or Service.txOn: part 2 of #735
// moves every writer of session state into the funnel, so that the hint number moves with the write, and this test keeps a
// new writer from being born outside it. A function that sessionwrite_exceptions.txt lists is allowed; a listed function
// that no longer writes outside the funnel fails ("remove it"), so the work queue can only shrink, and a "permanent:"
// entry records a writer that never touches session state (the rule cannot tell tables apart).
//
// What counts as a write, by name and without types (go/parser only, as the other rules here):
//   - a transaction is a function literal given to db.InTx, or a call of Begin or BeginTx on a value named pool or ending in
//     .pool; the function that holds it is the offender, once, whatever the number of sites in it;
//   - the literal writes if it, nested literals included, calls a method or function that is a query of the package's own
//     *.sql files whose statement has an INSERT INTO, an UPDATE ... SET or a DELETE FROM (a sqlc method has the query's
//     name; a query that only SELECTs, even FOR UPDATE, is a read), or one of sessionSentinels;
//   - a call of a parameter of the enclosing function that has a function type counts as a write too (the closure
//     that the caller passes in is not seen, so it is taken to write: changeRun, changeActions);
//   - the call is followed through the package: a call of a name that is the name of a function or method declared in a
//     non-test file of the same directory counts when that function writes by the same rule (any receiver, any depth, by
//     name, so two methods of one name are one). The funnel is never entered, and a call of tx or txOn is not a write;
//   - an exported function or method with a pgx.Tx parameter is also an offender when it writes by that rule: it is the seam
//     a module calls with its own transaction (characters' ReviewHost), and no closure of this package holds it.
//
// Limits:
//   - a closure that is a method value, a field or a package-level function is not read;
//   - a closure that is a parameter of the enclosing function is not read either, but a literal that calls a parameter of
//     function type is taken to write;
//   - a name that two functions share is followed in both, which can list a site that only reads (list it, with the reason,
//     or rename the helper);
//   - a call through an interface or a function value is not followed;
//   - a write by raw SQL in a string is seen only through the named queries (the txhints rule keeps hint_seq out of strings);
//   - a savepoint (tx.Begin) is part of its transaction and not read on its own;
//   - a sentinel is matched by its bare name: any x.send, x.publish or x.touch call marks a writer, even if x is not a
//     session stream;
//   - db.InTx is recognized only when the receiver is an identifier literally named db.
func TestWritesGoThroughTheFunnel(t *testing.T) {
	root := os.DirFS(backendRoot)
	found := map[string]string{}
	files := 0
	for _, dir := range sessionWriteDirs {
		entries, err := fs.ReadDir(root, dir)
		if err != nil {
			t.Fatalf("reading %s: %v", dir, err)
		}
		srcs, sqls := map[string][]byte{}, ""
		for _, e := range entries {
			p := path.Join(dir, e.Name())
			if e.IsDir() {
				continue
			}
			src, err := fs.ReadFile(root, p)
			if err != nil {
				t.Fatalf("reading %s: %v", p, err)
			}
			switch {
			case path.Ext(p) == ".sql":
				sqls += "\n" + string(src)
			case path.Ext(p) == ".go" && !isTestFile(p):
				srcs[p] = src
			}
		}
		writers := writerQueries(sqls)
		if len(writers) == 0 || len(srcs) == 0 {
			t.Fatalf("%s: found %d writer queries and %d Go files: is backendRoot right?", dir, len(writers), len(srcs))
		}
		files += len(srcs)
		keys, err := sessionWriteKeys(srcs, writers)
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		for _, k := range keys {
			found[k] = fmt.Sprintf("%q writes in a transaction outside the funnel: use Service.tx (internal/play/hints.go) "+
				"and collect the hints with h.tell, or list the function in sessionwrite_exceptions.txt "+
				"(# permanent: <reason> if it never touches session state)", k)
		}
	}
	failProblems(t, compareExceptions("sessionwrite_exceptions.txt", parseExceptions(sessionWriteExceptions), found))
	work, permanent := countExceptions(sessionWriteExceptions)
	t.Logf("%d non-test Go files checked; %d known exceptions: %d in the work queue, %d permanent", files, len(found), work, permanent)
}

// countExceptions returns the number of entries of the work queue and of the permanent ones (the reason, after " #", starts
// with "permanent:").
func countExceptions(text string) (work, permanent int) {
	for line := range strings.Lines(text) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if _, reason, ok := strings.Cut(line, " #"); ok && strings.HasPrefix(strings.TrimSpace(reason), "permanent:") {
			permanent++
		} else {
			work++
		}
	}
	return work, permanent
}

var (
	queryNameRe  = regexp.MustCompile(`^--\s*name:\s*(\w+)`)
	writeStmtRe  = regexp.MustCompile(`(?i)\b(INSERT\s+INTO|DELETE\s+FROM|UPDATE\s+\w+(\s+(AS\s+)?\w+)?\s+SET)\b`)
	commentTailR = regexp.MustCompile(`\s--.*$`)
)

// writerQueries returns the names of the queries in sqlc query text that write: an INSERT INTO, a DELETE FROM or an
// UPDATE ... SET in a line that is not a comment (FOR UPDATE is a lock, not a write: UPDATE there is not followed by SET).
func writerQueries(sql string) map[string]bool {
	bodies := map[string]string{}
	cur := ""
	for line := range strings.Lines(sql) {
		line = strings.TrimSpace(line)
		if m := queryNameRe.FindStringSubmatch(line); m != nil {
			cur = m[1]
			continue
		}
		if cur != "" && !strings.HasPrefix(line, "--") {
			bodies[cur] += " " + commentTailR.ReplaceAllString(line, "")
		}
	}
	out := map[string]bool{}
	for name, body := range bodies {
		if writeStmtRe.MatchString(body) {
			out[name] = true
		}
	}
	return out
}

// sessionFile is one parsed file of the package.
type sessionFile struct {
	path string
	ast  *ast.File
}

// sessionWriteKeys returns the sorted keys "package file function" of the functions of one package (the files given by
// path, the writers being the names of its writing queries) that open a transaction outside the funnel which writes, or
// that are exported with a pgx.Tx parameter and write.
func sessionWriteKeys(srcs map[string][]byte, writers map[string]bool) ([]string, error) {
	var files []sessionFile
	for _, p := range slices.Sorted(maps.Keys(srcs)) {
		f, err := parser.ParseFile(token.NewFileSet(), p, srcs[p], parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		files = append(files, sessionFile{p, f})
	}
	reach := writingNames(files, writers)
	var keys []string
	add := func(f sessionFile, fn string) {
		k := fmt.Sprintf("%s %s %s", f.ast.Name.Name, path.Base(f.path), fn)
		if !slices.Contains(keys, k) {
			keys = append(keys, k)
		}
	}
	for _, f := range files {
		for _, decl := range f.ast.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Body == nil || sessionFunnel[f.path+":"+funcName(d)] {
					continue
				}
				if opensWritingTx(d, writers, reach) || (d.Name.IsExported() && hasTxParam(d) && writes(d.Body, writers, reach)) {
					add(f, funcName(d))
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok && opensWritingTx(vs, writers, reach) {
						name := "init"
						if len(vs.Names) > 0 && vs.Names[0].Name != "_" {
							name = vs.Names[0].Name
						}
						add(f, name)
					}
				}
			}
		}
	}
	slices.Sort(keys)
	return keys, nil
}

// callName is the name a call is made by: f(...) or x.f(...).
func callName(call *ast.CallExpr) string {
	switch f := call.Fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// writingNames returns the names of the functions of the package that write: they call a writer query or a sentinel, or a
// name that writes (the funnel's own functions are not entered).
func writingNames(files []sessionFile, writers map[string]bool) map[string]bool {
	type fn struct {
		name  string
		calls []string
	}
	var fns []fn
	for _, f := range files {
		for _, decl := range f.ast.Decls {
			if d, ok := decl.(*ast.FuncDecl); ok && d.Body != nil && !sessionFunnel[f.path+":"+funcName(d)] {
				var calls []string
				ast.Inspect(d.Body, func(n ast.Node) bool {
					if call, ok := n.(*ast.CallExpr); ok {
						calls = append(calls, callName(call))
					}
					return true
				})
				fns = append(fns, fn{d.Name.Name, calls})
			}
		}
	}
	reach := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, f := range fns {
			if reach[f.name] {
				continue
			}
			for _, c := range f.calls {
				if isWriteName(c, writers) || reach[c] {
					reach[f.name], changed = true, true
					break
				}
			}
		}
	}
	return reach
}

func isWriteName(name string, writers map[string]bool) bool {
	return writers[name] || slices.Contains(sessionSentinels, name)
}

// writes reports whether n, nested literals included, calls a writer query, a sentinel or a function that writes.
func writes(n ast.Node, writers, reach map[string]bool) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if name := callName(call); name != "tx" && name != "txOn" && (isWriteName(name, writers) || reach[name]) {
				found = true
			}
		}
		return !found
	})
	return found
}

// opensWritingTx reports whether n holds a transaction that writes: the literal of a db.InTx (given by a literal or by
// a name assigned a literal in n), or a Begin on a pool.
func opensWritingTx(root ast.Node, writers, reach map[string]bool) bool {
	lits := funcLiterals(root)
	unknown := funcParams(root)
	writesOrCallsParam := func(n ast.Node) bool {
		return writes(n, writers, reach) || callsAny(n, unknown)
	}
	found := false
	ast.Inspect(root, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || found {
			return !found
		}
		switch {
		case isDBInTx(call):
			for _, arg := range call.Args {
				if id, ok := arg.(*ast.Ident); ok && lits[id.Name] != nil {
					arg = lits[id.Name]
				}
				if lit, ok := arg.(*ast.FuncLit); ok && writesOrCallsParam(lit) {
					found = true
				}
			}
		case isPoolBegin(call):
			// Everything after a pool.Begin of the function belongs to the transaction it opens.
			found = writesOrCallsParam(root)
		}
		return !found
	})
	return found
}

func isDBInTx(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "InTx" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "db"
}

// isPoolBegin reports whether call is pool.Begin(...), x.pool.Begin(...) or the BeginTx of either.
func isPoolBegin(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "Begin" && sel.Sel.Name != "BeginTx") {
		return false
	}
	switch x := sel.X.(type) {
	case *ast.Ident:
		return x.Name == "pool"
	case *ast.SelectorExpr:
		return x.Sel.Name == "pool"
	}
	return false
}

// funcParams returns the names of the parameters of function type of a declaration (none for other nodes).
func funcParams(n ast.Node) map[string]bool {
	out := map[string]bool{}
	if d, ok := n.(*ast.FuncDecl); ok {
		for _, p := range d.Type.Params.List {
			if _, ok := p.Type.(*ast.FuncType); ok {
				for _, id := range p.Names {
					out[id.Name] = true
				}
			}
		}
	}
	return out
}

// callsAny reports whether n calls a plain name of the set.
func callsAny(n ast.Node, names map[string]bool) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && names[id.Name] {
				found = true
			}
		}
		return !found
	})
	return found
}

// hasTxParam reports whether the function takes a parameter of type pgx.Tx.
func hasTxParam(d *ast.FuncDecl) bool {
	for _, p := range d.Type.Params.List {
		if sel, ok := p.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "Tx" {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "pgx" {
				return true
			}
		}
	}
	return false
}

func TestWriterQueries(t *testing.T) {
	sql := "-- name: A :one\n-- INSERT INTO nope\nSELECT 1;\n\n-- name: B :exec\nUPDATE game_sessions SET x = 1 WHERE id = $1;\n\n" +
		"-- name: C :one\nSELECT id FROM game_sessions WHERE id = $1 FOR UPDATE;\n\n-- name: D :one\nINSERT INTO t (a) VALUES (1) RETURNING *;\n\n" +
		"-- name: E :exec\ndelete from t where id = $1;\n\n-- name: F :one\nUPDATE t x\nSET a = 1 FROM u WHERE x.id = u.id RETURNING *; -- note\n"
	got := writerQueries(sql)
	for name, want := range map[string]bool{"A": false, "B": true, "C": false, "D": true, "E": true, "F": true} {
		if got[name] != want {
			t.Errorf("writerQueries: %s writes = %v, want %v", name, got[name], want)
		}
	}
}

func TestCountExceptions(t *testing.T) {
	work, permanent := countExceptions("# c\n\na f F # 4d: x\nb g G # permanent: y\nc h H\nd i I # permanent: z\n")
	if work != 2 || permanent != 2 {
		t.Errorf("countExceptions = %d work, %d permanent, want 2 and 2", work, permanent)
	}
}

func TestSessionWriteKeys(t *testing.T) {
	writers := map[string]bool{"UpdateThing": true}
	cases := []struct {
		name string
		srcs map[string]string
		want []string
	}{
		{
			"a fresh unconverted writer",
			map[string]string{
				"internal/play/a.go": "package play\n" +
					"func (s *Service) Fresh() error {\n" +
					"\treturn db.InTx(c, s.pool, func(tx pgx.Tx) error { _, err := s.queries.WithTx(tx).UpdateThing(c, p); return err })\n" +
					"}",
			},
			[]string{"play a.go (*Service).Fresh"},
		},
		{
			"a read is not a write",
			map[string]string{
				"internal/play/a.go": "package play\n" +
					"func F(s S) error { return db.InTx(c, s.pool, func(tx pgx.Tx) error { _, err := s.q.GetThing(c); return err }) }",
			},
			nil,
		},
		{
			"through Service.tx",
			map[string]string{
				"internal/play/a.go": "package play\n" +
					"func F(s S) error { return s.tx(c, id, func(tx pgx.Tx, h *hints) error { _, err := s.q.UpdateThing(c); return err }) }",
			},
			nil,
		},
		{
			"a sentinel",
			map[string]string{
				"internal/play/a.go": "package play\nfunc F(s S) error { return db.InTx(c, p, func(tx pgx.Tx) error { s.touch(c); return nil }) }",
			},
			[]string{"play a.go F"},
		},
		{
			"through a helper, in another file",
			map[string]string{
				"internal/maps/a.go": "package maps\nfunc F(s S) error { return db.InTx(c, p, func(tx pgx.Tx) error { return s.step(c, tx) }) }",
				"internal/maps/b.go": "package maps\nfunc (s S) step(c C, tx T) error { return deeper(c) }\nfunc deeper(c C) error { return q.UpdateThing(c) }",
			},
			[]string{"maps a.go F"},
		},
		{
			"a helper that only reads",
			map[string]string{
				"internal/maps/a.go": "package maps\n" +
					"func F(s S) error { return db.InTx(c, p, func(tx pgx.Tx) error { return s.look(c) }) }\n" +
					"func (s S) look(c C) error { return q.GetThing(c) }",
			},
			nil,
		},
		{
			"named closure",
			map[string]string{
				"internal/play/a.go": "package play\nfunc F(s S) error { run := func(tx pgx.Tx) error { return q.UpdateThing(c) }; return db.InTx(c, p, run) }",
			},
			[]string{"play a.go F"},
		},
		{
			"a closure that is a parameter is not read",
			map[string]string{
				"internal/play/a.go": "package play\nfunc F(s S, run func(pgx.Tx) error) error { return db.InTx(c, p, run) }",
			},
			nil,
		},
		{
			"a closure that calls a function parameter",
			map[string]string{
				"internal/play/a.go": "package play\n" +
					"func F(s S, change func(Q) error) error { return db.InTx(c, p, func(tx pgx.Tx) error { return change(q) }) }",
			},
			[]string{"play a.go F"},
		},
		{
			"a closure that calls a local func is not a parameter",
			map[string]string{
				"internal/play/a.go": "package play\nfunc F(s S) error { return db.InTx(c, p, func(tx pgx.Tx) error { return change(q) }) }",
			},
			nil,
		},
		{
			"another InTx",
			map[string]string{
				"internal/play/a.go": "package play\nfunc F(s S) error { return other.InTx(c, p, func(tx pgx.Tx) error { return q.UpdateThing(c) }) }",
			},
			nil,
		},
		{
			"pool.Begin",
			map[string]string{
				"internal/play/a.go": "package play\nfunc F(s S) error { tx, _ := s.pool.Begin(c); return q.UpdateThing(c) }",
			},
			[]string{"play a.go F"},
		},
		{
			"a savepoint is not a pool",
			map[string]string{
				"internal/play/a.go": "package play\nfunc F(c C) error { sp, _ := c.tx.Begin(c); return q.UpdateThing(c) }",
			},
			nil,
		},
		{
			"the seam: exported with a pgx.Tx",
			map[string]string{
				"internal/play/a.go": "package play\n" +
					"func (s *Service) Host(c C, tx pgx.Tx) error { return s.helper() }\n" +
					"func (s *Service) helper() error { return q.UpdateThing(c) }\n" +
					"func (s *Service) quiet(c C, tx pgx.Tx) error { return q.UpdateThing(c) }",
			},
			[]string{"play a.go (*Service).Host"},
		},
		{
			"the seam that only reads",
			map[string]string{
				"internal/play/a.go": "package play\nfunc (s *Service) Host(c C, tx pgx.Tx) error { return q.GetThing(c) }",
			},
			nil,
		},
		{"the funnel is not an offender and not entered", map[string]string{
			"internal/play/hints.go": "package play\n" +
				"func (s *Service) tx(c C, id string, fn F) error { return s.txOn(c, s.pool, id, fn) }\n" +
				"func (s *Service) txOn(c C, st S, id string, fn F) error { return db.InTx(c, st, func(tx pgx.Tx) error { q.BumpHintSeq(c, id); return fn(tx, &h) }) }",
			"internal/play/a.go": "package play\nfunc F(s S) error { return db.InTx(c, p, func(tx pgx.Tx) error { return s.tx(c, id, nil) }) }",
		}, nil},
		{
			"package-level literal",
			map[string]string{
				"internal/play/a.go": "package play\nvar run = func(s S) error { return db.InTx(c, p, func(tx pgx.Tx) error { return q.UpdateThing(c) }) }",
			},
			[]string{"play a.go run"},
		},
		{
			"one key per function",
			map[string]string{
				"internal/play/a.go": "package play\n" +
					"func F(s S) { db.InTx(c, p, func(tx pgx.Tx) error { return q.UpdateThing(c) }); db.InTx(c, p, func(tx pgx.Tx) error { return q.UpdateThing(c) }) }",
			},
			[]string{"play a.go F"},
		},
	}
	for _, c := range cases {
		srcs := map[string][]byte{}
		for p, s := range c.srcs {
			srcs[p] = []byte(s)
		}
		got, err := sessionWriteKeys(srcs, writers)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
