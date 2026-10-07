// Package leaktest is the systematic layer of the RN-10 tests: what the master
// keeps hidden never reaches a player. It holds only tests (see harness_test.go
// for how the real services are wired, leak_test.go for the table of reads, and
// docs/arquitetura.md, "Os testes de vazamento (RN-10)").
package leaktest
