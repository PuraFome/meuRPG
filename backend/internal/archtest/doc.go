// Package archtest holds the backend's architecture tests: the module
// boundaries and the table of how the modules depend on each other (see
// boundaries_test.go and docs/architecture/overview.md#module-boundaries-are-a-test).
// Five more conventions each have a test and a plain-text exception list that can
// only shrink: no bare go statement outside internal/platform/safego (nogo_test.go),
// no query that touches another module's table (table_owner_test.go), no playv1
// import outside the play module and the API wiring (playv1_test.go), and no
// database-error codes read by hand in identity and authz (pg_errors_test.go), and no :many query without an
// ORDER BY in its outermost statement (query_order_test.go).
// The tests read the source with go/parser, so they need no database and no
// build of the other packages. The package has no production code.
package archtest
