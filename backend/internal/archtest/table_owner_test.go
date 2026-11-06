package archtest

import (
	_ "embed"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// tableOwnerExceptions is the list of queries that still touch another
// module's table; it can only shrink.
//
//go:embed table_owner_exceptions.txt
var tableOwnerExceptions string

// dataReadme is docs/data/README.md, seen from this package's directory.
const dataReadme = "../../../docs/data/README.md"

// ownerless are the tables that belong to no module (docs/data/README.md).
var ownerless = []string{"goose_db_version", "migration_lock"}

// sqlKeywords are words that can follow FROM/JOIN/INTO/UPDATE/USING without
// being a table name: "ON CONFLICT DO UPDATE SET", "FOR UPDATE OF t",
// "FOR UPDATE SKIP LOCKED", "IS DISTINCT FROM", "EXTRACT(x FROM ..)".
var sqlKeywords = []string{"set", "of", "skip", "nowait", "select", "lateral", "only"}

var (
	ownerRow     = regexp.MustCompile("^\\|\\s*`(\\w+)`(?:\\s*\\([^)]*\\))?\\s*\\|[^|]*\\|(.*)\\|\\s*$")
	backtickName = regexp.MustCompile("`(\\w+)`")
	queryHeader  = regexp.MustCompile(`(?m)^--\s*name:\s*(\w+)`)
	lineComment  = regexp.MustCompile(`--[^\n]*`)
	stringLit    = regexp.MustCompile(`'(?:[^']|'')*'`)
	cteWith      = regexp.MustCompile(`(?i)\bWITH\s+(?:RECURSIVE\s+)?(\w+)\s*(?:\([^)]*\))?\s+AS\b`)
	cteNext      = regexp.MustCompile(`(?i),\s*(\w+)\s*(?:\([^)]*\))?\s+AS\s+(?:NOT\s+)?(?:MATERIALIZED\s*)?\(`)
	tableRef     = regexp.MustCompile(`(?i)\b(FROM|JOIN|INTO|UPDATE|USING)\s+([a-z_][a-z0-9_.]*)(\()?`)
	notTableWord = regexp.MustCompile(`(?i)\bDISTINCT\s+\w*$`)
)

// TestQueriesTouchOnlyOwnTables fails when a query in internal/<module>/queries.sql
// names a table that another module owns (docs/data/README.md, "Modules and
// tables") and table_owner_exceptions.txt does not list the query, when a
// listed query no longer does, and when a query names a table no module lists.
func TestQueriesTouchOnlyOwnTables(t *testing.T) {
	owners := readTableOwners(t)
	matches, err := fs.Glob(os.DirFS(backendRoot), "internal/*/queries.sql")
	if err != nil || len(matches) == 0 {
		t.Fatalf("found no internal/*/queries.sql (err %v): is backendRoot right?", err)
	}
	found := map[string]string{}
	var unlisted []string
	queries := 0
	for _, m := range matches {
		src, err := os.ReadFile(backendRoot + "/" + m)
		if err != nil {
			t.Fatal(err)
		}
		module := strings.Split(m, "/")[1]
		qs := scanQueries(module, string(src), owners)
		queries += qs.count
		for k, msg := range qs.foreign {
			found[k] = msg
		}
		unlisted = append(unlisted, qs.unlisted...)
	}
	slices.Sort(unlisted)
	for _, u := range slices.Compact(unlisted) {
		t.Error(u)
	}
	failProblems(t, compareExceptions("table_owner_exceptions.txt", parseExceptions(tableOwnerExceptions), found))
	t.Logf("%d queries in %d queries.sql files checked against %d tables of %d modules; %d known exceptions",
		queries, len(matches), len(owners), countModules(owners), len(found))
}

func countModules(owners map[string]string) int {
	seen := map[string]bool{}
	for _, m := range owners {
		seen[m] = true
	}
	return len(seen)
}

// readTableOwners parses the table of docs/data/README.md, "Modules and
// tables": table name -> owning module.
func readTableOwners(t *testing.T) map[string]string {
	t.Helper()
	md, err := os.ReadFile(dataReadme)
	if err != nil {
		t.Fatalf("reading %s: %v", dataReadme, err)
	}
	owners := parseTableOwners(string(md))
	if len(owners) == 0 {
		t.Fatalf("%s has no table rows under \"Modules and tables\"", dataReadme)
	}
	return owners
}

func parseTableOwners(md string) map[string]string {
	owners := map[string]string{}
	in := false
	for line := range strings.Lines(md) {
		line = strings.TrimRight(line, "\n")
		if strings.HasPrefix(line, "## ") {
			in = strings.TrimSpace(strings.TrimPrefix(line, "## ")) == "Modules and tables"
			continue
		}
		if !in {
			continue
		}
		if m := ownerRow.FindStringSubmatch(line); m != nil {
			for _, tb := range backtickName.FindAllStringSubmatch(m[2], -1) {
				owners[tb[1]] = m[1]
			}
		}
	}
	return owners
}

// queryScan is what one queries.sql holds.
type queryScan struct {
	count    int
	foreign  map[string]string // "module/Query" -> failure message
	unlisted []string          // failure messages: a table no module lists
}

// scanQueries checks every "-- name:" block of one module's queries.sql.
func scanQueries(module, sql string, owners map[string]string) queryScan {
	qs := queryScan{foreign: map[string]string{}}
	heads := queryHeader.FindAllStringSubmatchIndex(sql, -1)
	for i, h := range heads {
		end := len(sql)
		if i+1 < len(heads) {
			end = heads[i+1][0]
		}
		name := sql[h[2]:h[3]]
		key := module + "/" + name
		qs.count++
		foreign := map[string]string{} // table -> owner
		for _, tb := range tablesOf(sql[h[1]:end]) {
			owner, ok := owners[tb]
			switch {
			case slices.Contains(ownerless, tb):
			case !ok:
				qs.unlisted = append(qs.unlisted, fmt.Sprintf("%s reads the table %q, which no module lists in docs/data/README.md "+
					"(\"Modules and tables\"): add it to its module's row there", key, tb))
			case owner != module:
				foreign[tb] = owner
			}
		}
		if len(foreign) > 0 {
			var parts []string
			for tb, owner := range foreign {
				parts = append(parts, tb+" ("+owner+")")
			}
			sort.Strings(parts)
			qs.foreign[key] = fmt.Sprintf("%s touches tables of another module: %s. Read another module's data through its link "+
				"interface (docs/architecture/overview.md), never by joining its tables "+
				"(or, for a sweep that must, list the query in table_owner_exceptions.txt)", key, strings.Join(parts, ", "))
		}
	}
	return qs
}

// tablesOf returns the table names one query reads or writes, CTE aliases,
// function calls and keywords left out.
func tablesOf(body string) []string {
	body = lineComment.ReplaceAllString(body, "")
	body = stringLit.ReplaceAllString(body, "''")
	ctes := map[string]bool{}
	for _, m := range cteWith.FindAllStringSubmatch(body, -1) {
		ctes[strings.ToLower(m[1])] = true
	}
	for _, m := range cteNext.FindAllStringSubmatch(body, -1) {
		ctes[strings.ToLower(m[1])] = true
	}
	var out []string
	for _, m := range tableRef.FindAllStringSubmatchIndex(body, -1) {
		verb := strings.ToUpper(body[m[2]:m[3]])
		name := strings.ToLower(body[m[4]:m[5]])
		call := m[6] >= 0
		if call && verb != "INTO" && verb != "UPDATE" { // FROM unnest(...) is a function
			continue
		}
		if notTableWord.MatchString(body[:m[3]]) || slices.Contains(sqlKeywords, name) || ctes[name] {
			continue
		}
		if i := strings.LastIndex(name, "."); i >= 0 {
			name = name[i+1:]
		}
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

func TestTablesOf(t *testing.T) {
	cases := []struct {
		name, sql string
		want      []string
	}{
		{"join", "SELECT * FROM characters c JOIN map_points p ON p.id = c.id", []string{"characters", "map_points"}},
		{"insert and update", "INSERT INTO a (x) VALUES ($1) ON CONFLICT (x) DO UPDATE SET x = 1", []string{"a"}},
		{"cte alias ignored", "WITH recent AS (SELECT id FROM maps), other AS (SELECT 1) SELECT * FROM recent JOIN other ON true", []string{"maps"}},
		{"function ignored", "SELECT * FROM unnest($1::uuid[]) AS u(id) JOIN jsonb_object_keys(x) k ON true", nil},
		{"delete using", "DELETE FROM a USING b WHERE a.id = b.id", []string{"a", "b"}},
		{"for update of", "SELECT 1 FROM a FOR UPDATE OF a SKIP LOCKED", []string{"a"}},
		{"is distinct from", "SELECT 1 FROM a WHERE x IS DISTINCT FROM y", []string{"a"}},
		{"comment and string", "SELECT 'from nowhere' FROM a -- join b\n", []string{"a"}},
		{"subselect", "SELECT * FROM (SELECT * FROM a) s", []string{"a"}},
	}
	for _, c := range cases {
		if got := tablesOf(c.sql); !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestScanQueries(t *testing.T) {
	owners := parseTableOwners("## Modules and tables\n\n" +
		"| Module | Page | Tables |\n| --- | --- | --- |\n" +
		"| `maps` | [maps.md](maps.md) | `maps`, `map_points` |\n" +
		"| `play` (combat) | [combat.md](combat.md) | `encounters` |\n\n## Next\n\n| `other` | x | `ignored` |\n")
	if owners["map_points"] != "maps" || owners["encounters"] != "play" || owners["ignored"] != "" {
		t.Fatalf("parseTableOwners = %v", owners)
	}
	sql := "-- name: Own :one\nSELECT * FROM encounters;\n" +
		"-- name: Joins :many\n-- a comment\nSELECT * FROM encounters e JOIN map_points p ON p.id = e.id;\n" +
		"-- name: Alias :many\nWITH map_points AS (SELECT 1) SELECT * FROM map_points;\n" +
		"-- name: Unknown :one\nSELECT * FROM mystery_table;\n" +
		"-- name: Goose :one\nSELECT * FROM goose_db_version;\n"
	qs := scanQueries("play", sql, owners)
	if qs.count != 5 {
		t.Errorf("count = %d, want 5", qs.count)
	}
	if len(qs.foreign) != 1 || !strings.Contains(qs.foreign["play/Joins"], "map_points (maps)") {
		t.Errorf("foreign = %v, want only play/Joins naming map_points (maps)", qs.foreign)
	}
	if len(qs.unlisted) != 1 || !strings.Contains(qs.unlisted[0], "mystery_table") {
		t.Errorf("unlisted = %v, want only mystery_table", qs.unlisted)
	}
}
