// Package encounter is the arithmetic of an encounter (MR-043, RN-29, Etapa 10,
// slice 10.9c): how much XP a party can spend, how much an encounter costs, which
// band (low, moderate or high) that is, the strongest creature the party may meet,
// the generator of an encounter from a seed, and the creatures one can swap for
// another at the same XP.
//
// The budget table is the "XP Budget per Character" of the SRD 5.2.1 (the 2024
// rules, CC BY 4.0): the rules module loads it from effects/encounter_budget.json
// and hands it to these functions as plain numbers. Like the rest of package
// rules this one is pure: no database, no network, no clock and no randomness of
// its own. A seed is an input, and the same seed gives the same encounter
// (ADR-0008). The generator is our own algorithm, written for this app and
// described in docs/arquitetura.md (ADR-0015).
package encounter
