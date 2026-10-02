// Package combat is the pure part of the fight (MR-012, MR-013, MR-014): what
// a character can do on their turn, and the arithmetic of attacks, damage,
// healing, slots and death saves. Like package rules it never touches a
// database, the network, the clock or randomness (ADR-0008): the dice are
// rolled by the caller and passed in, and every function returns a new value
// instead of changing its input. The play module stores the state and calls
// these functions; the web shows their results in Portuguese.
package combat
