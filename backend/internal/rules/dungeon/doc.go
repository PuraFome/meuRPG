// Package dungeon is the dungeon generator (MR-010, Etapa 10): it turns
// (Options, seed) into a level, a rectangular grid of squares with rooms,
// 1-square corridors, doors and stairs, plus the metadata of a map.
//
// Like the rest of package rules it is pure (ADR-0008): no database,
// network, clock or global state, no map iteration without sorted keys, no
// floating point in any decision, no recursion in the maze (an explicit
// stack), no goroutines. The same (Version, seed, Options) gives a
// byte-identical Dungeon on every platform. Generate is the entry point;
// Check runs every invariant; WallsMask, Markers and PromptOf are the
// renderings the map integration derives from a result.
//
// # Clean room
//
// The package was written from a behavioral specification alone, in a clean
// room (ADR-0015). The specification and the audit that preceded it live with
// the PR, not in the code; nothing here comes from another generator.
//
// # Algorithm and draw order (the contract of Version 1)
//
// The phases run in this order, each with its own random stream (SplitMix64
// of the seed XOR a phase constant, constants in types.go): mask, rooms
// (spread or tiled), doors from rooms, corridors (a randomized
// depth-first maze), connectivity (components, orphan corridors dropped,
// orphan rooms linked or discarded, optional extra loops), stairs,
// dead-end removal, cleanup. Every draw in the code is commented with
// "DRAW:" and the order is the one of the specification's 4.5; any change to
// it, to a constant or to a number that alters an output needs a new Version
// and a new golden file (testdata/golden.json).
//
// # Where the specification was read, not copied
//
// Every place where it was silent or contradicted itself, with the smallest
// reading that keeps the invariants of its section 5:
//
//  1. Door kinds and trapped flags all come from one extra stream, "door
//     kinds" (4.2, amended), in the order the doors are made: 3.4 per room in
//     order of acceptance, then each link door, then each loop door. The
//     number of values a kind consumes depends on door_mix, so drawing them
//     from a phase's stream would move doors; this way a different mix changes
//     only kinds and flags.
//  2. A bounded draw among n <= 1 things draws nothing (like "no draw when
//     the side has one candidate"). Chances at 0 and 100 draw nothing.
//  3. Spread anchors are drawn over every lattice node (row, then column),
//     not over the nodes where the size fits: a size that does not fit at the
//     anchor fails the attempt, as 3.3 says. The aspect check allows the first
//     pair and 4 redraws.
//  4. A room is valid only if its floor and its wall ring contain no blocked
//     square. In tiled mode the aspect limit is part of "fits" while shrinking
//     (shrinking the width of a tall room would break 5.2), and the shrink
//     takes the width all the way down to RoomSideMin before the height
//     (3.3 says "the width and then the height").
//  5. If placement leaves no room, the forced room of the edge cases is tried
//     (no draw); with none, ErrNoSpace. A forced room also covers plain bad
//     luck on a small grid.
//  6. Options. Height may go to 399, width to 199 (section 6). A RoomSideMax
//     that normalises below the normalised RoomSideMin (14, 14) becomes
//     RoomSideMin. MaskHole counts only for donut and CustomMask only for
//     custom. AspectLimit is converted once to hundredths.
//  7. Doors. A candidate also needs, when it opens straight into another
//     room, the other room's neighboring thresholds to be free of doors
//     (the door is on both rings, and 5.6 is per room). Thresholds are 2
//     apart, so "4 squares from every door on that side" means the two
//     neighboring thresholds.
//  8. Corridors. The first node of a door lane walk prefers the direction
//     away from the door without a keep-direction draw; its four directions
//     are still shuffled. The directions of a node are drawn when the walk
//     reaches it and kept for when the walk backs up. A lane is a valid step
//     destination until a walk has started from it or stepped onto it (3.5,
//     amended); a lane already joined when its turn to start comes is skipped
//     (a walk from it would find nothing and only waste draws).
//  9. Connectivity. Candidates are gathered from both sides (3.6.3, amended):
//     (a) thresholds of the group's rooms whose far side is a main-component
//     open square (corridor, or room floor for a direct door) or a free usable
//     node, then (b) thresholds of the main component's rooms whose far side
//     is an open square of the group (always opened directly). All obey the
//     same 4-square spacing as 3.4. The path search targets main-component
//     corridor squares only. A group that fails is deferred to the back of
//     the queue; each attempt gathers and shuffles its candidates again, one
//     DRAW per attempt. When a full pass links nothing, only the front group
//     (lowest room ID) is discarded, with its whole component (corridors
//     too), and another pass runs. The group's "other rooms" are already in
//     the one candidate list. The sealed corridor trees of 3.6.2 are erased
//     before any link, so their nodes are free for the paths. Extra loops:
//     candidates are corridor-corridor wall-line squares and room thresholds
//     whose far side is a corridor, gathered in row-major order of the square
//     (a threshold by its door square); one shuffle; each still-valid
//     candidate is opened with extra_loops percent chance (no draw at 100).
//  10. Stairs. A stair site's tests (c) and (d) of 3.7 follow from "exactly
//     one open neighbor"; two stairs never share a segment because they are
//     5 apart and a segment with two dead ends has no door and was erased.
//     Stairs 0 draws nothing. The room-site list is gathered in row-major
//     order and shuffled after the dead-end sites are exhausted; the up/down
//     draws of the stairs after the second come last. A room-site stair
//     faces west at the west corners and east at the east ones.
//  11. Dead ends. A lane square whose only open neighbor is its door is a
//     dead end; cutting it leaves a door to nowhere, which 3.9 removes. The
//     cut loop stops at a junction, a door, a protected square or the first
//     failed chance; a square it stops at after a failed chance is not drawn
//     for again by the scan. A room left with no exit in a level of two or
//     more rooms is discarded (3.9.2, into discarded_rooms; with SelfCheck on
//     it is an ErrSelfCheck instead); a one-room level keeps its room.
//  12. Derived data. Corridor runs are the connected corridor squares that
//     are not junctions; a junction square (3+ open neighbors) is a
//     corridor of its own. Door IDs and corridor IDs are row-major.
//     connections[] holds the direct room-to-room doors; networks[] the
//     connected corridor-square networks with their corridors, the rooms with
//     a door onto them and their size (1.5 d, amended): one labeling and one
//     pass over the doors. Door.RoomA is the room at the north or west end of
//     a room-to-room door, Side the side of RoomA's ring it is on.
//  13. Result. Mask is true where the silhouette lets the level be. Blocked
//     squares are KindRock in Kinds. Options.SelfCheck is the debug self-check
//     of 3.9.6.
package dungeon
