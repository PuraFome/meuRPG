// Package grid is the geometry of a map's grid (MR-034, RN-21, Etapa 9): the
// squares and their size, the painted layers (walls, difficult terrain,
// light), the straight line between two squares, what a move costs, which
// squares a mover reaches, and the range of attacks and spells.
//
// Like the rest of package rules it is pure: no database, network, clock or
// randomness (ADR-0008). The maps module stores the layers this package
// packs, the play module asks it what a move costs, and package vision builds
// the fog of war on its line. The browser does no geometry of its own: it
// draws what the server computed with these functions.
//
// Units. A square is 5 ft (1,5 m). Movement is kept in tenths of a foot
// ("decifeet", the DFt suffix), so a square straight is 50 and the diagonal of
// a square is 71 (70,7 rounded to the nearest tenth), exactly as the circle
// of RN-21 asks. Ranges are in whole feet.
package grid
